package expert_system

import (
	"fmt"
	"sort" // Para ordenar reglas por prioridad
	"sync"

	"tripcodechain_go/utils" // Asumiendo que utils está accesible
)

// RuleEngine es el motor de inferencia para el sistema de reglas expertas.
type RuleEngine struct {
	rules []Rule
	mu    sync.RWMutex
}

// NewRuleEngine crea una nueva instancia de RuleEngine.
func NewRuleEngine() *RuleEngine {
	return &RuleEngine{
		rules: make([]Rule, 0),
	}
}

// LoadRules carga y ordena las reglas desde el archivo especificado.
// Las reglas se ordenan por prioridad descendente (mayor prioridad primero).
func (re *RuleEngine) LoadRules(filePath string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	rules, err := LoadRules(filePath) // Usa la función de rules.go
	if err != nil {
		utils.LogError("RuleEngine: Failed to load rules: %v", err)
		return err
	}

	// Ordenar reglas por prioridad (mayor primero)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	re.rules = rules
	utils.LogInfo("RuleEngine: Successfully loaded and sorted %d rules from %s", len(re.rules), filePath)
	return nil
}

// Evaluate procesa el input contra las reglas cargadas.
// Devuelve la acción de la primera regla que coincida (debido a la priorización).
func (re *RuleEngine) Evaluate(input QueryInput) (ActionResult, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if len(re.rules) == 0 {
		utils.LogInfo("RuleEngine: No rules loaded to evaluate.") // Changed LogWarn to LogInfo
		return ActionResult{}, fmt.Errorf("no rules loaded")
	}

	utils.LogInfo("RuleEngine: Evaluating input: %+v", input)

	for _, rule := range re.rules {
		match := true
		utils.LogDebug("RuleEngine: Evaluating rule ID: %s, Priority: %d", rule.ID, rule.Priority)
		for _, cond := range rule.Conditions {
			condResult, err := cond.evaluate(input)
			if err != nil {
				utils.LogError("RuleEngine: Error evaluating condition for rule %s: %v. Fact: %s, Operator: %s", rule.ID, err, cond.Fact, cond.Operator)
				match = false
				break // Error en una condición, la regla no puede coincidir
			}
			if !condResult {
				match = false
				break // Una condición no se cumple, pasar a la siguiente regla
			}
		}

		if match {
			utils.LogInfo("RuleEngine: Rule %s matched. Action: %s", rule.ID, rule.Action.Type)
			return ActionResult{
				ActionType:      rule.Action.Type,
				ResponsePayload: rule.Action.ResponsePayload,
				Parameters:      rule.Action.Parameters,
				RuleID:          rule.ID,
			}, nil
		}
	}

	utils.LogInfo("RuleEngine: No rule matched for the given input.")
	return ActionResult{}, fmt.Errorf("no rule matched for the input")
}

// Ensure RuleEngine implements RuleEngineInterface
var _ RuleEngineInterface = (*RuleEngine)(nil)
