package domain

import "errors"

var (
	// ErrInvalidCredentials is returned for an unknown login and a wrong password alike.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrLoginTaken is returned by account creation for an already used normalized login.
	ErrLoginTaken = errors.New("login taken")
	// ErrPrincipalNotFound is returned when no account has the requested ID.
	// For a found session this means inconsistent data, not a normal outcome.
	ErrPrincipalNotFound = errors.New("principal not found")
	// ErrSessionNotFound is returned when no session matches a token hash.
	ErrSessionNotFound = errors.New("session not found")
	// ErrNotImplemented marks scaffold operations.
	ErrNotImplemented = errors.New("not implemented")
)

// FieldError is the first validation error of a request field.
// Message must not repeat the entered password.
type FieldError struct {
	Field   string
	Code    string
	Message string
}

// ValidationError reports a malformed request, one entry per invalid field.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	return "validation failed"
}
