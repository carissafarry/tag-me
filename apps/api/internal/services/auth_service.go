package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/repository"
)

var (
	ErrInvalidRequest      = errors.New("invalid request payload")
	ErrInvalidContact      = errors.New("invalid contact format")
	ErrInvalidOTP          = errors.New("invalid or expired otp")
	ErrTooManyAttempts     = errors.New("too many attempts, please try again later")
	ErrTooManyRequests     = errors.New("too many requests, please try again later")
	ErrContactRequired     = errors.New("contact is required")
	ErrContactTypeRequired = errors.New("contact_type is required")
	ErrOTPRequired         = errors.New("otp is required")
	ErrOTPNotFound         = errors.New("otp not found or expired")
	ErrAccountBlocked      = errors.New("account locked, contact support")
	ErrAccountDisabled     = errors.New("account disabled")
)

type AuthService struct {
	ownerRepo      *repository.OwnerRepository
	otpRepo        *repository.OTPRepository
	jwtSecret      string
	jwtExpiry      time.Duration
	maxOTPAttempts int64
}

type OTPRequest struct {
	Contact     string `json:"contact" binding:"required"`
	ContactType string `json:"contact_type" binding:"required"`
}

// OTPVerify
type OTPVerifyRequest struct {
	Contact string `json:"contact" binding:"required"`
	OTP     string `json:"otp" binding:"required"`
}

type OTPVerifyResponse struct {
	Token string        `json:"token"`
	Owner *models.Owner `json:"owner"`
}

