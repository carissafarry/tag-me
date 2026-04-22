package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type OTPRepository struct {
	client *redis.Client
}

// OTPRequest stores OTP code + metadata
type OTPRequest struct {
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
}

// OTPVerify stores attempt tracking + block flag
type OTPVerify struct {
	Code      string    `json:"code"`
	Attempt   int64     `json:"attempt"`
	CreatedAt time.Time `json:"created_at"`
	IsBlocked bool      `json:"is_blocked"`
}

func NewOTPRepository(client *redis.Client) *OTPRepository {
	return &OTPRepository{client: client}
}

// StoreOTPRequest stores OTP request (3 min TTL)
func (r *OTPRepository) StoreOTPRequest(ctx context.Context, contact, code string) error {
	key := fmt.Sprintf("otp_request:%s", contact)
	req := OTPRequest{
		Code:      code,
		CreatedAt: time.Now(),
	}
	data, _ := json.Marshal(req)
	return r.client.Set(ctx, key, data, 3*time.Minute).Err()
}

// GetOTPRequest retrieves OTP request
func (r *OTPRepository) GetOTPRequest(ctx context.Context, contact string) (*OTPRequest, error) {
	key := fmt.Sprintf("otp_request:%s", contact)
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Key expired or doesn't exist
	}
	if err != nil {
		return nil, err
	}

	var req OTPRequest
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// DeleteOTPRequest removes OTP request
func (r *OTPRepository) DeleteOTPRequest(ctx context.Context, contact string) error {
	key := fmt.Sprintf("otp_request:%s", contact)
	return r.client.Del(ctx, key).Err()
}

// IncrOTPRequestCount increments hourly request counter, returns count
func (r *OTPRepository) IncrOTPRequestCount(ctx context.Context, contact string) (int64, error) {
	key := fmt.Sprintf("otp_request_count:%s", contact)
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	// Set TTL if first increment (1 hour)
	if count == 1 {
		_ = r.client.Expire(ctx, key, 60*time.Minute).Err()
	}
	return count, nil
}

// InitVerifyOTPRedis initializes otp_verify from otp_request (3 min TTL)
func (r *OTPRepository) InitVerifyOTPRedis(ctx context.Context, contact string) error {
	// Get OTP request
	req, err := r.GetOTPRequest(ctx, contact)
	if err != nil {
		return err
	}
	if req == nil {
		return fmt.Errorf("otp_request not found")
	}

	key := fmt.Sprintf("otp_verify:%s", contact)
	verify := OTPVerify{
		Code:      req.Code,
		Attempt:   0,
		CreatedAt: time.Now(),
		IsBlocked: false,
	}
	data, _ := json.Marshal(verify)
	return r.client.Set(ctx, key, data, 3*time.Minute).Err() 
}

// GetVerify retrieves otp_verify
func (r *OTPRepository) GetVerify(ctx context.Context, contact string) (*OTPVerify, error) {
	key := fmt.Sprintf("otp_verify:%s", contact)
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var verify OTPVerify
	if err := json.Unmarshal([]byte(data), &verify); err != nil {
		return nil, err
	}
	return &verify, nil
}

// IncrVerifyOTPAttempt increments attempt counter in otp_verify, returns updated struct
func (r *OTPRepository) IncrVerifyOTPAttempt(ctx context.Context, contact string) (*OTPVerify, error) {
	key := fmt.Sprintf("otp_verify:%s", contact)
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var verify OTPVerify
	if err := json.Unmarshal([]byte(data), &verify); err != nil {
		return nil, err
	}

	verify.Attempt++
	newData, _ := json.Marshal(verify)
	if err := r.client.Set(ctx, key, newData, 3*time.Minute).Err(); err != nil {
		return nil, err
	}
	return &verify, nil
}

// SetBlocked sets is_blocked flag in otp_verify
func (r *OTPRepository) SetBlocked(ctx context.Context, contact string) error {
	key := fmt.Sprintf("otp_verify:%s", contact)
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	var verify OTPVerify
	if err := json.Unmarshal([]byte(data), &verify); err != nil {
		return err
	}

	verify.IsBlocked = true
	newData, _ := json.Marshal(verify)
	return r.client.Set(ctx, key, newData, 3*time.Minute).Err()
}

// ResetAttempt resets attempt counter to 0 in otp_verify
func (r *OTPRepository) ResetAttempt(ctx context.Context, contact string) error {
	key := fmt.Sprintf("otp_verify:%s", contact)
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil // Key expired, nothing to reset
	}
	if err != nil {
		return err
	}

	var verify OTPVerify
	if err := json.Unmarshal([]byte(data), &verify); err != nil {
		return err
	}

	verify.Attempt = 0
	newData, _ := json.Marshal(verify)
	return r.client.Set(ctx, key, newData, 3*time.Minute).Err()
}

// DeleteAll removes all 3 OTP keys (on successful verify)
func (r *OTPRepository) DeleteAll(ctx context.Context, contact string) error {
	keys := []string{
		fmt.Sprintf("otp_request:%s", contact),
		fmt.Sprintf("otp_request_count:%s", contact),
		fmt.Sprintf("otp_verify:%s", contact),
	}
	return r.client.Del(ctx, keys...).Err()
}
