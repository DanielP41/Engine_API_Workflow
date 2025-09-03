package utils

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator wrapper para go-playground/validator
type Validator struct {
	validate *validator.Validate
}

// NewValidator crea una nueva instancia del validador
func NewValidator() *Validator {
	return &Validator{
		validate: validator.New(),
	}
}

// Validate valida una estructura
func (v *Validator) Validate(s interface{}) error {
	return v.validate.Struct(s)
}

// ValidateStruct valida una estructura (función global para compatibilidad)
func ValidateStruct(s interface{}) error {
	validator := validator.New()
	return validator.Struct(s)
}

// ValidateVar valida una variable individual
func ValidateVar(field interface{}, tag string) error {
	validator := validator.New()
	return validator.Var(field, tag)
}

// GetValidationError convierte errores de validación a formato legible
func GetValidationError(err error) []ValidationErrorDetail {
	var validationErrors []ValidationErrorDetail

	if validationErr, ok := err.(validator.ValidationErrors); ok {
		for _, fieldError := range validationErr {
			validationErrors = append(validationErrors, ValidationErrorDetail{
				Field:   strings.ToLower(fieldError.Field()),
				Message: getValidationMessage(fieldError),
				Value:   fieldError.Value(),
			})
		}
	}

	return validationErrors
}

// getValidationMessage genera un mensaje descriptivo para el error de validación
func getValidationMessage(fieldError validator.FieldError) string {
	field := fieldError.Field()
	tag := fieldError.Tag()
	param := fieldError.Param()

	switch tag {
	case "required":
		return field + " is required"
	case "email":
		return field + " must be a valid email address"
	case "min":
		return field + " must be at least " + param + " characters long"
	case "max":
		return field + " must be at most " + param + " characters long"
	case "oneof":
		return field + " must be one of: " + param
	case "url":
		return field + " must be a valid URL"
	case "gt":
		return field + " must be greater than " + param
	case "gte":
		return field + " must be greater than or equal to " + param
	case "lt":
		return field + " must be less than " + param
	case "lte":
		return field + " must be less than or equal to " + param
	case "len":
		return field + " must be exactly " + param + " characters long"
	case "alpha":
		return field + " must contain only alphabetic characters"
	case "alphanum":
		return field + " must contain only alphanumeric characters"
	case "numeric":
		return field + " must be a valid number"
	case "uuid":
		return field + " must be a valid UUID"
	default:
		return field + " is invalid"
	}
}

// IsValidEmail verifica si un email es válido
func IsValidEmail(email string) bool {
	return ValidateVar(email, "email") == nil
}

// IsValidURL verifica si una URL es válida
func IsValidURL(url string) bool {
	return ValidateVar(url, "url") == nil
}

// ValidatePassword valida que una contraseña cumpla con los requisitos
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	return nil
}

// ValidateObjectID valida que un string sea un ObjectID válido
func ValidateObjectID(id string) error {
	return ValidateVar(id, "required,len=24,alphanum")
}

// SanitizeInput limpia y sanitiza input del usuario
func SanitizeInput(input string) string {
	// Remover espacios en blanco al inicio y final
	input = strings.TrimSpace(input)

	// Remover caracteres de control
	input = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, input)

	return input
}

// ValidateRole valida que un rol sea válido
func ValidateRole(role string) bool {
	validRoles := []string{"admin", "user"}
	for _, validRole := range validRoles {
		if role == validRole {
			return true
		}
	}
	return false
}

// ValidateWorkflowStatus valida que un status de workflow sea válido
func ValidateWorkflowStatus(status string) bool {
	validStatuses := []string{"draft", "active", "inactive", "archived"}
	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}
