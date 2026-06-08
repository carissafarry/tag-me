package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type QRCodeService struct {
	qrRepo   QRCodeRepository
	objRepo  ObjectRepository
	redisCmd redis.Cmdable
	genTTL   time.Duration
}

func NewQRCodeService(qrRepo QRCodeRepository, objRepo ObjectRepository, redisCmd redis.Cmdable) *QRCodeService {
	return &QRCodeService{
		qrRepo:   qrRepo,
		objRepo:  objRepo,
		redisCmd: redisCmd,
		genTTL:   30 * time.Second, // TODO: make this configurable
	}
}

func (s *QRCodeService) GenerateQRCode(ctx context.Context, ownerID uuid.UUID, objectID uuid.UUID) (*models.QRCode, error) {
	// Verify object exists and belongs to owner
	obj, err := s.objRepo.FindByID(ctx, objectID, ownerID)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, ErrObjectNotFound
	}

	// Check for generation in progress (Redis lock)
	lockKey := fmt.Sprintf("qr_gen:%s:%s", ownerID.String(), objectID.String())
	lockResult, err := s.redisCmd.SetNX(ctx, lockKey, "1", s.genTTL).Result()
	if err != nil {
		return nil, fmt.Errorf("redis lock check failed: %w", err)
	}
	if !lockResult {
		return nil, ErrQRCodeGenerationInProgress
	}
	defer s.redisCmd.Del(ctx, lockKey)

	// Check if QR code exists
	existing, _ := s.qrRepo.FindByObjectID(ctx, ownerID, objectID)

	// Generate QR token
	token, err := generateQRToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	var qr *models.QRCode
	if existing != nil {
		// Update existing QR code token
		qr, err = s.qrRepo.UpdateToken(ctx, objectID, token)
		if err != nil {
			return nil, fmt.Errorf("update qr code: %w", err)
		}
	} else {
		// Create new QR code
		qr, err = s.qrRepo.CreateToken(ctx, ownerID, objectID, token)
		if err != nil {
			return nil, fmt.Errorf("create qr code: %w", err)
		}
	}

	return qr, nil
}

func (s *QRCodeService) GetQRCode(ctx context.Context, ownerID uuid.UUID, objectID uuid.UUID) (*models.QRCode, error) {
	// Verify object exists and belongs to owner
	obj, err := s.objRepo.FindByID(ctx, objectID, ownerID)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, ErrObjectNotFound
	}

	qr, err := s.qrRepo.FindByObjectID(ctx, ownerID, objectID)
	if err != nil {
		return nil, err
	}
	if qr == nil {
		return nil, ErrQRCodeNotFound
	}

	return qr, nil
}

func (s *QRCodeService) GenerateQRCodeImage(ctx context.Context, qrToken string) (string, error) {
	qrImageDir := "storage/qr_images"
	filePath := fmt.Sprintf("%s/%s.png", qrImageDir, qrToken)

	qrImage, err := s.qrRepo.GenerateQRImage(ctx, qrToken, filePath)
	if err != nil {
		return "", err
	}
	
	return qrImage, nil
}

func generateQRToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
