package expert_system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp" // Added for matches_regex
	"strings"
	"time"

	"github.com/PaesslerAG/gval" // Para evaluación de expresiones
)

const GvalEvaluationTimeout = 1 * time.Second

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

// RuleEngineAccessProvider provides access to RuleEngine functionalities needed by Condition.evaluate.
// This is used to avoid a direct cyclic dependency if Condition needed full RuleEngine,
// and to clearly define the contract. For now, it's simple enough.
type RuleEngineAccessProvider interface {
	getCompiledRegex(pattern string) (*regexp.Regexp, error)
}

// evaluate verifica si una condición individual se cumple dado el input, el contexto y el acceso al motor de reglas.
func (c *Condition) evaluate(ctx context.Context, input QueryInput, re RuleEngineAccessProvider) (bool, error) {
	// Check for cancellation from the parent context first.
	select {
	case <-ctx.Done():
		return false, NewError(ErrorTypeTimeout, "Context cancelled before condition evaluation").Wrap(ctx.Err())
	default:
	}

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

	if !found && c.Operator != "evaluate_gval" {
		return false, NewErrorf(ErrorTypeRuleEvaluation, "fact '%s' not found in input", c.Fact)
	}

	// Convertir factValue y c.Value a tipos comparables si es necesario
	// Por simplicidad, muchas comparaciones se harán con strings o numéricos.
	// Se puede añadir conversión más robusta aquí.

	sFactValue, factIsString := factValue.(string)
	sValue, valueIsString := c.Value.(string)

	switch c.Operator {
	case "equals":
		if factIsString && valueIsString {
			return strings.EqualFold(sFactValue, sValue), nil
		}
		return factValue == c.Value, nil
	case "not_equals":
		if factIsString && valueIsString {
			return !strings.EqualFold(sFactValue, sValue), nil
		}
		return factValue != c.Value, nil
	case "contains":
		if !factIsString || !valueIsString {
			return false, NewErrorf(ErrorTypeRuleEvaluation, "operator 'contains' requires string types for fact ('%s') and value", c.Fact)
		}
		return strings.Contains(strings.ToLower(sFactValue), strings.ToLower(sValue)), nil
	case "starts_with":
		if !factIsString || !valueIsString {
			return false, NewErrorf(ErrorTypeRuleEvaluation, "operator 'starts_with' requires string types for fact ('%s') and value", c.Fact)
		}
		return strings.HasPrefix(strings.ToLower(sFactValue), strings.ToLower(sValue)), nil
	case "ends_with":
		if !factIsString || !valueIsString {
			return false, NewErrorf(ErrorTypeRuleEvaluation, "operator 'ends_with' requires string types for fact ('%s') and value", c.Fact)
		}
		return strings.HasSuffix(strings.ToLower(sFactValue), strings.ToLower(sValue)), nil
	case "greater_than", "gt":
		fVal, okF := factValue.(float64)
		cVal, okC := c.Value.(float64)
		if !okF || !okC {
			return false, NewErrorf(ErrorTypeRuleEvaluation, "operator '%s' requires numeric types for fact ('%s')", c.Operator, c.Fact)
		}
		return fVal > cVal, nil
	case "less_than", "lt":
		fVal, okF := factValue.(float64)
		cVal, okC := c.Value.(float64)
		if !okF || !okC {
			return false, NewErrorf(ErrorTypeRuleEvaluation, "operator '%s' requires numeric types for fact ('%s')", c.Operator, c.Fact)
		}
		return fVal < cVal, nil
	case "matches_regex":
		if !factIsString || !valueIsString {
			return false, NewErrorf(ErrorTypeRuleEvaluation, "operator 'matches_regex' requires string types for fact ('%s') and value", c.Fact)
		}
		compiledRegex, err := re.getCompiledRegex(sValue)
		if err != nil {
			return false, NewErrorf(ErrorTypeRuleEvaluation, "error obtaining compiled regex for pattern '%s' for fact '%s'", sValue, c.Fact).Wrap(err)
		}
		return compiledRegex.MatchString(sFactValue), nil
	case "evaluate_gval":
		expression, ok := c.Value.(string)
		if !ok {
			return false, NewError(ErrorTypeRuleEvaluation, "'evaluate_gval' operator requires a string expression in 'value'").WithRuleID(c.Fact) // Assuming c.Fact might be relevant context for rule ID here
		}

		gvalLang := gval.NewLanguage(gval.Arithmetic(), gval.Bitmask(), gval.Text(), gval.PropositionalLogic(), gval.JSON())

		evalMapContext := make(map[string]interface{})
		if input.FactMap != nil {
			for k, v_map := range input.FactMap {
				evalMapContext[k] = v_map
			}
		}
		if input.InputText != "" {
			evalMapContext["InputText"] = input.InputText
		}

		gvalEvalCtx, gvalCancel := context.WithTimeout(ctx, GvalEvaluationTimeout)
		defer gvalCancel()

		var res interface{}
		var errEval error

		evalCh := make(chan struct{})
		go func() {
			defer close(evalCh)
			res, errEval = gvalLang.Evaluate(expression, evalMapContext)
		}()

		select {
		case <-gvalEvalCtx.Done():
			err := gvalEvalCtx.Err()
			if err == context.DeadlineExceeded {
				return false, NewErrorf(ErrorTypeTimeout, "gval expression '%s' timed out after %s", expression, GvalEvaluationTimeout).Wrap(err)
			}
			return false, NewErrorf(ErrorTypeTimeout, "gval evaluation cancelled for expression '%s'", expression).Wrap(err)
		case <-evalCh:
			// Evaluation finished or errored
		}

		if errEval != nil {
			return false, NewErrorf(ErrorTypeGvalExecution, "error evaluating gval expression '%s'", expression).Wrap(errEval)
		}
		boolRes, ok := res.(bool)
		if !ok {
			return false, NewErrorf(ErrorTypeGvalExecution, "gval expression '%s' did not return a boolean, got: %T (%v)", expression, res, res)
		}
		return boolRes, nil
	default:
		return false, NewErrorf(ErrorTypeRuleEvaluation, "unknown operator: %s", c.Operator)
	}
}

