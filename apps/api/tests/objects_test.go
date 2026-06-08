package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/repository"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
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
	qrQuery := `INSERT INTO qr_codes (id, owner_id, object_id, qr_token, is_active)
		VALUES ($1, $2, $3, $4, $5)`
	_, err = db.Exec(ctx, qrQuery, qrID, owner, obj.ID, "token-"+qrID.String(), true)
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

func setupTestRedis() redis.Cmdable {
	mr := miniredis.RunT(&testing.T{})
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return client
}

func TestGenerateQRCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	owner := uuid.New()
	setupOwner(t, db, owner, "owner@example.com")

	// Create an object
	objRepo := repository.NewObjectRepository(db)
	objService := services.NewObjectService(objRepo)
	obj, err := objService.CreateObject(ctx, owner, &models.CreateObjectRequest{
		Name:       "My Car",
		ObjectType: "car",
	})
	if err != nil {
		t.Fatalf("failed to create object: %v", err)
	}

	// Setup Redis for the QR service (using mock or real Redis)
	redisCmd := setupTestRedis()
	qrService := services.NewQRCodeService(
		repository.NewQRCodeRepository(db),
		objRepo,
		redisCmd,
	)

	// Generate QR code
	qr, err := qrService.GenerateQRCode(ctx, owner, obj.ID)
	if err != nil {
		t.Fatalf("GenerateQRCode failed: %v", err)
	}

	if qr == nil {
		t.Errorf("expected QR code, got nil")
	}
	if qr.ObjectID != obj.ID {
		t.Errorf("expected object_id %s, got %s", obj.ID, qr.ObjectID)
	}
	if qr.OwnerID != owner {
		t.Errorf("expected owner_id %s, got %s", owner, qr.OwnerID)
	}
	if qr.QRToken == "" {
		t.Errorf("expected non-empty QR token")
	}
	if !qr.IsActive {
		t.Errorf("expected QR code to be active")
	}
}

func TestGenerateQRCodeObjectNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	owner := uuid.New()
	setupOwner(t, db, owner, "owner@example.com")

	objRepo := repository.NewObjectRepository(db)
	redisCmd := setupTestRedis()
	qrService := services.NewQRCodeService(
		repository.NewQRCodeRepository(db),
		objRepo,
		redisCmd,
	)

	// Try to generate QR for non-existent object
	nonExistentObjectID := uuid.New()
	_, err := qrService.GenerateQRCode(ctx, owner, nonExistentObjectID)
	if err == nil {
		t.Errorf("expected error for non-existent object")
	}
	if !errors.Is(err, services.ErrObjectNotFound) {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestGenerateQRCodeReturnsExistingWhenAlreadyGenerated(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	owner := uuid.New()
	setupOwner(t, db, owner, "owner@example.com")

	objRepo := repository.NewObjectRepository(db)
	objService := services.NewObjectService(objRepo)
	obj, err := objService.CreateObject(ctx, owner, &models.CreateObjectRequest{
		Name:       "My Car",
		ObjectType: "car",
	})
	if err != nil {
		t.Fatalf("failed to create object: %v", err)
	}

	redisCmd := setupTestRedis()
	qrService := services.NewQRCodeService(
		repository.NewQRCodeRepository(db),
		objRepo,
		redisCmd,
	)

	// Generate first QR code
	qr1, err := qrService.GenerateQRCode(ctx, owner, obj.ID)
	if err != nil {
		t.Fatalf("first GenerateQRCode failed: %v", err)
	}

	// Generate again - should return existing
	qr2, err := qrService.GenerateQRCode(ctx, owner, obj.ID)
	if err != nil {
		t.Fatalf("second GenerateQRCode failed: %v", err)
	}

	if qr1.ID != qr2.ID {
		t.Errorf("expected same QR code ID, got %s vs %s", qr1.ID, qr2.ID)
	}
	if qr1.QRToken != qr2.QRToken {
		t.Errorf("expected same QR token, got %s vs %s", qr1.QRToken, qr2.QRToken)
	}
}

func TestGetQRCode(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	owner := uuid.New()
	setupOwner(t, db, owner, "owner@example.com")

	objRepo := repository.NewObjectRepository(db)
	objService := services.NewObjectService(objRepo)
	obj, err := objService.CreateObject(ctx, owner, &models.CreateObjectRequest{
		Name:       "My Car",
		ObjectType: "car",
	})
	if err != nil {
		t.Fatalf("failed to create object: %v", err)
	}

	redisCmd := setupTestRedis()
	qrService := services.NewQRCodeService(
		repository.NewQRCodeRepository(db),
		objRepo,
		redisCmd,
	)

	// Generate QR code
	qr1, err := qrService.GenerateQRCode(ctx, owner, obj.ID)
	if err != nil {
		t.Fatalf("GenerateQRCode failed: %v", err)
	}

	// Get QR code
	qr2, err := qrService.GetQRCode(ctx, owner, obj.ID)
	if err != nil {
		t.Fatalf("GetQRCode failed: %v", err)
	}

	if qr1.ID != qr2.ID {
		t.Errorf("expected same QR code ID, got %s vs %s", qr1.ID, qr2.ID)
	}
	if qr1.QRToken != qr2.QRToken {
		t.Errorf("expected same QR token, got %s vs %s", qr1.QRToken, qr2.QRToken)
	}
}

func TestGetQRCodeNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	owner := uuid.New()
	setupOwner(t, db, owner, "owner@example.com")

	objRepo := repository.NewObjectRepository(db)
	objService := services.NewObjectService(objRepo)
	obj, err := objService.CreateObject(ctx, owner, &models.CreateObjectRequest{
		Name:       "My Car",
		ObjectType: "car",
	})
	if err != nil {
		t.Fatalf("failed to create object: %v", err)
	}

	redisCmd := setupTestRedis()
	qrService := services.NewQRCodeService(
		repository.NewQRCodeRepository(db),
		objRepo,
		redisCmd,
	)

	// Try to get QR code without generating
	_, err = qrService.GetQRCode(ctx, owner, obj.ID)
	if err == nil {
		t.Errorf("expected error for non-existent QR code")
	}
}
