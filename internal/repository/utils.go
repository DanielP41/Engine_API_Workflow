package repository

import "go.mongodb.org/mongo-driver/bson"

// Helper functions para verificar tipos de errores
func IsNotFoundError(err error) bool {
	if repoErr, ok := err.(*RepositoryError); ok {
		return repoErr.Code == "NOTIFICATION_NOT_FOUND" ||
			repoErr.Code == "TEMPLATE_NOT_FOUND" ||
			repoErr.Code == "USER_NOT_FOUND" ||
			repoErr.Code == "WORKFLOW_NOT_FOUND"
	}
	return false
}

func IsAlreadyExistsError(err error) bool {
	if repoErr, ok := err.(*RepositoryError); ok {
		return repoErr.Code == "TEMPLATE_ALREADY_EXISTS" ||
			repoErr.Code == "USER_ALREADY_EXISTS" ||
			repoErr.Code == "WORKFLOW_ALREADY_EXISTS"
	}
	return false
}

func IsValidationError(err error) bool {
	if repoErr, ok := err.(*RepositoryError); ok {
		return repoErr.Code == "INVALID_NOTIFICATION_DATA" ||
			repoErr.Code == "INVALID_TEMPLATE_DATA" ||
			repoErr.Code == "VALIDATION_FAILED"
	}
	return false
}

// Helper function para extraer int64 de BSON (para estadísticas)
func getInt64FromBSON(doc bson.M, key string) int64 {
	if value, ok := doc[key]; ok {
		switch v := value.(type) {
		case int64:
			return v
		case int32:
			return int64(v)
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

// Helper function para extraer string de BSON
func getStringFromBSON(doc bson.M, key string) string {
	if value, ok := doc[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// Helper function para extraer float64 de BSON
func getFloat64FromBSON(doc bson.M, key string) float64 {
	if value, ok := doc[key]; ok {
		switch v := value.(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		case int32:
			return float64(v)
		case int:
			return float64(v)
		}
	}
	return 0.0
}

// DefaultPaginationOptions obtiene opciones de paginación por defecto
func DefaultPaginationOptions() *PaginationOptions {
	return &PaginationOptions{
		Page:     1,
		PageSize: 50,
		SortBy:   "created_at",
		SortDesc: true,
	}
}

// NewRepositoryError crea un nuevo error del repositorio
func NewRepositoryError(code, message string) *RepositoryError {
	return &RepositoryError{
		Code:    code,
		Message: message,
	}
}

// NewRepositoryErrorWithDetails crea un nuevo error con detalles adicionales
func NewRepositoryErrorWithDetails(code, message string, err error) *RepositoryError {
	return &RepositoryError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
