package services

import (
	"errors"
)

var (
	ErrInvalidRequest             = errors.New("invalid request payload")
	ErrInvalidContact             = errors.New("invalid contact format")
	ErrInvalidOTP                 = errors.New("invalid or expired otp")
	ErrTooManyRequests            = errors.New("too many requests, please try again later")
	ErrContactRequired            = errors.New("contact is required")
	ErrContactTypeRequired        = errors.New("contact_type is required")
	ErrOTPRequired                = errors.New("otp is required")
	ErrOTPNotFound                = errors.New("otp not found or expired")
	ErrAccountBlocked             = errors.New("account locked, contact support")
	ErrAccountDisabled            = errors.New("account disabled")
	ErrQRCodeGenerationInProgress = errors.New("qr code generation already in progress for this object")
	ErrObjectNotFound             = errors.New("object not found")
	ErrObjectNotOwned             = errors.New("object not owned by user")
	ErrConversationActive         = errors.New("please resolve conversations before deleting this object")
)
