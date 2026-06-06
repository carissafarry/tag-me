package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/repository"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateObject(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Create an owner first
	owner := &models.Owner{
		ID:          uuid.New(),
		Contact:     "test@example.com",
		ContactType: "email",
		IsActive:    true,
	}
	ownerQuery := `INSERT INTO owners (id, contact, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`
	_, err := db.Exec(ctx, ownerQuery, owner.ID, owner.Contact, owner.ContactType, owner.IsActive)
	if err != nil {
		t.Fatalf("failed to insert owner: %v", err)
	}

	service := services.NewObjectService(repository.NewObjectRepository(db))

	req := &models.CreateObjectRequest{
		Name:       "My Car",
		ObjectType: "car",
		Plate:      strPtr("AB123CD"),
	}

	obj, err := service.CreateObject(ctx, owner.ID, req)
	if err != nil {
		t.Fatalf("CreateObject failed: %v", err)
	}

	if obj.Name != req.Name {
		t.Errorf("expected name %s, got %s", req.Name, obj.Name)
	}
	if obj.ObjectType != req.ObjectType {
		t.Errorf("expected type %s, got %s", req.ObjectType, obj.ObjectType)
	}
	if obj.OwnerID != owner.ID {
		t.Errorf("owner_id mismatch")
	}
}

func TestListObjectsOwnerScoped(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// Create two owners
	owner1 := uuid.New()
	owner2 := uuid.New()
	setupOwner(t, db, owner1, "owner1@example.com")
	setupOwner(t, db, owner2, "owner2@example.com")

	// Create objects for each owner
	service := services.NewObjectService(repository.NewObjectRepository(db))
	_, err := service.CreateObject(ctx, owner1, &models.CreateObjectRequest{
		Name:       "Owner1 Car",
		ObjectType: "car",
	})
	if err != nil {
		t.Fatalf("failed to create object: %v", err)
	}

	_, err = service.CreateObject(ctx, owner2, &models.CreateObjectRequest{
		Name:       "Owner2 Bike",
		ObjectType: "motorcycle",
	})
	if err != nil {
		t.Fatalf("failed to create object: %v", err)
	}

	// List for owner1 should only see their object
	objects, err := service.GetObjects(ctx, owner1, map[string]string{})
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}

	if len(objects) != 1 {
		t.Errorf("expected 1 object for owner1, got %d", len(objects))
	}
	if objects[0].Name != "Owner1 Car" {
		t.Errorf("expected 'Owner1 Car', got %s", objects[0].Name)
	}
}

func TestGetObjectUnauthorized(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	owner1 := uuid.New()
	owner2 := uuid.New()
	setupOwner(t, db, owner1, "owner1@example.com")
	setupOwner(t, db, owner2, "owner2@example.com")

	service := services.NewObjectService(repository.NewObjectRepository(db))
	obj, err := service.CreateObject(ctx, owner1, &models.CreateObjectRequest{
		Name:       "Owner1 Car",
		ObjectType: "car",
	})
	if err != nil {
		t.Fatalf("failed to create object: %v", err)
	}

	// owner2 tries to get owner1's object
	retrievedObj, err := service.GetObject(ctx, owner2, obj.ID)
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}

	if retrievedObj != nil {
		t.Errorf("owner2 should not access owner1's object")
	}

	// owner1 can get their own object
	retrievedObj, err = service.GetObject(ctx, owner1, obj.ID)
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}

	if retrievedObj == nil {
		t.Errorf("owner1 should access their own object")
	}
}

func TestDeleteObjectBlockedByActiveConversation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	owner := uuid.New()
	setupOwner(t, db, owner, "owner@example.com")

	service := services.NewObjectService(repository.NewObjectRepository(db))
	obj, err := service.CreateObject(ctx, owner, &models.CreateObjectRequest{
		Name:       "My Car",
		ObjectType: "car",
	})
	if err != nil {
		t.Fatalf("failed to create object: %v", err)
	}

	// Create QR code for this object
	qrID := uuid.New()
	qrQuery := `INSERT INTO qr_codes (id, owner_id, object_id, qr_token, object_type, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW())`
	_, err = db.Exec(ctx, qrQuery, qrID, owner, obj.ID, "token-"+qrID.String(), "car")
	if err != nil {
		t.Fatalf("failed to insert qr_code: %v", err)
	}

	// Create an active conversation
	convID := uuid.New()
	convQuery := `INSERT INTO conversations (id, qr_code_id, owner_id, status, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'PENDING', NOW() + interval '24 hours', NOW(), NOW())`
	_, err = db.Exec(ctx, convQuery, convID, qrID, owner)
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}

	// Try to delete - should fail
	err = service.DeleteObject(ctx, owner, obj.ID)
	if err == nil {
		t.Errorf("expected error when active conversation exists")
	}

	// Check if it's the right error
	if !errors.Is(err, services.ErrConversationActive) {
		t.Errorf("expected ErrConversationActive, got %v", err)
	}

	// Resolve the conversation
	_, err = db.Exec(ctx, "UPDATE conversations SET status = 'RESOLVED', resolved_at = NOW() WHERE id = $1", convID)
	if err != nil {
		t.Fatalf("failed to update conversation: %v", err)
	}

	// Delete the conversation (simulating cleanup)
	_, err = db.Exec(ctx, "DELETE FROM conversations WHERE id = $1", convID)
	if err != nil {
		t.Fatalf("failed to delete conversation: %v", err)
	}

	// Now delete should succeed
	err = service.DeleteObject(ctx, owner, obj.ID)
	if err != nil {
		t.Errorf("DeleteObject should succeed after conversation deleted: %v", err)
	}
}

func TestDeleteObjectSuccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	owner := uuid.New()
	setupOwner(t, db, owner, "owner@example.com")

	service := services.NewObjectService(repository.NewObjectRepository(db))
	obj, err := service.CreateObject(ctx, owner, &models.CreateObjectRequest{
		Name:       "My Bag",
		ObjectType: "bag",
	})
	if err != nil {
		t.Fatalf("failed to create object: %v", err)
	}

	// Delete should succeed
	err = service.DeleteObject(ctx, owner, obj.ID)
	if err != nil {
		t.Errorf("DeleteObject failed: %v", err)
	}

	// Verify it's deleted
	retrieved, err := service.GetObject(ctx, owner, obj.ID)
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	if retrieved != nil {
		t.Errorf("object should be deleted")
	}
}

// Helper functions

func setupOwner(t *testing.T, db *pgxpool.Pool, ownerID uuid.UUID, contact string) {
	t.Helper()
	ctx := context.Background()
	query := `INSERT INTO owners (id, contact, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, true, NOW(), NOW())`
	_, err := db.Exec(ctx, query, ownerID, contact, "email")
	if err != nil {
		t.Fatalf("failed to setup owner: %v", err)
	}
}

func strPtr(s string) *string {
	return &s
}
