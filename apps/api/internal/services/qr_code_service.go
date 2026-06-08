package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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

	// Check for existing QR code
	existingQR, err := s.qrRepo.FindByObjectID(ctx, ownerID, objectID)
	if err == nil && existingQR != nil {
		return existingQR, nil
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

	// Generate QR token
	token, err := generateQRToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	// Create QR code
	qr, err := s.qrRepo.Create(ctx, ownerID, objectID, token)
	if err != nil {
		return nil, fmt.Errorf("create qr code: %w", err)
	}

	// TODO: Add QR code image generation (e.g. generate image, upload to storage, etc.)

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
		return nil, errors.New("qr code not found")
	}

	return qr, nil
}

func generateQRToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
