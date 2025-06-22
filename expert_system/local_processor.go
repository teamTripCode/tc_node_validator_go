package expert_system

import (
	"context"
	"encoding/json"
	"errors" // Added for errors.As
	"fmt"
	"time"

	"tripcodechain_go/utils" // Asumiendo que utils está accesible
)

const DefaultEvaluationTimeout = 5 * time.Second

// LocalExpertSystemProcessor implementa la interfaz LocalProcessor.
// Procesa consultas localmente utilizando el RuleEngine.
type LocalExpertSystemProcessor struct {
	engine RuleEngineInterface
	// evaluationTimeout time.Duration // Could be made configurable
}

// NewLocalExpertSystemProcessor crea un nuevo procesador local.
// Se asume que el RuleEngine es construido y configurado externamente.
// Para este ejemplo, si se pasa un nil engine, podría ser un problema.
// Si LocalExpertSystemProcessor fuera responsable de crear el RuleEngine,
// aquí se instanciaría el RuleEngine con un DefaultFallbackProcessor.
// Por ahora, la estructura asume que 'engine' ya está configurado con su fallback.
func NewLocalExpertSystemProcessor(engine RuleEngineInterface) *LocalExpertSystemProcessor {
	if engine == nil {
		// This case should ideally be handled by the caller or by constructing a default engine here.
		// For example, one might do:
		// engine = NewRuleEngine(NewDefaultFallbackProcessor())
		// engine.LoadRules("path/to/rules.json") // Or handle error
		utils.LogError("LocalExpertSystemProcessor: RuleEngineInterface is nil during initialization. Fallbacks depend on engine's configuration.")
	}
	return &LocalExpertSystemProcessor{
		engine: engine,
	}
}

// ProcessQuery deserializa el payload de la consulta, lo evalúa con el motor de reglas
// y serializa el resultado. Utiliza un contexto para manejar timeouts.
func (p *LocalExpertSystemProcessor) ProcessQuery(ctx context.Context, queryPayload []byte) ([]byte, error) {
	if p.engine == nil {
		utils.LogError("LocalExpertSystemProcessor: RuleEngine is not initialized.")
		err := NewError(ErrorTypeInternal, "RuleEngine not initialized")
		responseBytes, _ := json.Marshal(err) // Marshal the ExpertSystemError
		return responseBytes, err              // Return both JSON and Go error
	}

	utils.LogDebug("LocalExpertSystemProcessor: Received query payload: %s", string(queryPayload))

	var queryInput QueryInput
	if err := json.Unmarshal(queryPayload, &queryInput); err != nil {
		utils.LogError("LocalExpertSystemProcessor: Error unmarshalling query payload: %v", err)
		esErr := NewError(ErrorTypeValidation, "Invalid query payload").Wrap(err)
		responseBytes, _ := json.Marshal(esErr)
		return responseBytes, nil // Return JSON error, not a Go error to the direct P2P caller
	}

	if err := queryInput.Validate(); err != nil {
		utils.LogError("LocalExpertSystemProcessor: QueryInput validation failed: %v", err)
		esErr := NewError(ErrorTypeValidation, "Query input validation failed").Wrap(err)
		responseBytes, _ := json.Marshal(esErr)
		return responseBytes, nil // Return JSON error
	}

	evalCtx, cancel := context.WithTimeout(ctx, DefaultEvaluationTimeout)
	defer cancel()

	actionResult, err := p.engine.Evaluate(evalCtx, queryInput)
	if err != nil {
		utils.LogError("LocalExpertSystemProcessor: Error evaluating input with rule engine: %v", err)

		var esErr *ExpertSystemError
		if errors.As(err, &esErr) {
			// Error is already an ExpertSystemError, just pass it through
		} else if err == context.DeadlineExceeded {
			esErr = NewError(ErrorTypeTimeout, "Query evaluation timed out").Wrap(err)
			utils.LogWarn("LocalExpertSystemProcessor: Evaluation timed out for query.")
		} else if err == context.Canceled {
			// If the cancellation is from our timeout, it's DeadlineExceeded.
			// If it's from the parent context, it's Canceled.
			esErr = NewError(ErrorTypeInternal, "Query evaluation cancelled by client").Wrap(err)
			utils.LogInfo("LocalExpertSystemProcessor: Evaluation cancelled for query by client.")
		} else {
			// Generic rule engine error
			esErr = NewError(ErrorTypeRuleEvaluation, "Failed to process query with rule engine").Wrap(err)
		}

		responseBytes, _ := json.Marshal(esErr)
		return responseBytes, nil // No Go error to P2P caller, JSON response indicates failure
	}

	utils.LogInfo("LocalExpertSystemProcessor: Rule engine returned action: %s for rule %s", actionResult.ActionType, actionResult.RuleID)

	responseBytes, err := json.Marshal(actionResult)
	if err != nil {
		utils.LogError("LocalExpertSystemProcessor: Error marshalling action result: %v", err)
		esErr := NewError(ErrorTypeInternal, "Failed to marshal action result").Wrap(err)
		// For this internal marshal error, we might want to return a Go error too.
		// However, to keep consistency for now, send JSON error response.
		errorResponseBytes, _ := json.Marshal(esErr)
		return errorResponseBytes, nil
	}

	utils.LogDebug("LocalExpertSystemProcessor: Sending response: %s", string(responseBytes))
	return responseBytes, nil
}

// Ensure LocalExpertSystemProcessor implements LocalProcessor interface
var _ LocalProcessor = (*LocalExpertSystemProcessor)(nil)
