package expert_system

import (
	"context"
	"fmt"
	"unicode/utf8"
)

const (
	MaxInputTextLength = 10 * 1024 // 10 KB
	MaxFactKeyLength   = 256
	MaxFactValueStringLength = 4 * 1024 // 4 KB
	MaxFactMapDepth = 5
	MaxFactMapTotalKeys = 100
)

// QueryInput representa la estructura esperada del payload de la consulta después de deserializar.
type QueryInput struct {
	InputText string                 `json:"inputText"`
	FactMap   map[string]interface{} `json:"factMap,omitempty"` // Para hechos más estructurados
}

// Validate realiza una validación estricta de los campos de QueryInput.
func (qi *QueryInput) Validate() error {
	// Validar longitud de InputText
	if utf8.RuneCountInString(qi.InputText) > MaxInputTextLength {
		return fmt.Errorf("InputText exceeds maximum length of %d characters", MaxInputTextLength)
	}

	// Validar FactMap
	if qi.FactMap != nil {
		if err := validateFactMap(qi.FactMap, 0, 0); err != nil {
			return fmt.Errorf("FactMap validation failed: %w", err)
		}
	}

	return nil
}

// validateFactMap valida recursivamente el FactMap.
// currentDepth para controlar la profundidad de anidamiento.
// numKeys para controlar el número total de claves.
func validateFactMap(factMap map[string]interface{}, currentDepth int, numKeys int) error {
	if currentDepth > MaxFactMapDepth {
		return fmt.Errorf("exceeds maximum map depth of %d", MaxFactMapDepth)
	}
	if numKeys + len(factMap) > MaxFactMapTotalKeys && currentDepth == 0 { // Check total keys only at top level or pass count down
		return fmt.Errorf("exceeds maximum total number of keys in FactMap (%d)", MaxFactMapTotalKeys)
	}


	for key, value := range factMap {
		if utf8.RuneCountInString(key) > MaxFactKeyLength {
			return fmt.Errorf("fact key '%s' exceeds maximum length of %d characters", key, MaxFactKeyLength)
		}

		switch v := value.(type) {
		case string:
			if utf8.RuneCountInString(v) > MaxFactValueStringLength {
				return fmt.Errorf("value for fact key '%s' exceeds maximum string length of %d characters", key, MaxFactValueStringLength)
			}
		case float64, bool:
			// Estos tipos son aceptables y no requieren validación de longitud adicional aquí.
			// JSON numbers son float64.
		case map[string]interface{}:
			err := validateFactMap(v, currentDepth+1, numKeys + len(factMap))
			if err != nil {
				return fmt.Errorf("nested map for key '%s' validation failed: %w", key, err)
			}
		case []interface{}:
			// Podríamos querer validar también la longitud y profundidad de los slices.
			// Por ahora, permitimos slices de tipos simples o mapas anidados (que serán validados).
			if err := validateSlice(v, currentDepth+1, numKeys + len(factMap)); err != nil {
				return fmt.Errorf("slice for key '%s' validation failed: %w", key, err)
			}
		case nil:
			// nil es un valor JSON válido (null), lo permitimos.
		default:
			return fmt.Errorf("fact key '%s' has unsupported type: %T", key, value)
		}
	}
	return nil
}

func validateSlice(slice []interface{}, currentDepth int, numKeys int) error {
	if currentDepth > MaxFactMapDepth { // Re-utilizamos MaxFactMapDepth para la profundidad de arrays/slices
		return fmt.Errorf("slice exceeds maximum depth of %d", MaxFactMapDepth)
	}
	// Podríamos añadir un MaxSliceLength si fuera necesario.

	for i, item := range slice {
		switch v := item.(type) {
		case string:
			if utf8.RuneCountInString(v) > MaxFactValueStringLength {
				return fmt.Errorf("string item in slice at index %d exceeds maximum string length of %d characters", i, MaxFactValueStringLength)
			}
		case float64, bool:
			// OK
		case map[string]interface{}:
			err := validateFactMap(v, currentDepth+1, numKeys) // numKeys no se incrementa aquí, se cuenta por mapa
			if err != nil {
				return fmt.Errorf("map item in slice at index %d validation failed: %w", i, err)
			}
		case []interface{}:
			err := validateSlice(v, currentDepth+1, numKeys)
			if err != nil {
				return fmt.Errorf("nested slice at index %d validation failed: %w", i, err)
			}
		case nil:
			// OK
		default:
			return fmt.Errorf("item in slice at index %d has unsupported type: %T", i, item)
		}
	}
	return nil
}


// ActionResult representa el resultado de la evaluación de una regla.
type ActionResult struct {
	ActionType      string                 `json:"actionType"`      // Ej: "generate_response", "delegate_task"
	ResponsePayload interface{}            `json:"responsePayload"` // Puede ser string, JSON, etc.
	Parameters      map[string]interface{} `json:"parameters,omitempty"`
	RuleID          string                 `json:"ruleId"` // ID de la regla que se disparó
}

// RuleEngineInterface define el contrato para el motor de reglas.
type RuleEngineInterface interface {
	Evaluate(ctx context.Context, input QueryInput) (ActionResult, error)
	LoadRules(filePath string) error
	// getCompiledRegex is internal to RuleEngine, not part of the main interface for callers usually
	// However, Condition.evaluate needs it. RuleEngineAccessProvider handles this.
}

// LocalProcessor define la interfaz para el procesador local que usa el sistema experto.
// Esta interfaz reemplazará a p2p.LocalLLMProcessor.
// Asumimos que el framework que llama a ProcessQuery puede proveer un context.Context.
type LocalProcessor interface {
	ProcessQuery(ctx context.Context, queryPayload []byte) ([]byte, error)
}
