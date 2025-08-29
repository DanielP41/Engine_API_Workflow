package utils

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10" //es un validador de estructuras (structs) y campos de Go que utiliza etiquetas (tags) para definir las reglas de validación.
)

// Validator instance global
var validate *validator.Validate

// Init inicializa el validador global
func init() {
	validate = validator.New()
}

// ValidateStruct valida una estructura usando tags de validación
func ValidateStruct(s interface{}) error {
	if err := validate.Struct(s); err != nil {
		var errorMessages []string

		// Convertir errores de validación a mensajes legibles
		for _, err := range err.(validator.ValidationErrors) {
			errorMessages = append(errorMessages, formatValidationError(err))
		}

		return fmt.Errorf(strings.Join(errorMessages, "; "))
	}
	return nil
}

// formatValidationError convierte un error de validación en un mensaje legible
func formatValidationError(err validator.FieldError) string {
	field := strings.ToLower(err.Field())

	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters long", field, err.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters long", field, err.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, err.Param())
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

// IsValidEmail valida si una cadena es un email válido
func IsValidEmail(email string) bool {
	err := validate.Var(email, "required,email")
	return err == nil
}

// IsValidPassword valida si una contraseña cumple los requisitos mínimos
func IsValidPassword(password string) bool {
	err := validate.Var(password, "required,min=6,max=100")
	return err == nil
}
