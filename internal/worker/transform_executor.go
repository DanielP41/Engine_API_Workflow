package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"Engine_API_Workflow/internal/models"
)

// TransformActionExecutor ejecuta transformaciones de datos
type TransformActionExecutor struct {
	logger *zap.Logger
}

// TransformConfig configuración para transformaciones
type TransformConfig struct {
	Operation string                 `json:"operation"` // "map", "filter", "template", "extract"
	Source    string                 `json:"source"`    // Campo fuente o "input"
	Target    string                 `json:"target"`    // Campo destino o "output"
	Rules     []TransformRule        `json:"rules"`     // Reglas de transformación
	Template  string                 `json:"template"`  // Template para operación template
	Filter    map[string]interface{} `json:"filter"`    // Filtros para operación filter
}

// TransformRule regla individual de transformación
type TransformRule struct {
	Field     string      `json:"field"`   // Campo a transformar
	FromValue interface{} `json:"from"`    // Valor original (opcional)
	ToValue   interface{} `json:"to"`      // Valor destino
	Type      string      `json:"type"`    // "replace", "format", "extract", "calculate"
	Pattern   string      `json:"pattern"` // Regex pattern para extract
	Format    string      `json:"format"`  // Formato para operación format
}

// NewTransformActionExecutor crea un nuevo ejecutor de transformaciones
func NewTransformActionExecutor(logger *zap.Logger) *TransformActionExecutor {
	return &TransformActionExecutor{
		logger: logger,
	}
}

// Execute ejecuta la transformación de datos
func (t *TransformActionExecutor) Execute(ctx context.Context, step *models.WorkflowStep, execCtx *ExecutionContext) (*StepResult, error) {
	startTime := time.Now()

	result := &StepResult{
		Success: true,
		Output:  make(map[string]interface{}),
	}

	t.logger.Info("Executing Transform action",
		zap.String("step_id", step.ID),
		zap.String("step_name", step.Name))

	// Parsear configuración
	config, err := t.parseTransformConfig(step.Config)
	if err != nil {
		return nil, fmt.Errorf("invalid transform configuration: %w", err)
	}

	// Obtener datos de entrada
	var inputData interface{}
	if config.Source == "input" || config.Source == "" {
		inputData = execCtx.Variables
	} else {
		inputData = execCtx.Variables[config.Source]
	}

	// Ejecutar transformación según el tipo de operación
	var transformedData interface{}
	switch config.Operation {
	case "map":
		transformedData, err = t.executeMapOperation(inputData, config)
	case "filter":
		transformedData, err = t.executeFilterOperation(inputData, config)
	case "template":
		transformedData, err = t.executeTemplateOperation(inputData, config)
	case "extract":
		transformedData, err = t.executeExtractOperation(inputData, config)
	case "format":
		transformedData, err = t.executeFormatOperation(inputData, config)
	default:
		return nil, fmt.Errorf("unsupported transform operation: %s", config.Operation)
	}

	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, err
	}

	// Guardar resultado
	if config.Target == "output" || config.Target == "" {
		result.Output["transformed_data"] = transformedData
	} else {
		result.Output[config.Target] = transformedData
	}

	result.Output["operation"] = config.Operation
	result.Output["source"] = config.Source
	result.Output["target"] = config.Target
	result.Output["execution_time_ms"] = time.Since(startTime).Milliseconds()

	t.logger.Info("Transform action completed",
		zap.String("step_id", step.ID),
		zap.String("operation", config.Operation),
		zap.Duration("duration", time.Since(startTime)))

	return result, nil
}

// parseTransformConfig parsea la configuración de transformación
func (t *TransformActionExecutor) parseTransformConfig(config map[string]interface{}) (*TransformConfig, error) {
	configBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var transformConfig TransformConfig
	err = json.Unmarshal(configBytes, &transformConfig)
	if err != nil {
		return nil, err
	}

	// Valores por defecto
	if transformConfig.Operation == "" {
		transformConfig.Operation = "map"
	}
	if transformConfig.Source == "" {
		transformConfig.Source = "input"
	}
	if transformConfig.Target == "" {
		transformConfig.Target = "output"
	}

	return &transformConfig, nil
}

// executeMapOperation ejecuta operación de mapeo de datos
func (t *TransformActionExecutor) executeMapOperation(data interface{}, config *TransformConfig) (interface{}, error) {
	if len(config.Rules) == 0 {
		return data, nil
	}

	// Convertir datos a mapa para manipulación
	dataMap, err := t.toMap(data)
	if err != nil {
		return nil, fmt.Errorf("cannot convert data to map: %w", err)
	}

	result := make(map[string]interface{})

	// Aplicar reglas de mapeo
	for _, rule := range config.Rules {
		value, exists := dataMap[rule.Field]
		if !exists {
			continue
		}

		var transformedValue interface{}
		switch rule.Type {
		case "replace":
			transformedValue = t.replaceValue(value, rule.FromValue, rule.ToValue)
		case "format":
			transformedValue = t.formatValue(value, rule.Format)
		case "extract":
			transformedValue = t.extractValue(value, rule.Pattern)
		case "calculate":
			transformedValue = t.calculateValue(value, rule.Format)
		default:
			transformedValue = value
		}

		result[rule.Field] = transformedValue
	}

	return result, nil
}