// LoadRules carga las reglas desde un archivo JSON.
func LoadRules(filePath string) ([]Rule, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, NewErrorf(ErrorTypeRuleLoading, "failed to read rules file %s", filePath).Wrap(err)
	}

	var rules []Rule
	err = json.Unmarshal(data, &rules)
	if err != nil {
		return nil, NewErrorf(ErrorTypeRuleLoading, "failed to unmarshal rules from %s", filePath).Wrap(err)
	}
	return rules, nil
}

// validateRuleStructure performs basic structural and syntax validation for a single rule.
// It does not validate against dynamic engine state like available operators if they were pluggable.
func validateRuleStructure(rule Rule) error {
	if strings.TrimSpace(rule.ID) == "" {
		return NewError(ErrorTypeValidation, "rule ID cannot be empty")
	}
	if len(rule.Conditions) == 0 && rule.Action.Type == "" { // Allow rule with no conditions if it has an action (e.g. default rule)
		// This might be too lenient; a rule with no conditions should typically be a very specific fallback.
		// For now, we allow it if an action is present.
		// Consider a specific check for "always true" gval conditions if len(rule.Conditions) > 0
	}
	if strings.TrimSpace(rule.Action.Type) == "" {
		return NewErrorf(ErrorTypeValidation, "rule (ID: %s) action type cannot be empty", rule.ID)
	}

	knownOperators := map[string]bool{
		"equals": true, "not_equals": true, "contains": true, "starts_with": true, "ends_with": true,
		"greater_than": true, "gt": true, "less_than": true, "lt": true,
		"matches_regex": true, "evaluate_gval": true,
		// Add any new operators here
	}

	for i, cond := range rule.Conditions {
		if strings.TrimSpace(cond.Fact) == "" && cond.Operator != "evaluate_gval" { // gval might not need a fact
			return NewErrorf(ErrorTypeValidation, "rule (ID: %s) condition %d: fact cannot be empty unless operator is evaluate_gval", rule.ID, i)
		}
		if _, ok := knownOperators[cond.Operator]; !ok {
			return NewErrorf(ErrorTypeValidation, "rule (ID: %s) condition %d: unknown operator '%s'", rule.ID, i, cond.Operator)
		}

		// Validate GVAL syntax
		if cond.Operator == "evaluate_gval" {
			exprStr, ok := cond.Value.(string)
			if !ok {
				return NewErrorf(ErrorTypeValidation, "rule (ID: %s) condition %d: 'evaluate_gval' value must be a string expression", rule.ID, i)
			}
			if _, err := gval.NewEvaluable(exprStr); err != nil {
				return NewErrorf(ErrorTypeValidation, "rule (ID: %s) condition %d: invalid gval expression syntax for '%s'", rule.ID, i, exprStr).Wrap(err)
			}
		}

		// Validate Regex syntax (actual compilation can be done by RuleEngine to populate cache)
		if cond.Operator == "matches_regex" {
			patternStr, ok := cond.Value.(string)
			if !ok {
				return NewErrorf(ErrorTypeValidation, "rule (ID: %s) condition %d: 'matches_regex' value must be a string pattern", rule.ID, i)
			}
			if _, err := regexp.Compile(patternStr); err != nil {
				return NewErrorf(ErrorTypeValidation, "rule (ID: %s) condition %d: invalid regex pattern syntax for '%s'", rule.ID, i, patternStr).Wrap(err)
			}
		}
	}
	return nil
}