type AuthClaims struct {
	OwnerID   string `json:"owner_id"`
	Contact   string `json:"contact"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

func NewAuthService(ownerRepo *repository.OwnerRepository, otpRepo *repository.OTPRepository, jwtSecret string, jwtExpiry time.Duration) *AuthService {
	return &AuthService{
		ownerRepo:      ownerRepo,
		otpRepo:        otpRepo,
		jwtSecret:      jwtSecret,
		jwtExpiry:      jwtExpiry,
		maxOTPAttempts: 5,
	}
}

// GenerateOTP generates a 6-digit OTP.
func (s *AuthService) GenerateOTP() (string, error) {
	max := big.NewInt(1000000)
	num, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf("%06d", num.Int64())
	return code, nil
}

// RequestOTP generates and stores OTP for contact.
// Flow:
// 1. Check cooldown (otp_request exists) → 429
// 2. Check hourly rate limit (request_count >= 3) → 429
// 3. Generate code, store, increment counter
func (s *AuthService) RequestOTP(ctx context.Context, req *OTPRequest) error {
	if req.Contact == "" {
		return ErrContactRequired
	}
	if req.ContactType == "" {
		return ErrContactTypeRequired
	}

	// Validate contact_type
	if req.ContactType != "phone" && req.ContactType != "email" {
		return ErrInvalidContact
	}

	// Check cooldown: otp_request:{contact} still exists (3-min TTL)
	existing, err := s.otpRepo.GetOTPRequest(ctx, req.Contact)
	if err != nil {
		return err
	}
	if existing != nil {
		// Request exists, still in 3-min cooldown
		return ErrTooManyRequests
	}

	// Check hourly rate limit: otp_request_count:{contact}
	count, err := s.otpRepo.IncrOTPRequestCount(ctx, req.Contact)
	if err != nil {
		return err
	}
	if count > s.otpRepo.OTPMaxRequestAttempts {
		// Exceeded maximum requests per hour
		return ErrTooManyRequests
	}

	// Generate OTP code
	code, err := s.GenerateOTP()
	if err != nil {
		return err
	}

	// Store OTP request (3 min TTL)
	if err := s.otpRepo.StoreOTPRequest(ctx, req.Contact, code); err != nil {
		return err
	}

	// TODO: Send OTP to contact (email/SMS) — out of scope for MVP
	// For testing: log code or return in non-production
	return nil
}

// VerifyOTP validates OTP, creates/retrieves owner, returns signed JWT.
// Flow:
// 1. Initialize otp_verify from otp_request if needed
// 2. Check is_blocked flag → 403
// 3. Validate code matches
// 4. On match: upsert owner, delete keys, sign JWT → 200
// 5. On mismatch: increment attempt
//   - If attempt < 3: return 401
//   - If attempt >= 3: query DB for owner existence
//   - Existing: set is_blocked + disable account → 403
//   - New: reset counter → 401 allow retry
func (s *AuthService) VerifyOTP(ctx context.Context, req *OTPVerifyRequest) (*OTPVerifyResponse, error) {
	if req.Contact == "" {
		return nil, ErrContactRequired
	}
	if req.OTP == "" {
		return nil, ErrOTPRequired
	}

	// Initialize otp_verify if it doesn't exist
	verify, err := s.otpRepo.GetVerify(ctx, req.Contact)
	if err != nil {
		return nil, err
	}
	if verify == nil {
		// Initialize from otp_request
		if err := s.otpRepo.InitVerifyOTPRedis(ctx, req.Contact); err != nil {
			// otp_request not found or expired
			return nil, err
		}
		// Fetch the newly created verify
		verify, err = s.otpRepo.GetVerify(ctx, req.Contact)
		if err != nil {
			return nil, err
		}
	}

	// Check if blocked (permanent lock from previous failed attempts)
	if verify.IsBlocked {
		return nil, ErrAccountBlocked
	}

	// Validate code matches
	if verify.Code == req.OTP {
		// OTP valid
		// Determine contact_type (infer from contact format)
		contactType := "phone"
		if isEmail(req.Contact) {
			contactType = "email"
		}

		// Upsert owner (create if not exists)
		owner, err := s.ownerRepo.UpsertByContact(ctx, req.Contact, contactType)
		if err != nil {
			return nil, err
		}

		// Delete all OTP keys (success cleanup)
		_ = s.otpRepo.DeleteAll(ctx, req.Contact)

		// Sign JWT
		token, err := s.signToken(owner)
		if err != nil {
			return nil, err
		}

		return &OTPVerifyResponse{
			Token: token,
			Owner: owner,
		}, nil
	}

	// Code doesn't match - increment verify OTP attempt
	verify, err = s.otpRepo.IncrVerifyOTPAttempt(ctx, req.Contact)
	if err != nil {
		return nil, err
	}

	// Check if exceeded max attempts (3)
	if verify.Attempt >= s.otpRepo.OTPMaxVerifyAttempts {
		// Query DB to check if owner exists
		owner, err := s.ownerRepo.GetByContact(ctx, req.Contact)
		if err != nil {
			return nil, err
		}

		if owner != nil {
			// Existing owner - lock account permanently
			if err := s.otpRepo.SetBlocked(ctx, req.Contact); err != nil {
				return nil, err
			}
			// Disable account in DB (permanent lock)
			if err := s.ownerRepo.DisableAccount(ctx, req.Contact); err != nil {
				return nil, err
			}
			return nil, ErrAccountBlocked
		}

		// New contact - reset attempt counter, allow retry with new OTP
		if err := s.otpRepo.DeleteAll(ctx, req.Contact); err != nil {
			return nil, err
		}
		return nil, ErrInvalidOTP
	}

	// Attempt < 3, allow retry
	return nil, ErrInvalidOTP
}

// signToken creates a JWT token with owner ID and contact, signed with HMAC SHA256.
func (s *AuthService) signToken(owner *models.Owner) (string, error) {
	now := time.Now()
	claims := &AuthClaims{
		OwnerID:   owner.ID.String(),
		Contact:   owner.Contact,
		ExpiresAt: now.Add(s.jwtExpiry).Unix(),
		IssuedAt:  now.Unix(),
	}

	// Encode header
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Encode claims
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Create signature
	message := headerB64 + "." + claimsB64
	h := hmac.New(sha256.New, []byte(s.jwtSecret))
	h.Write([]byte(message))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return message + "." + signature, nil
}

// isEmail is a simple heuristic; real validation should use email/phone libraries.
func isEmail(contact string) bool {
	for _, c := range contact {
		if c == '@' {
			return true
		}
	}
	return false
}
