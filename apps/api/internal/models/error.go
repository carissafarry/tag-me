package models

// ErrorResponse is a safe error returned to scanner clients
type ErrorResponse struct {
	Error  string `json:"error"`
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}
