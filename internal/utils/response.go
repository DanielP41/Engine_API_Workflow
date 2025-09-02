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
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// PaginationInfo información de paginación
type PaginationInfo struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// PaginatedResponse respuesta con paginación
type PaginatedResponse struct {
	Success    bool           `json:"success"`
	Message    string         `json:"message,omitempty"`
	Data       interface{}    `json:"data"`
	Pagination PaginationInfo `json:"pagination"`
	Timestamp  time.Time      `json:"timestamp"`
}

// ErrorResponse respuesta de error simplificada
type ErrorResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// DataResponse respuesta con datos
type DataResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// MessageResponse respuesta solo con mensaje
type MessageResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// FUNCIONES PRINCIPALES - SIGNATURE UNIFICADA

// SuccessResponse crea una respuesta exitosa
func SuccessResponse(c *fiber.Ctx, statusCode int, message string, data interface{}) error {
	response := DataResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
	return c.Status(statusCode).JSON(response)
}

// ErrorResponse crea una respuesta de error
func ErrorResponse(c *fiber.Ctx, statusCode int, message string, errorDetail string) error {
	response := ErrorResponse{
		Success:   false,
		Message:   message,
		Error:     errorDetail,
		Timestamp: time.Now(),
	}
	return c.Status(statusCode).JSON(response)
}

// FUNCIONES DE CONVENIENCIA - SIGNATURES CONSISTENTES

// SuccessResponseSimple respuesta exitosa sin status code específico
func SuccessResponseSimple(message string, data interface{}) DataResponse {
	return DataResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewErrorResponse constructor de error response
func NewErrorResponse(message string, errorDetail string) ErrorResponse {
	return ErrorResponse{
		Success:   false,
		Message:   message,
		Error:     errorDetail,
		Timestamp: time.Now(),
	}
}

// PaginatedResponseHelper crea respuesta paginada
func PaginatedResponseHelper(message string, data interface{}, pagination PaginationInfo) PaginatedResponse {
	return PaginatedResponse{
		Success:    true,
		Message:    message,
		Data:       data,
		Pagination: pagination,
		Timestamp:  time.Now(),
	}
}

// FUNCIONES ESPECÍFICAS POR STATUS CODE

// BadRequest respuesta 400
func BadRequest(c *fiber.Ctx, message string, err error) error {
	errorDetail := ""
	if err != nil {
		errorDetail = err.Error()
	}
	return ErrorResponse(c, fiber.StatusBadRequest, message, errorDetail)
}

// Unauthorized respuesta 401
func Unauthorized(c *fiber.Ctx, message string) error {
	return ErrorResponse(c, fiber.StatusUnauthorized, message, "")
}

// Forbidden respuesta 403
func Forbidden(c *fiber.Ctx, message string) error {
	return ErrorResponse(c, fiber.StatusForbidden, message, "")
}

// NotFound respuesta 404
func NotFound(c *fiber.Ctx, message string) error {
	return ErrorResponse(c, fiber.StatusNotFound, message, "")
}

// InternalServerError respuesta 500
func InternalServerError(c *fiber.Ctx, message string, err error) error {
	errorDetail := ""
	if err != nil {
		errorDetail = err.Error()
	}
	return ErrorResponse(c, fiber.StatusInternalServerError, message, errorDetail)
}

// ValidationError respuesta para errores de validación
func ValidationError(c *fiber.Ctx, message string, validationErrors interface{}) error {
	response := fiber.Map{
		"success":           false,
		"message":           message,
		"validation_errors": validationErrors,
		"timestamp":         time.Now(),
	}
	return c.Status(fiber.StatusBadRequest).JSON(response)
}

// OK respuesta 200 exitosa
func OK(c *fiber.Ctx, message string, data interface{}) error {
	return SuccessResponse(c, fiber.StatusOK, message, data)
}

// Created respuesta 201 creado
func Created(c *fiber.Ctx, message string, data interface{}) error {
	return SuccessResponse(c, fiber.StatusCreated, message, data)
}

// Accepted respuesta 202 aceptado
func Accepted(c *fiber.Ctx, message string, data interface{}) error {
	return SuccessResponse(c, fiber.StatusAccepted, message, data)
}

// NoContent respuesta 204 sin contenido
func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}
