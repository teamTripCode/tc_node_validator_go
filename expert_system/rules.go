package expert_system

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp" // Added for matches_regex
	"strings"

	"github.com/PaesslerAG/gval" // Para evaluación de expresiones
)

// Condition representa una condición dentro de una regla.
type Condition struct {
	Fact     string      `json:"fact"`     // Clave en el QueryInput.FactMap o campo de QueryInput (ej: "InputText")
	Operator string      `json:"operator"` // Ej: "equals", "contains", "greater_than", "less_than", "matches_regex", "evaluate_gval"
	Value    interface{} `json:"value"`    // Valor a comparar o expresión gval
}

// Action define la acción a tomar si una regla se cumple.
type Action struct {
	Type            string                 `json:"type"`            // Ej: "generate_response", "delegate_task"
	ResponsePayload interface{}            `json:"responsePayload"` // Payload directo para "generate_response"
	Parameters      map[string]interface{} `json:"parameters,omitempty"`
}

// Rule define una regla experta.
type Rule struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Priority    int         `json:"priority"` // Mayor número = Mayor prioridad
	Conditions  []Condition `json:"conditions"`
	Action      Action      `json:"action"`
}

// evaluateCondición verifica si una condición individual se cumple dado el input.
func (c *Condition) evaluate(input QueryInput) (bool, error) {
	var factValue interface{}
	found := false

	// Priorizar FactMap si existe la clave
	if input.FactMap != nil {
		if val, ok := input.FactMap[c.Fact]; ok {
			factValue = val
			found = true
		}
	}

	// Si no se encontró en FactMap, intentar con campos directos de QueryInput (ej: InputText)
	if !found {
		if strings.EqualFold(c.Fact, "InputText") { // Comparación insensible a mayúsculas/minúsculas para "InputText"
			factValue = input.InputText
			found = true
		}
		// Añadir más campos directos si es necesario
	}

	if !found {
		return false, fmt.Errorf("fact '%s' not found in input", c.Fact)
	}

	// Convertir factValue y c.Value a tipos comparables si es necesario
	// Por simplicidad, muchas comparaciones se harán con strings o numéricos.
	// Se puede añadir conversión más robusta aquí.

	sFactValue, factIsString := factValue.(string)
	sValue, valueIsString := c.Value.(string)

	switch c.Operator {
	case "equals":
		if factIsString && valueIsString {
			return strings.EqualFold(sFactValue, sValue), nil // Insensible a mayúsculas/minúsculas para strings
		}
		return factValue == c.Value, nil
	case "not_equals":
		if factIsString && valueIsString {
			return !strings.EqualFold(sFactValue, sValue), nil
		}
		return factValue != c.Value, nil
	case "contains":
		if !factIsString || !valueIsString {
			return false, fmt.Errorf("operator 'contains' requires string types for fact and value")
		}
		return strings.Contains(strings.ToLower(sFactValue), strings.ToLower(sValue)), nil // Insensible
	case "starts_with":
		if !factIsString || !valueIsString {
			return false, fmt.Errorf("operator 'starts_with' requires string types for fact and value")
		}
		return strings.HasPrefix(strings.ToLower(sFactValue), strings.ToLower(sValue)), nil // Insensible
	case "ends_with":
		if !factIsString || !valueIsString {
			return false, fmt.Errorf("operator 'ends_with' requires string types for fact and value")
		}
		return strings.HasSuffix(strings.ToLower(sFactValue), strings.ToLower(sValue)), nil // Insensible
	case "greater_than", "gt":
		fVal, okF := factValue.(float64) // JSON numbers son float64
		cVal, okC := c.Value.(float64)
		if !okF || !okC {
			return false, fmt.Errorf("operator '%s' requires numeric types", c.Operator)
		}
		return fVal > cVal, nil
	case "less_than", "lt":
		fVal, okF := factValue.(float64)
		cVal, okC := c.Value.(float64)
		if !okF || !okC {
			return false, fmt.Errorf("operator '%s' requires numeric types", c.Operator)
		}
		return fVal < cVal, nil
	case "matches_regex":
		if !factIsString || !valueIsString {
			return false, fmt.Errorf("operator 'matches_regex' requires string types for fact and value")
		}
		// Considerar cachear expresiones regulares compiladas si el rendimiento es crítico y hay muchas reglas con las mismas regex.
		// Por ahora, compilamos en cada evaluación.
		matched, err := regexp.MatchString(sValue, sFactValue)
		if err != nil {
			return false, fmt.Errorf("error matching regex for pattern '%s': %v", sValue, err)
		}
		return matched, nil
	case "evaluate_gval":
		// Para gval, el "fact" es el nombre de la variable en la expresión gval
		// y el "value" es la expresión gval.
		// El contexto para gval será el QueryInput.FactMap o un mapa construido
		expression, ok := c.Value.(string)
		if !ok {
			return false, fmt.Errorf("'evaluate_gval' operator requires a string expression in 'value'")
		}
		gvalContext := make(map[string]interface{})
		if input.FactMap != nil {
			for k, v := range input.FactMap {
				gvalContext[k] = v
			}
		}
		// Permitir acceso a InputText también, si es relevante para la expresión
		// gvalContext["InputText"] = input.InputText // Descomentar si se necesita

		res, err := gval.Evaluate(expression, gvalContext)
		if err != nil {
			return false, fmt.Errorf("error evaluating gval expression '%s': %v", expression, err)
		}
		boolRes, ok := res.(bool)
		if !ok {
			return false, fmt.Errorf("gval expression '%s' did not return a boolean", expression)
		}
		return boolRes, nil

	// TODO: Añadir más operadores como "matches_regex", "in_list", etc.
	default:
		return false, fmt.Errorf("unknown operator: %s", c.Operator)
	}
}

// LoadRules carga las reglas desde un archivo JSON.
func LoadRules(filePath string) ([]Rule, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file %s: %w", filePath, err)
	}

	var rules []Rule
	err = json.Unmarshal(data, &rules)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal rules from %s: %w", filePath, err)
	}
	return rules, nil
}