// executeFilterOperation ejecuta operación de filtrado
func (t *TransformActionExecutor) executeFilterOperation(data interface{}, config *TransformConfig) (interface{}, error) {
	dataMap, err := t.toMap(data)
	if err != nil {
		return nil, fmt.Errorf("cannot convert data to map: %w", err)
	}

	result := make(map[string]interface{})

	// Aplicar filtros
	for key, value := range dataMap {
		include := true

		// Verificar filtros
		for filterKey, filterValue := range config.Filter {
			if key == filterKey {
				include = t.matchesFilter(value, filterValue)
				break
			}
		}

		if include {
			result[key] = value
		}
	}

	return result, nil
}

// executeTemplateOperation ejecuta operación de template
func (t *TransformActionExecutor) executeTemplateOperation(data interface{}, config *TransformConfig) (interface{}, error) {
	if config.Template == "" {
		return "", fmt.Errorf("template is required for template operation")
	}

	dataMap, err := t.toMap(data)
	if err != nil {
		return nil, fmt.Errorf("cannot convert data to map: %w", err)
	}

	// Reemplazar variables en template
	result := config.Template
	for key, value := range dataMap {
		placeholder := fmt.Sprintf("{{%s}}", key)
		valueStr := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, valueStr)
	}

	return result, nil
}

// executeExtractOperation ejecuta operación de extracción
func (t *TransformActionExecutor) executeExtractOperation(data interface{}, config *TransformConfig) (interface{}, error) {
	dataMap, err := t.toMap(data)
	if err != nil {
		return nil, fmt.Errorf("cannot convert data to map: %w", err)
	}

	result := make(map[string]interface{})

	// Aplicar reglas de extracción
	for _, rule := range config.Rules {
		value, exists := dataMap[rule.Field]
		if !exists {
			continue
		}

		extractedValue := t.extractValue(value, rule.Pattern)
		result[rule.Field] = extractedValue
	}

	return result, nil
}

// executeFormatOperation ejecuta operación de formateo
func (t *TransformActionExecutor) executeFormatOperation(data interface{}, config *TransformConfig) (interface{}, error) {
	dataMap, err := t.toMap(data)
	if err != nil {
		return nil, fmt.Errorf("cannot convert data to map: %w", err)
	}

	result := make(map[string]interface{})

	// Aplicar reglas de formato
	for _, rule := range config.Rules {
		value, exists := dataMap[rule.Field]
		if !exists {
			continue
		}

		formattedValue := t.formatValue(value, rule.Format)
		result[rule.Field] = formattedValue
	}

	return result, nil
}

// Métodos auxiliares

func (t *TransformActionExecutor) toMap(data interface{}) (map[string]interface{}, error) {
	switch v := data.(type) {
	case map[string]interface{}:
		return v, nil
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			keyStr := fmt.Sprintf("%v", key)
			result[keyStr] = value
		}
		return result, nil
	default:
		// Intentar convertir via JSON
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}

		var result map[string]interface{}
		err = json.Unmarshal(bytes, &result)
		return result, err
	}
}

func (t *TransformActionExecutor) replaceValue(value, fromValue, toValue interface{}) interface{} {
	if reflect.DeepEqual(value, fromValue) {
		return toValue
	}
	return value
}

func (t *TransformActionExecutor) formatValue(value interface{}, format string) interface{} {
	if format == "" {
		return value
	}

	switch format {
	case "string":
		return fmt.Sprintf("%v", value)
	case "upper":
		return strings.ToUpper(fmt.Sprintf("%v", value))
	case "lower":
		return strings.ToLower(fmt.Sprintf("%v", value))
	case "trim":
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	default:
		// Intentar formateo personalizado
		return fmt.Sprintf(format, value)
	}
}

func (t *TransformActionExecutor) extractValue(value interface{}, pattern string) interface{} {
	if pattern == "" {
		return value
	}

	valueStr := fmt.Sprintf("%v", value)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return value
	}

	matches := re.FindStringSubmatch(valueStr)
	if len(matches) > 1 {
		return matches[1] // Primer grupo capturado
	} else if len(matches) > 0 {
		return matches[0] // Match completo
	}

	return ""
}

func (t *TransformActionExecutor) calculateValue(value interface{}, operation string) interface{} {
	switch operation {
	case "length":
		valueStr := fmt.Sprintf("%v", value)
		return len(valueStr)
	case "abs":
		if num, err := strconv.ParseFloat(fmt.Sprintf("%v", value), 64); err == nil {
			if num < 0 {
				return -num
			}
			return num
		}
	}
	return value
}

func (t *TransformActionExecutor) matchesFilter(value, filterValue interface{}) bool {
	return reflect.DeepEqual(value, filterValue)
}
