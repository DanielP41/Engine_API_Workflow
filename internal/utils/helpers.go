// internal/utils/helpers.go
package utils

import (
	"fmt"
	"math"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SplitAndTrim divide una cadena por un delimitador y elimina espacios
func SplitAndTrim(s string, delimiter string) []string {
	if s == "" {
		return []string{}
	}

	parts := strings.Split(s, delimiter)
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// Contains verifica si un slice contiene un elemento
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ContainsAny verifica si un slice contiene alguno de los elementos
func ContainsAny(slice []string, items []string) bool {
	for _, item := range items {
		if Contains(slice, item) {
			return true
		}
	}
	return false
}

// RemoveEmpty elimina elementos vacíos de un slice
func RemoveEmpty(slice []string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if strings.TrimSpace(s) != "" {
			result = append(result, s)
		}
	}
	return result
}

// Unique elimina duplicados de un slice
func Unique(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))

	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

// TimeAgo retorna una representación legible del tiempo transcurrido
func TimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if diff < 30*24*time.Hour {
		weeks := int(diff.Hours() / (24 * 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if diff < 365*24*time.Hour {
		months := int(diff.Hours() / (24 * 30))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	} else {
		years := int(diff.Hours() / (24 * 365))
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// FormatDuration convierte duración en milisegundos a string legible
func FormatDuration(milliseconds int64) string {
	if milliseconds < 1000 {
		return fmt.Sprintf("%dms", milliseconds)
	}

	seconds := float64(milliseconds) / 1000.0
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	}

	minutes := seconds / 60.0
	if minutes < 60 {
		return fmt.Sprintf("%.1fm", minutes)
	}

	hours := minutes / 60.0
	return fmt.Sprintf("%.1fh", hours)
}

// FormatBytes convierte bytes a string legible
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// FormatPercentage formatea un porcentaje con decimales apropiados
func FormatPercentage(value float64) string {
	if value >= 99.9 {
		return "99.9%"
	} else if value >= 10 {
		return fmt.Sprintf("%.1f%%", value)
	} else {
		return fmt.Sprintf("%.2f%%", value)
	}
}

// TruncateString trunca una cadena a la longitud máxima especificada
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// SafeStringPtr retorna un puntero a string si no está vacío, nil en caso contrario
func SafeStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// SafeIntPtr retorna un puntero a int si es mayor que 0, nil en caso contrario
func SafeIntPtr(i int) *int {
	if i <= 0 {
		return nil
	}
	return &i
}

// SafeTimePtr retorna un puntero a time si no es zero, nil en caso contrario
func SafeTimePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// CoalesceString retorna el primer valor no vacío
func CoalesceString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// CoalesceInt retorna el primer valor mayor que 0
func CoalesceInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

// MapKeys retorna las claves de un map
func MapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// MapValues retorna los valores de un map
func MapValues(m map[string]interface{}) []interface{} {
	values := make([]interface{}, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// IsValidTimeRange verifica si un time_range es válido para el dashboard
func IsValidTimeRange(timeRange string) bool {
	validRanges := []string{"1h", "6h", "12h", "24h", "7d", "30d"}
	return Contains(validRanges, timeRange)
}

// ParseTimeRange convierte un string de time_range a time.Duration
func ParseTimeRange(timeRange string) time.Duration {
	switch timeRange {
	case "1h":
		return 1 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// GetStartOfDay retorna el inicio del día para una fecha dada
func GetStartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// GetStartOfWeek retorna el inicio de la semana para una fecha dada
func GetStartOfWeek(t time.Time) time.Time {
	weekday := t.Weekday()
	days := int(weekday)
	return GetStartOfDay(t.AddDate(0, 0, -days))
}

// GetStartOfMonth retorna el inicio del mes para una fecha dada
func GetStartOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// IsBusinessHour verifica si una hora está dentro del horario comercial
func IsBusinessHour(t time.Time) bool {
	hour := t.Hour()
	weekday := t.Weekday()

	// Lunes a Viernes, 9 AM a 6 PM
	return weekday >= time.Monday && weekday <= time.Friday && hour >= 9 && hour < 18
}

// CalculatePercentageChange calcula el cambio porcentual entre dos valores
func CalculatePercentageChange(oldValue, newValue float64) float64 {
	if oldValue == 0 {
		if newValue == 0 {
			return 0
		}
		return 100 // 100% increase from 0
	}
	return ((newValue - oldValue) / oldValue) * 100
}

// RoundToDecimals redondea un número a la cantidad especificada de decimales
func RoundToDecimals(value float64, decimals int) float64 {
	multiplier := math.Pow(10, float64(decimals))
	return math.Round(value*multiplier) / multiplier
}

// ClampInt asegura que un entero esté dentro de un rango
func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampFloat64 asegura que un float64 esté dentro de un rango
func ClampFloat64(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// GenerateID genera un ID único para alertas y otros elementos
func GenerateID() string {
	return primitive.NewObjectID().Hex()
}

// IsValidObjectID verifica si una cadena es un ObjectID válido de MongoDB
func IsValidObjectID(id string) bool {
	_, err := primitive.ObjectIDFromHex(id)
	return err == nil
}

// StringSliceToMap convierte un slice de strings a un map para búsquedas rápidas
func StringSliceToMap(slice []string) map[string]bool {
	m := make(map[string]bool, len(slice))
	for _, s := range slice {
		m[s] = true
	}
	return m
}
