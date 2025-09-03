package utils

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// APIResponse estructura genérica de respuesta API
type APIResponse struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message,omitempty"`
	Data      interface{}  `json:"data,omitempty"`
	Error     *ErrorDetail `json:"error,omitempty"`
	Meta      *MetaInfo    `json:"meta,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// DataResponse para respuestas con datos
type DataResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// MessageResponse para respuestas solo con mensaje
type MessageResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// ErrorResponse estructura específica para errores
type ErrorResponse struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message"`
	Error     *ErrorDetail `json:"error,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// ErrorDetail detalles del error
type ErrorDetail struct {
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	Field   string      `json:"field,omitempty"`
}

// MetaInfo información adicional de la respuesta
type MetaInfo struct {
	RequestID     string      `json:"request_id,omitempty"`
	Version       string      `json:"version,omitempty"`
	ExecutionTime int64       `json:"execution_time,omitempty"`
	Pagination    interface{} `json:"pagination,omitempty"`
}

// PaginatedResponse respuesta con paginación
type PaginatedResponse struct {
	Success    bool           `json:"success"`
	Message    string         `json:"message"`
	Data       interface{}    `json:"data"`
	Pagination PaginationInfo `json:"pagination"`
	Timestamp  time.Time      `json:"timestamp"`
}

// PaginationInfo información de paginación
type PaginationInfo struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// SuccessResponse crea una respuesta exitosa con status code
func SuccessResponse(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	response := DataResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	return c.Status(statusCode).JSON(response)
}

// ErrorResponseFunc crea una respuesta de error
func ErrorResponseFunc(c *fiber.Ctx, statusCode int, message string, details interface{}) error {
	errorDetail := &ErrorDetail{
		Code:    GetErrorCode(statusCode),
		Message: message,
		Details: details,
	}

	response := ErrorResponse{
		Success:   false,
		Message:   message,
		Error:     errorDetail,
		Timestamp: time.Now(),
	}

	return c.Status(statusCode).JSON(response)
}

// SuccessResponseSimple crea una respuesta exitosa sin status code
func SuccessResponseSimple(c *fiber.Ctx, message string, data interface{}) error {
	response := DataResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	return c.JSON(response)
}

// BadRequestResponse respuesta para errores 400
func BadRequestResponse(c *fiber.Ctx, message string, err error) error {
	var details interface{}
	if err != nil {
		details = err.Error()
	}
	return ErrorResponseFunc(c, fiber.StatusBadRequest, message, details)
}

// UnauthorizedResponse respuesta para errores 401
func UnauthorizedResponse(c *fiber.Ctx, message string) error {
	return ErrorResponseFunc(c, fiber.StatusUnauthorized, message, nil)
}

// ForbiddenResponse respuesta para errores 403
func ForbiddenResponse(c *fiber.Ctx, message string) error {
	return ErrorResponseFunc(c, fiber.StatusForbidden, message, nil)
}

// NotFoundResponse respuesta para errores 404
func NotFoundResponse(c *fiber.Ctx, message string) error {
	return ErrorResponseFunc(c, fiber.StatusNotFound, message, nil)
}

// InternalServerErrorResponse respuesta para errores 500
func InternalServerErrorResponse(c *fiber.Ctx, message string, err error) error {
	var details interface{}
	if err != nil {
		details = err.Error()
	}
	return ErrorResponseFunc(c, fiber.StatusInternalServerError, message, details)
}

// ValidationErrorResponse respuesta para errores de validación
func ValidationErrorResponse(c *fiber.Ctx, message string, validationErrors interface{}) error {
	errorDetail := &ErrorDetail{
		Code:    "VALIDATION_ERROR",
		Message: message,
		Details: validationErrors,
	}

	response := ErrorResponse{
		Success:   false,
		Message:   message,
		Error:     errorDetail,
		Timestamp: time.Now(),
	}

	return c.Status(fiber.StatusBadRequest).JSON(response)
}

// PaginatedResponseFunc respuesta con paginación
func PaginatedResponseFunc(c *fiber.Ctx, message string, data interface{}, pagination PaginationInfo) error {
	response := PaginatedResponse{
		Success:    true,
		Message:    message,
		Data:       data,
		Pagination: pagination,
		Timestamp:  time.Now(),
	}

	return c.JSON(response)
}

// GetErrorCode obtiene el código de error basado en el status HTTP
func GetErrorCode(statusCode int) string {
	switch statusCode {
	case fiber.StatusBadRequest:
		return "BAD_REQUEST"
	case fiber.StatusUnauthorized:
		return "UNAUTHORIZED"
	case fiber.StatusForbidden:
		return "FORBIDDEN"
	case fiber.StatusNotFound:
		return "NOT_FOUND"
	case fiber.StatusConflict:
		return "CONFLICT"
	case fiber.StatusUnprocessableEntity:
		return "VALIDATION_ERROR"
	case fiber.StatusInternalServerError:
		return "INTERNAL_SERVER_ERROR"
	default:
		return "UNKNOWN_ERROR"
	}
}
