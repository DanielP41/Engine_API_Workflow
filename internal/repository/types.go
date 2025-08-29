package repository

import (
	"Engine_API_Workflow/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LogSearchFilter filtros para búsqueda de logs
type LogSearchFilter struct {
	WorkflowID  *primitive.ObjectID    `json:"workflow_id,omitempty"`
	UserID      *primitive.ObjectID    `json:"user_id,omitempty"`
	Status      *models.WorkflowStatus `json:"status,omitempty"`
	TriggerType *models.TriggerType    `json:"trigger_type,omitempty"`
	StartDate   *time.Time             `json:"start_date,omitempty"`
	EndDate     *time.Time             `json:"end_date,omitempty"`
	Source      *string                `json:"source,omitempty"`
	Environment *string                `json:"environment,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	HasErrors   *bool                  `json:"has_errors,omitempty"`
	MinDuration *int64                 `json:"min_duration,omitempty"`
	MaxDuration *int64                 `json:"max_duration,omitempty"`
	Search      *string                `json:"search,omitempty"`
}

// WorkflowSearchFilters filtros para búsqueda de workflows
type WorkflowSearchFilters struct {
	UserID        *primitive.ObjectID    `json:"user_id,omitempty"`
	Status        *models.WorkflowStatus `json:"status,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Search        *string                `json:"search,omitempty"`
	CreatedAfter  *time.Time             `json:"created_after,omitempty"`
	CreatedBefore *time.Time             `json:"created_before,omitempty"`
	TriggerType   *models.TriggerType    `json:"trigger_type,omitempty"`
	Environment   *string                `json:"environment,omitempty"`
	IsActive      *bool                  `json:"is_active,omitempty"`
}

// PaginationOptions opciones de paginación
type PaginationOptions struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
	SortBy    string `json:"sort_by"`
	SortOrder string `json:"sort_order"`
}

// SortOptions opciones de ordenamiento
type SortOptions struct {
	Field string `json:"field"`
	Order int    `json:"order"` // 1 para ASC, -1 para DESC
}

// SearchOptions opciones de búsqueda avanzada
type SearchOptions struct {
	Query         string   `json:"query"`
	Fields        []string `json:"fields"`
	CaseSensitive bool     `json:"case_sensitive"`
	ExactMatch    bool     `json:"exact_match"`
}

// AggregationOptions opciones para agregaciones
type AggregationOptions struct {
	GroupBy    string                 `json:"group_by"`
	Operations []AggregationOperation `json:"operations"`
	Having     map[string]interface{} `json:"having,omitempty"`
}

// AggregationOperation operación de agregación
type AggregationOperation struct {
	Field     string `json:"field"`
	Operation string `json:"operation"` // sum, avg, count, max, min
	Alias     string `json:"alias,omitempty"`
}

// FilterOptions opciones generales de filtrado
type FilterOptions struct {
	Filters    map[string]interface{} `json:"filters"`
	DateRange  *DateRangeFilter       `json:"date_range,omitempty"`
	TextSearch *TextSearchFilter      `json:"text_search,omitempty"`
}

// DateRangeFilter filtro por rango de fechas
type DateRangeFilter struct {
	Field string     `json:"field"`
	From  *time.Time `json:"from,omitempty"`
	To    *time.Time `json:"to,omitempty"`
}

// TextSearchFilter filtro de búsqueda de texto
type TextSearchFilter struct {
	Fields  []string          `json:"fields"`
	Query   string            `json:"query"`
	Options TextSearchOptions `json:"options"`
}

// TextSearchOptions opciones de búsqueda de texto
type TextSearchOptions struct {
	CaseSensitive bool `json:"case_sensitive"`
	WholeWords    bool `json:"whole_words"`
	Fuzzy         bool `json:"fuzzy"`
}
