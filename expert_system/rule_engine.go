package expert_system

import (
	"context"
	"errors"
	"fmt"
	"path/filepath" // Added
	"regexp"
	"sort"
	"sync"

	"github.com/fsnotify/fsnotify" // Added
	"tripcodechain_go/utils"
)

// RuleEngine es el motor de inferencia para el sistema de reglas expertas.
type RuleEngine struct {
	rules                 []Rule
	mu                    sync.RWMutex
	ruleIndex             map[string][]Rule
	factIndex             map[string][]Rule
	alwaysApplicableRules []Rule
	compiledRegexCache    map[string]*regexp.Regexp
	fallback              FallbackProcessor
}

// NewRuleEngine creates a new instance of RuleEngine.
// If no fallbackProcessor is provided, a NilFallbackProcessor will be used, meaning no fallback actions.
func NewRuleEngine(fallback FallbackProcessor) *RuleEngine {
	if fallback == nil {
		fallback = &NilFallbackProcessor{} // Default to no fallback action
	}
	return &RuleEngine{
		rules:                 make([]Rule, 0),
		ruleIndex:             make(map[string][]Rule),
		factIndex:             make(map[string][]Rule),
		alwaysApplicableRules: make([]Rule, 0),
		compiledRegexCache:    make(map[string]*regexp.Regexp),
		fallback:              fallback,
	}
}

// getCompiledRegex recupera una expresión regular compilada del caché o la compila y guarda.
// Este método es seguro para concurrencia para lectura, pero la escritura debe estar protegida si se llama concurrentemente.
// Dado que LoadRules (y por ende indexRules/compilación inicial de regex) se hace bajo Lock,
// y Evaluate (que usaría esto) se hace bajo RLock, necesitamos un Lock para escrituras en el caché si la compilación es lazy.
func (re *RuleEngine) getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	re.mu.RLock() // RLock para lectura inicial
	cachedRegex, found := re.compiledRegexCache[pattern]
	re.mu.RUnlock()

	if found {
		regexCacheHitsTotal.Inc()
		return cachedRegex, nil
	}

	// Compilar si no está en caché. Esto requiere un Lock completo para la escritura.
	re.mu.Lock()
	defer re.mu.Unlock()

	// Doble checkeo por si otra goroutine lo compiló mientras se esperaba el Lock
	if cachedRegex, found = re.compiledRegexCache[pattern]; found {
		// This was a concurrent compilation, another goroutine did it first. Still a "hit" from this one's perspective after lock.
		// Or, consider it a "miss" that was resolved by another's compilation.
		// For simplicity, let's count the initial check as the hit/miss. If it wasn't found initially, it's a miss.
		// The fact that another goroutine compiled it doesn't change this goroutine's initial miss.
		return cachedRegex, nil
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		utils.LogError("component:RuleEngine msg:Failed to compile regex pattern pattern:%s error:%v", pattern, err)
		// Not incrementing misses here as it's a compilation failure, not a cache miss leading to successful compilation.
		return nil, err
	}
	re.compiledRegexCache[pattern] = compiled
	regexCacheMissesTotal.Inc() // Successful compilation after a miss
	regexCacheSize.Set(float64(len(re.compiledRegexCache)))
	utils.LogDebug("component:RuleEngine msg:Compiled and cached regex pattern:%s", pattern)
	return compiled, nil
}


// indexRules crea índices para las reglas para acelerar la evaluación.
// Este método no es seguro para concurrencia y debe llamarse bajo un Lock.
func (re *RuleEngine) indexRules() {
	re.ruleIndex = make(map[string][]Rule)
	re.factIndex = make(map[string][]Rule)
	re.alwaysApplicableRules = make([]Rule, 0)

	for _, rule := range re.rules {
		if len(rule.Conditions) == 0 {
			re.alwaysApplicableRules = append(re.alwaysApplicableRules, rule)
			continue // No more indexing needed for this rule
		}

		isAlwaysApplicableFromConditions := true
		for _, cond := range rule.Conditions {
			// Index by operator type
			re.ruleIndex[cond.Operator] = append(re.ruleIndex[cond.Operator], rule)
			// Index by fact name
			re.factIndex[cond.Fact] = append(re.factIndex[cond.Fact], rule)

			// Check if condition is of the type gval "true" (or similar always true)
			// This is a simple heuristic. A more robust check might involve parsing gval.
			if !(cond.Operator == "evaluate_gval" && cond.Value == "true") {
				isAlwaysApplicableFromConditions = false
			}
		}
		if isAlwaysApplicableFromConditions {
			// If all conditions were of the "always true" type
			re.alwaysApplicableRules = append(re.alwaysApplicableRules, rule)
		}
	}

	// Remove duplicates from alwaysApplicableRules (if a rule was added directly and then via conditions)
	// This can be done by converting to a map and back, or sorting and unique pass.
	// For now, let's ensure it's sorted by priority as other rule sets.
	// Duplicates here are less of an issue as candidate map will overwrite.
	sort.Slice(re.alwaysApplicableRules, func(i, j int) bool {
		return re.alwaysApplicableRules[i].Priority > re.alwaysApplicableRules[j].Priority
	})
	// TODO: Consider removing duplicates from ruleIndex and factIndex lists if performance becomes an issue.
	// For now, the candidate map approach in Evaluate handles rule duplication.
	utils.LogInfo("component:RuleEngine msg:Rules indexed num_operators:%d num_facts:%d num_always_applicable:%d",
		len(re.ruleIndex), len(re.factIndex), len(re.alwaysApplicableRules))
}

