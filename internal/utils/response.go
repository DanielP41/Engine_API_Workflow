package utils

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// Response estructura genérica de respuesta API
type Response struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message,omitempty"`
	Data      interface{}  `json:"data,omitempty"`
	Error     *ErrorDetail `json:"error,omitempty"`
	Meta      *MetaInfo    `json:"meta,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// ErrorResponseStruct estructura específica para errores
type ErrorResponseStruct struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message"`
	Error     *ErrorDetail `json:"error"`
	Timestamp time.Time    `json:"timestamp"`
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

// PaginatedResponse estructura para respuestas paginadas
type PaginatedResponse struct {
	Data       interface{}    `json:"data"`
	Pagination PaginationInfo `json:"pagination"`
}

// PaginationInfo información de paginación
type PaginationInfo struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// DataResponse estructura para respuestas con datos
type DataResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// ErrorResponse estructura para respuestas de error
type ErrorResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// MessageResponse estructura para respuestas simples
type MessageResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// APIResponse estructura genérica de API
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// AGREGADO: Función NewErrorResponse que falta en trigger.go
func NewErrorResponse(message string, details string) ErrorResponse {
	return ErrorResponse{
		Success:   false,
		Message:   message,
		Error:     details,
		Timestamp: time.Now(),
	}
}

// SuccessResponse crea una respuesta exitosa con status code (compatible con auth.go)
func SuccessResponse(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	response := Response{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	return c.Status(statusCode).JSON(response)
}

// ErrorResponse crea una respuesta de error (compatible con auth.go)
func ErrorResponse(c *fiber.Ctx, statusCode int, message string, details string) error {
	errorDetail := &ErrorDetail{
		Code:    GetErrorCode(statusCode),
		Message: message,
		Details: details,
	}

	response := ErrorResponseStruct{
		Success:   false,
		Message:   message,
		Error:     errorDetail,
		Timestamp: time.Now(),
	}

	return c.Status(statusCode).JSON(response)
}

// SuccessResponseSimple crea una respuesta exitosa sin status code
func SuccessResponseSimple(c *fiber.Ctx, message string, data interface{}) error {
	response := Response{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	return c.JSON(response)
}

// ErrorResponseWithCode crea una respuesta de error con código HTTP específico
func ErrorResponseWithCode(c *fiber.Ctx, statusCode int, message string, err error) error {
	errorDetail := &ErrorDetail{
		Code:    GetErrorCode(statusCode),
		Message: message,
	}

	if err != nil {
		errorDetail.Details = err.Error()
	}

	response := ErrorResponseStruct{
		Success:   false,
		Message:   message,
		Error:     errorDetail,
		Timestamp: time.Now(),
	}

	return c.Status(statusCode).JSON(response)
}

// BadRequestResponse respuesta para errores 400
func BadRequestResponse(c *fiber.Ctx, message string, err error) error {
	return ErrorResponseWithCode(c, fiber.StatusBadRequest, message, err)
}

// UnauthorizedResponse respuesta para errores 401
func UnauthorizedResponse(c *fiber.Ctx, message string) error {
	return ErrorResponseWithCode(c, fiber.StatusUnauthorized, message, nil)
}

// ForbiddenResponse respuesta para errores 403
func ForbiddenResponse(c *fiber.Ctx, message string) error {
	return ErrorResponseWithCode(c, fiber.StatusForbidden, message, nil)
}

// NotFoundResponse respuesta para errores 404
func NotFoundResponse(c *fiber.Ctx, message string) error {
	return ErrorResponseWithCode(c, fiber.StatusNotFound, message, nil)
}

// InternalServerErrorResponse respuesta para errores 500
func InternalServerErrorResponse(c *fiber.Ctx, message string, err error) error {
	return ErrorResponseWithCode(c, fiber.StatusInternalServerError, message, err)
}

// ValidationErrorResponse respuesta para errores de validación
func ValidationErrorResponse(c *fiber.Ctx, message string, validationErrors interface{}) error {
	errorDetail := &ErrorDetail{
		Code:    "VALIDATION_ERROR",
		Message: message,
		Details: validationErrors,
	}

	response := ErrorResponseStruct{
		Success:   false,
		Message:   message,
		Error:     errorDetail,
		Timestamp: time.Now(),
	}

	return c.Status(fiber.StatusBadRequest).JSON(response)
}

// PaginatedResponse respuesta con paginación
func PaginatedResponse(c *fiber.Ctx, message string, data interface{}, pagination interface{}) error {
	meta := &MetaInfo{
		Pagination: pagination,
	}

	response := Response{
		Success:   true,
		Message:   message,
		Data:      data,
		Meta:      meta,
		Timestamp: time.Now(),
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

// WithDetails agrega detalles adicionales al ErrorResponse
func (e ErrorResponse) WithDetails(details interface{}) ErrorResponse {
	e.Error = details.(string)
	return e
}

// WithField agrega campo específico al ErrorResponse
func (e ErrorResponse) WithField(field string) ErrorResponse {
	// Esta función necesita ser implementada según las necesidades
	return e
}
