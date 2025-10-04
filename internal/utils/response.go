package utils

import (
	"errors"
	"time"

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

// Response estructura genérica de respuesta API
type Response struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message,omitempty"`
	Data      interface{}  `json:"data,omitempty"`
	Error     *ErrorDetail `json:"error,omitempty"`
	Meta      *MetaInfo    `json:"meta,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// ErrorResponse estructura específica para errores
type ErrorResponseStruct struct {
	Success   bool         `json:"success"`
	Message   string       `json:"message"`
	Error     *ErrorDetail `json:"error,omitempty"`
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

// DataResponse respuesta simple con datos
type DataResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// MessageResponse respuesta simple con mensaje
type MessageResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// ===============================================
// FUNCIONES PRINCIPALES DE RESPUESTA
// ===============================================

// SuccessResponse crea una respuesta exitosa (VERSIÓN COMPATIBLE CON HANDLERS)
func SuccessResponse(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	response := DataResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	return c.Status(statusCode).JSON(response)
}

// CreatedResponse crea una respuesta de recurso creado (201) - NUEVA FUNCIÓN AGREGADA
func CreatedResponse(c *fiber.Ctx, message string, data interface{}) error {
	response := DataResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	return c.Status(fiber.StatusCreated).JSON(response)
}

// ErrorResponse crea una respuesta de error (VERSIÓN COMPATIBLE CON HANDLERS)
func ErrorResponse(c *fiber.Ctx, statusCode int, message string, details interface{}) error {
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

// ===============================================
// FUNCIONES ESPECÍFICAS POR TIPO DE ERROR
// ===============================================

// BadRequestResponse respuesta para errores 400
func BadRequestResponse(c *fiber.Ctx, message string, err error) error {
	var details interface{}
	if err != nil {
		details = err.Error()
	}
	return ErrorResponse(c, fiber.StatusBadRequest, message, details)
}

// UnauthorizedResponse respuesta para errores 401
func UnauthorizedResponse(c *fiber.Ctx, message string) error {
	return ErrorResponse(c, fiber.StatusUnauthorized, message, nil)
}

// ForbiddenResponse respuesta para errores 403
func ForbiddenResponse(c *fiber.Ctx, message string) error {
	return ErrorResponse(c, fiber.StatusForbidden, message, nil)
}

// NotFoundResponse respuesta para errores 404
func NotFoundResponse(c *fiber.Ctx, message string) error {
	return ErrorResponse(c, fiber.StatusNotFound, message, nil)
}

// InternalServerErrorResponse respuesta para errores 500
func InternalServerErrorResponse(c *fiber.Ctx, message string, err error) error {
	var details interface{}
	if err != nil {
		details = err.Error()
	}
	return ErrorResponse(c, fiber.StatusInternalServerError, message, details)
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

// ===============================================
// FUNCIONES DE COMPATIBILIDAD (SIN FIBER.CTX)
// ===============================================

// SuccessResponseSimple crea una respuesta exitosa sin context
func SuccessResponseSimple(message string, data interface{}) DataResponse {
	return DataResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// ErrorResponseSimple crea una respuesta de error sin context
func ErrorResponseSimple(message string, code string) ErrorResponseStruct {
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

// ===============================================
// FUNCIONES DE PAGINACIÓN
// ===============================================

// PaginatedResponseFunc respuesta con paginación
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

// ===============================================
// FUNCIONES DE UTILIDAD
// ===============================================

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

// HandleError maneja diferentes tipos de errores y retorna respuestas HTTP apropiadas
func HandleError(c *fiber.Ctx, err error) error {
	// Check if it's already an APIError
	if apiErr, ok := err.(*APIError); ok {
		return ErrorResponse(c, apiErr.Code, apiErr.Message, apiErr.Details)
	}

	// Handle known business logic errors
	switch err {
	case ErrUserNotFound:
		return NotFoundResponse(c, "User not found")
	case ErrUserAlreadyExists:
		return ErrorResponse(c, fiber.StatusConflict, "User already exists", nil)
	case ErrInvalidCredentials:
		return UnauthorizedResponse(c, "Invalid credentials")
	case ErrWorkflowNotFound:
		return NotFoundResponse(c, "Workflow not found")
	case ErrUnauthorized:
		return UnauthorizedResponse(c, "Unauthorized")
	case ErrForbidden:
		return ForbiddenResponse(c, "Forbidden")
	case ErrValidationFailed:
		return ErrorResponse(c, fiber.StatusUnprocessableEntity, "Validation failed", nil)
	case ErrBadRequest:
		return BadRequestResponse(c, "Bad request", nil)
	default:
		// For unknown errors, return a generic internal server error
		return InternalServerErrorResponse(c, "Internal server error", err)
	}
}

// ===============================================
// ESTRUCTURAS DE ERROR ADICIONALES
// ===============================================

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

// ValidationErrorDetail represents a field validation error
type ValidationErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   any    `json:"value,omitempty"`
}

// ValidationError represents multiple validation errors
type ValidationError struct {
	Message string                  `json:"message"`
	Errors  []ValidationErrorDetail `json:"errors"`
}

func (e ValidationError) Error() string {
	return e.Message
}

// NewValidationErrors creates a new validation error with multiple field errors
func NewValidationErrors(errors []ValidationErrorDetail) *ValidationError {
	return &ValidationError{
		Message: "Validation failed",
		Errors:  errors,
	}
}

// NewErrorResponse constructor para compatibilidad
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

//FUNCIONES PARA MIDDLEWARE (COMPATIBILIDAD)

// SuccessResponse versión simple que retorna Response struct (para middleware)
func SuccessResponseData(message string, data interface{}) DataResponse {
	return DataResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// ErrorResponse versión simple que retorna ErrorResponse struct (para middleware)
func ErrorResponseData(message string, details interface{}) ErrorResponseStruct {
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

// GetCurrentUserRole extrae el rol del usuario del contexto de Fiber
func GetCurrentUserRole(c *fiber.Ctx) (string, error) {
	userRole := c.Locals("userRole")
	if userRole == nil {
		return "", errors.New("no user role in context")
	}

	if role, ok := userRole.(string); ok {
		return role, nil
	}

	return "", errors.New("invalid user role format")
}