// LoadRules carga y ordena las reglas desde el archivo especificado.
// Las reglas se ordenan por prioridad descendente (mayor prioridad primero).
func (re *RuleEngine) LoadRules(filePath string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	rules, err := LoadRules(filePath) // LoadRules itself returns a wrapped error or simple fmt.Errorf
	if err != nil {
		utils.LogError("RuleEngine: Failed to load rules from source: %v", err)
		ruleLoadsTotal.WithLabelValues("failure").Inc()
		// Wrap the error from LoadRules (which might be from os.ReadFile or json.Unmarshal)
		return NewErrorf(ErrorTypeRuleLoading, "failed to load rules from file %s", filePath).Wrap(err)
	}

	// Validate all loaded rules before applying them
	for _, rule := range rules {
		if err := validateRuleStructure(rule); err != nil {
			// It's important that LoadRules and ReloadRules clearly indicate failure
			// without modifying the engine's current active rules if validation fails.
			return NewErrorf(ErrorTypeRuleLoading, "invalid rule structure detected (ID: %s)", rule.ID).Wrap(err)
		}
		// Regex pre-compilation could also happen here by calling re.getCompiledRegex for each regex condition.
		// This would populate the cache early.
		for _, cond := range rule.Conditions {
			if cond.Operator == "matches_regex" {
				if pattern, ok := cond.Value.(string); ok {
					if _, err := re.getCompiledRegex(pattern); err != nil {
						return NewErrorf(ErrorTypeRuleLoading, "failed to compile regex for rule ID %s, pattern '%s'", rule.ID, pattern).Wrap(err)
					}
				}
			}
		}
	}

	// Ordenar reglas por prioridad (mayor primero)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	re.rules = rules
	re.indexRules() // Index rules after loading
	activeRulesCount.Set(float64(len(re.rules)))
	ruleLoadsTotal.WithLabelValues("success").Inc()
	utils.LogInfo("RuleEngine: Successfully loaded, sorted, and indexed %d rules from %s", len(re.rules), filePath)
	return nil
}

