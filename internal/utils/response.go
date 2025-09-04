package utils

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// Response estructura genérica de respuesta API
type Response struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ErrorDetail detalles del error
type ErrorDetail struct {
	Code    string      `json:"code"`
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

// PaginationInfo información de paginación
type PaginationInfo struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// TIPOS PARA COMPATIBILIDAD
type DataResponse = Response
type MessageResponse = Response
type APIResponse = Response

// ErrorResponse tipo para respuestas de error
type ErrorResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// PaginatedResponse estructura para respuestas paginadas
type PaginatedResponse struct {
	Data       interface{}    `json:"data"`
	Pagination PaginationInfo `json:"pagination"`
}

// NewErrorResponse crea una nueva respuesta de error
func NewErrorResponse(message string, details string) ErrorResponse {
	return ErrorResponse{
		Success:   false,
		Message:   message,
		Error:     details,
		Timestamp: time.Now(),
	}
}

// SuccessResponse crea una respuesta exitosa
func SuccessResponse(message string, data interface{}) Response {
	return Response{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// ErrorResponseFunc función para crear respuesta de error con fiber.Ctx
func ErrorResponseFunc(c *fiber.Ctx, statusCode int, message string, details string) error {
	response := ErrorResponse{
		Success:   false,
		Message:   message,
		Error:     details,
		Timestamp: time.Now(),
	}
	return c.Status(statusCode).JSON(response)
}

// SuccessResponseFunc función para crear respuesta exitosa con fiber.Ctx
func SuccessResponseFunc(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	response := Response{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	return c.Status(statusCode).JSON(response)
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

// ValidationErrorResponse respuesta para errores de validación
func ValidationErrorResponse(c *fiber.Ctx, message string, validationErrors interface{}) error {
	response := ErrorResponse{
		Success:   false,
		Message:   message,
		Error:     "Validation failed",
		Timestamp: time.Now(),
	}
	return c.Status(fiber.StatusBadRequest).JSON(response)
}
