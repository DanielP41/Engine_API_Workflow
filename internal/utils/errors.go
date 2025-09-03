package utils

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// Error types
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrWorkflowNotFound   = errors.New("workflow not found")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrValidationFailed   = errors.New("validation failed")
	ErrInternalServer     = errors.New("internal server error")
	ErrBadRequest         = errors.New("bad request")
)

// APIError represents a structured API error
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e APIError) Error() string {
	return e.Message
}

// NewAPIError creates a new API error
func NewAPIError(code int, message string, details ...string) *APIError {
	var detail string
	if len(details) > 0 {
		detail = details[0]
	}
	return &APIError{
		Code:    code,
		Message: message,
		Details: detail,
	}
}

// Common API errors
func NewBadRequestError(message string, details ...string) *APIError {
	return NewAPIError(http.StatusBadRequest, message, details...)
}

func NewUnauthorizedError(message string, details ...string) *APIError {
	return NewAPIError(http.StatusUnauthorized, message, details...)
}

func NewForbiddenError(message string, details ...string) *APIError {
	return NewAPIError(http.StatusForbidden, message, details...)
}

func NewNotFoundError(message string, details ...string) *APIError {
	return NewAPIError(http.StatusNotFound, message, details...)
}

func NewInternalServerError(message string, details ...string) *APIError {
	return NewAPIError(http.StatusInternalServerError, message, details...)
}

func NewValidationError(message string, details ...string) *APIError {
	return NewAPIError(http.StatusUnprocessableEntity, message, details...)
}

// HandleError handles different types of errors and returns appropriate HTTP responses
func HandleError(c *fiber.Ctx, err error) error {
	// Check if it's already an APIError
	if apiErr, ok := err.(*APIError); ok {
		return ErrorResponseFunc(c, apiErr.Code, apiErr.Message, apiErr.Details)
	}

	// Handle known business logic errors
	switch err {
	case ErrUserNotFound:
		return NotFoundResponse(c, "User not found")
	case ErrUserAlreadyExists:
		return ErrorResponseFunc(c, http.StatusConflict, "User already exists", "")
	case ErrInvalidCredentials:
		return UnauthorizedResponse(c, "Invalid credentials")
	case ErrWorkflowNotFound:
		return NotFoundResponse(c, "Workflow not found")
	case ErrUnauthorized:
		return UnauthorizedResponse(c, "Unauthorized")
	case ErrForbidden:
		return ForbiddenResponse(c, "Forbidden")
	case ErrValidationFailed:
		return ErrorResponseFunc(c, http.StatusUnprocessableEntity, "Validation failed", "")
	case ErrBadRequest:
		return BadRequestResponse(c, "Bad request", nil)
	default:
		// For unknown errors, log them and return a generic internal server error
		return InternalServerErrorResponse(c, "Internal server error", err)
	}
}

// ValidationErrorDetail represents a field validation error
type ValidationErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

// ValidationErrorStruct represents multiple validation errors
type ValidationErrorStruct struct {
	Message string                  `json:"message"`
	Errors  []ValidationErrorDetail `json:"errors"`
}

func (e ValidationErrorStruct) Error() string {
	return e.Message
}

// NewValidationErrors creates a new validation error with multiple field errors
func NewValidationErrors(errors []ValidationErrorDetail) *ValidationErrorStruct {
	return &ValidationErrorStruct{
		Message: "Validation failed",
		Errors:  errors,
	}
}

// WrapError wraps an error with additional context
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
