package services

import (
	"context"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/google/uuid"
)


type ObjectService struct {
	repo ObjectRepository
}

type ObjectRepository interface {
	Create(ctx context.Context, object *models.Object) error
	FindAll(ctx context.Context, ownerID uuid.UUID, filters map[string]string) ([]models.Object, error)
	FindByID(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) (*models.Object, error)
	Delete(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error
	HasActiveConversations(ctx context.Context, objectID uuid.UUID, ownerID uuid.UUID) (bool, error)
}

func NewObjectService(repo ObjectRepository) *ObjectService {
	return &ObjectService{
		repo: repo,
	}
}

func (s *ObjectService) CreateObject(ctx context.Context, ownerID uuid.UUID, req *models.CreateObjectRequest) (*models.Object, error) {
	obj := &models.Object{
		ID:         uuid.New(),
		OwnerID:    ownerID,
		Name:       req.Name,
		ObjectType: req.ObjectType,
		Plate:      req.Plate,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	err := s.repo.Create(ctx, obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (s *ObjectService) GetObjects(ctx context.Context, ownerID uuid.UUID, filters map[string]string) ([]models.Object, error) {
	return s.repo.FindAll(ctx, ownerID, filters)
}

func (s *ObjectService) GetObject(ctx context.Context, ownerID uuid.UUID, objectID uuid.UUID) (*models.Object, error) {
	return s.repo.FindByID(ctx, objectID, ownerID)
}

func (s *ObjectService) DeleteObject(ctx context.Context, ownerID uuid.UUID, objectID uuid.UUID) error {
	hasActive, err := s.repo.HasActiveConversations(ctx, objectID, ownerID)
	if err != nil {
		return err
	}
	if hasActive {
		return ErrConversationActive
	}

	return s.repo.Delete(ctx, objectID, ownerID)
}