// Evaluate procesa el input contra las reglas cargadas, respetando el contexto para timeouts/cancelación.
// Devuelve la acción de la primera regla que coincida (debido a la priorización).
func (re *RuleEngine) Evaluate(ctx context.Context, input QueryInput) (ActionResult, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if len(re.rules) == 0 && len(re.alwaysApplicableRules) == 0 {
		utils.LogInfo("RuleEngine: No rules loaded to evaluate (including always applicable).")
		return ActionResult{}, NewError(ErrorTypeRuleLoading, "no rules loaded")
	}

	utils.LogInfo("RuleEngine: Evaluating input: %+v with context", input)

	candidateRulesMap := make(map[string]Rule)

	// 1. Add rules from factIndex based on input.FactMap
	if input.FactMap != nil {
		for factKey := range input.FactMap {
			if rulesFromFact, ok := re.factIndex[factKey]; ok {
				for _, rule := range rulesFromFact {
					candidateRulesMap[rule.ID] = rule
				}
			}
		}
	}

	// 2. Add rules from factIndex based on InputText (if "InputText" is used as a fact name)
	if rulesFromInputTextFact, ok := re.factIndex["InputText"]; ok {
		for _, rule := range rulesFromInputTextFact {
			candidateRulesMap[rule.ID] = rule
		}
	}

	// 3. Add all alwaysApplicableRules
	// These are rules with no conditions or gval "true" conditions.
	// They are already sorted by priority during indexing.
	for _, rule := range re.alwaysApplicableRules {
		candidateRulesMap[rule.ID] = rule
	}

	// 4. Fallback: If no candidates were found through specific fact indexing,
	// but there are rules, it might mean the relevant rules are not specific to input facts
	// or our indexing missed them. The alwaysApplicableRules should cover many general cases.
	// If candidateRulesMap is still empty and re.rules is not, it means no facts from input matched
	// any indexed fact rule, and there were no "always applicable" rules.
	// In this scenario, it's safer to consider all rules to not miss any.
	// However, `alwaysApplicableRules` should contain fallbacks like `gval "true"`.
	// If `candidateRulesMap` is empty at this stage, it implies that either:
	//    a) No rules exist at all (covered by initial check).
	//    b) Input facts did not match any rule's facts AND no always-applicable rules exist.
	//       In this specific sub-case, it's likely no rule will match.
	// The current logic of adding alwaysApplicableRules should be sufficient.
	// If `candidateRulesMap` is empty, it truly means no specific rules + no general rules are candidates.

	if len(candidateRulesMap) == 0 {
		// This means no facts in the input matched any rule conditions via factIndex,
		// and there are no "always_applicable" rules (like unconditional or gval-true fallbacks).
		// In this scenario, it is highly likely no rule will match.
		// We could iterate all re.rules as a last resort, but it might be redundant if alwaysApplicableRules are well-defined.
		// For safety, if re.rules is not empty, let's add them all. This handles edge cases where indexing might be incomplete for some rule types.
		if len(re.rules) > 0 {
			utils.LogDebug("RuleEngine: No candidates from specific fact indexing or always applicable rules. Adding all rules as fallback.")
			for _, rule := range re.rules { // re.rules is already sorted by priority
				candidateRulesMap[rule.ID] = rule
			}
		} else {
			utils.LogInfo("RuleEngine: No candidate rules found and no fallback rules loaded.")
			return ActionResult{}, NewError(ErrorTypeNoRuleMatched, "no rule matched for the input (no candidates)")
		}
	}

	candidateRules := make([]Rule, 0, len(candidateRulesMap))
	for _, rule := range candidateRulesMap {
		candidateRules = append(candidateRules, rule)
	}

	// Sort all collected candidate rules by priority
	sort.Slice(candidateRules, func(i, j int) bool {
		return candidateRules[i].Priority > candidateRules[j].Priority
	})

// Evaluate procesa el input contra las reglas cargadas, respetando el contexto para timeouts/cancelación.
// Devuelve la acción de la primera regla que coincida (debido a la priorización).
func (re *RuleEngine) Evaluate(ctx context.Context, input QueryInput) (ActionResult, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if len(re.rules) == 0 && len(re.alwaysApplicableRules) == 0 {
		utils.LogInfo("RuleEngine: No rules loaded to evaluate (including always applicable).")
		return ActionResult{}, NewError(ErrorTypeRuleLoading, "no rules loaded")
	}

	utils.LogInfo("RuleEngine: Evaluating input: %+v with context", input)

	candidateRulesMap := make(map[string]Rule)

	// 1. Add rules from factIndex based on input.FactMap
	if input.FactMap != nil {
		for factKey := range input.FactMap {
			if rulesFromFact, ok := re.factIndex[factKey]; ok {
				for _, rule := range rulesFromFact {
					candidateRulesMap[rule.ID] = rule
				}
			}
		}
	}

	// 2. Add rules from factIndex based on InputText (if "InputText" is used as a fact name)
	if rulesFromInputTextFact, ok := re.factIndex["InputText"]; ok {
		for _, rule := range rulesFromInputTextFact {
			candidateRulesMap[rule.ID] = rule
		}
	}

	// 3. Add all alwaysApplicableRules
	for _, rule := range re.alwaysApplicableRules {
		candidateRulesMap[rule.ID] = rule
	}

	if len(candidateRulesMap) == 0 {
		if len(re.rules) > 0 {
			utils.LogDebug("RuleEngine: No candidates from specific fact indexing or always applicable rules. Adding all rules as fallback.")
			for _, rule := range re.rules {
				candidateRulesMap[rule.ID] = rule
			}
		} else {
			utils.LogInfo("RuleEngine: No candidate rules found and no fallback rules loaded.")
			return ActionResult{}, NewError(ErrorTypeNoRuleMatched, "no rule matched for the input (no candidates)")
		}
	}

	candidateRules := make([]Rule, 0, len(candidateRulesMap))
	for _, rule := range candidateRulesMap {
		candidateRules = append(candidateRules, rule)
	}

	sort.Slice(candidateRules, func(i, j int) bool {
		return candidateRules[i].Priority > candidateRules[j].Priority
	})

	utils.LogDebug("RuleEngine: Total candidate rules to evaluate (after indexing and sorting): %d", len(candidateRules))

	for _, rule := range candidateRules {
		startTime := time.Now()

		// Check for context cancellation before evaluating each rule
		select {
		case <-ctx.Done():
			utils.LogInfo("RuleEngine: Context cancelled during rule evaluation loop.")
			ruleEvaluationDurationSeconds.WithLabelValues(rule.ID).Observe(time.Since(startTime).Seconds())
			// rulesEvaluatedTotal is not incremented here as evaluation didn't fully start/complete for this rule due to context.
			return ActionResult{}, NewError(ErrorTypeTimeout, "Context cancelled during rule evaluation").Wrap(ctx.Err())
		default:
		}

		match := true
		if len(rule.Conditions) == 0 {
			match = true
		} else {
			for _, cond := range rule.Conditions {
				condResult, err := cond.evaluate(ctx, input, re)
				if err != nil {
					var esErr *ExpertSystemError
					if errors.As(err, &esErr) && esErr.Type == ErrorTypeTimeout {
						utils.LogInfo("RuleEngine: Timeout during condition evaluation for rule %s: %v", rule.ID, err)
						rulesEvaluatedTotal.WithLabelValues(rule.ID).Inc()
						ruleEvaluationDurationSeconds.WithLabelValues(rule.ID).Observe(time.Since(startTime).Seconds())
						return ActionResult{}, esErr
					} else if err == context.Canceled || err == context.DeadlineExceeded {
						utils.LogInfo("RuleEngine: Context %v during condition evaluation for rule %s.", err, rule.ID)
						rulesEvaluatedTotal.WithLabelValues(rule.ID).Inc()
						ruleEvaluationDurationSeconds.WithLabelValues(rule.ID).Observe(time.Since(startTime).Seconds())
						return ActionResult{}, NewError(ErrorTypeTimeout, "Context error in condition").Wrap(err)
					}
					utils.LogError("RuleEngine: Error evaluating condition for rule %s: %v. Fact: %s, Operator: %s", rule.ID, err, cond.Fact, cond.Operator)
					match = false
					break
				}
				if !condResult {
					match = false
					break
				}
			}
		}

		rulesEvaluatedTotal.WithLabelValues(rule.ID).Inc()
		ruleEvaluationDurationSeconds.WithLabelValues(rule.ID).Observe(time.Since(startTime).Seconds())

		if match {
			rulesMatchedTotal.WithLabelValues(rule.ID).Inc()
			utils.LogInfo("RuleEngine: Rule %s matched. Action: %s", rule.ID, rule.Action.Type)
			return ActionResult{
				ActionType:      rule.Action.Type,
				ResponsePayload: rule.Action.ResponsePayload,
				Parameters:      rule.Action.Parameters,
				RuleID:          rule.ID,
			}, nil
		}
	}

	utils.LogInfo("RuleEngine: No rule matched for the given input (after evaluating %d candidates). Attempting fallback.", len(candidateRules))
	// If no rule matched, use the fallback processor.
	if re.fallback != nil {
		return re.fallback.Process(input)
	}
	// If no fallback processor, return the NoRuleMatched error
	return ActionResult{}, NewError(ErrorTypeNoRuleMatched, "no rule matched for the input and no fallback configured")
}

// ReloadRules loads or reloads rules from the given file path.
// It is concurrency-safe.
func (re *RuleEngine) ReloadRules(filePath string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	// Store current rules in case of reload failure to potentially rollback,
	// or simply log and keep old rules. For now, if load fails, old rules remain.
	// oldRules := re.rules
	// oldRuleIndex := re.ruleIndex
	// oldFactIndex := re.factIndex
	// oldAlwaysApplicableRules := re.alwaysApplicableRules
	// oldCompiledRegexCache := re.compiledRegexCache // Cache might be better to just clear specific to new rules

	rules, err := LoadRules(filePath)
	if err != nil {
		utils.LogError("RuleEngine: Failed to reload rules from source %s: %v", filePath, err)
		ruleLoadsTotal.WithLabelValues("failure").Inc() // Use same metric for reloads
		// Optionally, restore old rules here if they were backed up and this is preferred.
		// For now, keep existing rules on critical load failure.
		return NewErrorf(ErrorTypeRuleLoading, "failed to reload rules from file %s", filePath).Wrap(err)
	}

	// Validate all loaded rules before applying them
	for _, rule := range rules {
		if err := validateRuleStructure(rule); err != nil {
			// On validation error, we should definitely keep old rules.
			// The lock is held, so no concurrent access issues. Current state is preserved.
			return NewErrorf(ErrorTypeRuleLoading, "invalid rule structure detected during reload (ID: %s)", rule.ID).Wrap(err)
		}
		// Pre-compile regexes and populate cache
		for _, cond := range rule.Conditions {
			if cond.Operator == "matches_regex" {
				if pattern, ok := cond.Value.(string); ok {
					if _, errCompile := re.getCompiledRegex(pattern); errCompile != nil {
						return NewErrorf(ErrorTypeRuleLoading, "failed to compile regex during reload for rule ID %s, pattern '%s'", rule.ID, pattern).Wrap(errCompile)
					}
				}
			}
		}
	}

	// Sort new rules by priority
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	re.rules = rules
	// Re-index. This also clears old indexes.
	re.indexRules()

	// Regex cache:
	// Option 1: Clear the entire cache. Simpler, but might recompile regexes unnecessarily if they were in old and new rules.
	// re.compiledRegexCache = make(map[string]*regexp.Regexp)
	// Option 2: Smarter diff or only add new. For now, let's clear it for simplicity upon reload.
	// A better approach would be to pre-compile regexes during rule validation (Task 4.2)
	// and populate the cache then. If validation passes, this new cache is swapped.
	// For now, let `getCompiledRegex` handle lazy caching. A full clear might be too aggressive.
	// Let's keep the cache, `getCompiledRegex` will fill it as needed.
	// If a rule is removed, its regex might stay in cache but that's not harmful.

	activeRulesCount.Set(float64(len(re.rules)))
	ruleLoadsTotal.WithLabelValues("success").Inc() // Use same metric for reloads
	utils.LogInfo("RuleEngine: Successfully reloaded, sorted, and indexed %d rules from %s", len(re.rules), filePath)
	return nil
}

// StartFileWatcher starts a goroutine to monitor the rules file for changes and reload them.
// It takes the file path and a channel to signal shutdown.
func (re *RuleEngine) StartFileWatcher(filePath string, done <-chan struct{}) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	// No defer watcher.Close() here, it's closed in the goroutine on exit

	go func() {
		defer watcher.Close()
		utils.LogInfo("RuleEngine FileWatcher: Starting watcher for %s", filePath)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					utils.LogInfo("RuleEngine FileWatcher: Watcher events channel closed.")
					return
				}
				// We only care about writes to the file.
				// fsnotify can sometimes send multiple events for a single save (e.g. CHMOD, WRITE)
				// A short debounce might be useful if this causes rapid reloads, but ReloadRules is locked.
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					utils.LogInfo("RuleEngine FileWatcher: Detected change in %s: %s. Reloading rules.", event.Name, event.Op.String())
					if err := re.ReloadRules(filePath); err != nil {
						utils.LogError("RuleEngine FileWatcher: Error reloading rules: %v", err)
					} else {
						utils.LogInfo("RuleEngine FileWatcher: Rules reloaded successfully from %s", filePath)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					utils.LogInfo("RuleEngine FileWatcher: Watcher errors channel closed.")
					return
				}
				utils.LogError("RuleEngine FileWatcher: Watcher error: %v", err)
			case <-done: // Signal to terminate the watcher
				utils.LogInfo("RuleEngine FileWatcher: Shutting down watcher for %s", filePath)
				return
			}
		}
	}()

	err = watcher.Add(filePath)
	if err != nil {
		// Close watcher if Add fails, otherwise the goroutine will handle it.
		// However, if Add fails, the goroutine might not have started or might exit quickly.
		// It's safer to attempt a close here and let the defer in goroutine handle subsequent closes if any.
		watcher.Close()
		return fmt.Errorf("failed to add file %s to watcher: %w", filePath, err)
	}
	// Also watch the directory to detect file replacements (e.g. k8s configmap update)
	dirPath := filepath.Dir(filePath)
	err = watcher.Add(dirPath)
	if err != nil {
		// Log this error but don't fail startup completely if watching only the file worked.
		// Some systems might have issues watching directories.
		utils.LogWarn("RuleEngine FileWatcher: Failed to add directory %s to watcher: %v. Will only watch file directly.", dirPath, err)
	}


	utils.LogInfo("RuleEngine FileWatcher: Watcher started for file %s and directory %s", filePath, dirPath)
	return nil
}


// Ensure RuleEngine implements RuleEngineInterface
var _ RuleEngineInterface = (*RuleEngine)(nil)
var _ RuleEngineAccessProvider = (*RuleEngine)(nil) // Ensure it implements the provider interface
