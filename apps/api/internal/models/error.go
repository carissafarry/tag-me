package models

// ErrorResponse is a safe error returned to scanner clients
type ErrorResponse struct {
	Code   string `json:"code"`
	Error  string `json:"error"`
	Detail string `json:"detail,omitempty"`
}
