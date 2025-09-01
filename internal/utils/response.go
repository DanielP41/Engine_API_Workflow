package utils

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// FUNCIONES FALTANTES PARA COMPATIBILIDAD

// ErrorResponse crea una respuesta de error (función simple que coincide con el uso en middleware)
func ErrorResponse(message string, details string) ErrorResponseStruct {
	return ErrorResponseStruct{
		Success: false,
		Message: message,
		Error: &ErrorDetail{
			Code:    "ERROR",
			Message: message,
			Details: details,
		},
		Timestamp: time.Now(),
	}
}

// SuccessResponse crea una respuesta exitosa (función simple)
func SuccessResponse(message string, data interface{}) Response {
	return Response{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// TIPOS NECESARIOS PARA COMPATIBILIDAD

// PaginatedResponse respuesta con paginación
type PaginatedResponse struct {
	Data       interface{}    `json:"data"`
	Pagination PaginationInfo `json:"pagination"`
}

// PaginationInfo información de paginación
type PaginationInfo struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// DataResponse para respuestas con datos
type DataResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// MessageResponse para respuestas con solo mensaje
type MessageResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// FUNCIONES FALTANTES ESPECÍFICAS PARA LOS HANDLERS

// NewSuccessResponse crea una respuesta exitosa
func NewSuccessResponse(message string, data interface{}) DataResponse {
	return DataResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewMessageResponse crea una respuesta con solo mensaje
func NewMessageResponse(message string) MessageResponse {
	return MessageResponse{
		Success:   true,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// Response estructura genérica de respuesta API (mantener la existente)
type Response struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message,omitempty"`
	Data      interface{}  `json:"data,omitempty"`
	Error     *ErrorDetail `json:"error,omitempty"`
	Meta      *MetaInfo    `json:"meta,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// ErrorResponseStruct estructura específica para errores (mantener la existente)
type ErrorResponseStruct struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message"`
	Error     *ErrorDetail `json:"error"`
	Timestamp time.Time    `json:"timestamp"`
}

// ErrorDetail detalles del error (mantener la existente)
type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	Field   string      `json:"field,omitempty"`
}

// MetaInfo información adicional de la respuesta (mantener la existente)
type MetaInfo struct {
	RequestID     string      `json:"request_id,omitempty"`
	Version       string      `json:"version,omitempty"`
	ExecutionTime int64       `json:"execution_time,omitempty"`
	Pagination    interface{} `json:"pagination,omitempty"`
}

// SuccessResponse crea una respuesta exitosa con status code (compatible con auth.go)
func SuccessResponseWithCode(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	response := Response{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	return c.Status(statusCode).JSON(response)
}

// ErrorResponse crea una respuesta de error (compatible con auth.go)
func ErrorResponseWithCode(c *fiber.Ctx, statusCode int, message string, details string) error {
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

// ErrorResponseWithCodeAndError crea una respuesta de error con código HTTP específico
func ErrorResponseWithCodeAndError(c *fiber.Ctx, statusCode int, message string, err error) error {
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
	return ErrorResponseWithCodeAndError(c, fiber.StatusBadRequest, message, err)
}

// UnauthorizedResponse respuesta para errores 401
func UnauthorizedResponse(c *fiber.Ctx, message string) error {
	return ErrorResponseWithCodeAndError(c, fiber.StatusUnauthorized, message, nil)
}

// ForbiddenResponse respuesta para errores 403
func ForbiddenResponse(c *fiber.Ctx, message string) error {
	return ErrorResponseWithCodeAndError(c, fiber.StatusForbidden, message, nil)
}

// NotFoundResponse respuesta para errores 404
func NotFoundResponse(c *fiber.Ctx, message string) error {
	return ErrorResponseWithCodeAndError(c, fiber.StatusNotFound, message, nil)
}

// InternalServerErrorResponse respuesta para errores 500
func InternalServerErrorResponse(c *fiber.Ctx, message string, err error) error {
	return ErrorResponseWithCodeAndError(c, fiber.StatusInternalServerError, message, err)
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
func PaginatedResponseFunc(c *fiber.Ctx, message string, data interface{}, pagination interface{}) error {
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

// NewErrorResponse crea un ErrorResponse (constructor correcto)
func NewErrorResponse(message string, code string) ErrorResponseStruct {
	return ErrorResponseStruct{
		Success: false,
		Message: message,
		Error: &ErrorDetail{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now(),
	}
}

// WithDetails agrega detalles adicionales al ErrorResponse
func (e ErrorResponseStruct) WithDetails(details interface{}) ErrorResponseStruct {
	if e.Error != nil {
		e.Error.Details = details
	}
	return e
}

// WithField agrega campo específico al ErrorResponse
func (e ErrorResponseStruct) WithField(field string) ErrorResponseStruct {
	if e.Error != nil {
		e.Error.Field = field
	}
	return e
}
