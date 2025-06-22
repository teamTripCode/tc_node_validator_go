package expert_system

import (
	"encoding/json"
	"fmt"

	"tripcodechain_go/utils" // Asumiendo que utils está accesible
)

// LocalExpertSystemProcessor implementa la interfaz LocalProcessor.
// Procesa consultas localmente utilizando el RuleEngine.
type LocalExpertSystemProcessor struct {
	engine RuleEngineInterface
}

// NewLocalExpertSystemProcessor crea un nuevo procesador local.
func NewLocalExpertSystemProcessor(engine RuleEngineInterface) *LocalExpertSystemProcessor {
	if engine == nil {
		utils.LogError("LocalExpertSystemProcessor: RuleEngineInterface is nil during initialization.")
		// Podría retornar un error o un procesador que siempre falle.
		// Por ahora, permitimos la creación, pero ProcessQuery fallará si el engine es nil.
	}
	return &LocalExpertSystemProcessor{
		engine: engine,
	}
}

// ProcessQuery deserializa el payload de la consulta, lo evalúa con el motor de reglas
// y serializa el resultado.
func (p *LocalExpertSystemProcessor) ProcessQuery(queryPayload []byte) ([]byte, error) {
	if p.engine == nil {
		errMsg := "LocalExpertSystemProcessor: RuleEngine is not initialized."
		utils.LogError(errMsg)
		return nil, fmt.Errorf("%s", errMsg) // Corrected: Use %s for errMsg
	}

	utils.LogDebug("LocalExpertSystemProcessor: Received query payload: %s", string(queryPayload))

	var queryInput QueryInput
	err := json.Unmarshal(queryPayload, &queryInput)
	if err != nil {
		utils.LogError("LocalExpertSystemProcessor: Error unmarshalling query payload: %v", err)
		return nil, fmt.Errorf("invalid query payload: %w", err)
	}

	actionResult, err := p.engine.Evaluate(queryInput)
	if err != nil {
		utils.LogError("LocalExpertSystemProcessor: Error evaluating input with rule engine: %v", err)
		// Decidir si se devuelve un error genérico o el error específico del motor.
		// Por ahora, devolvemos un error que indica que no hubo coincidencia o fallo en evaluación.
		// Podríamos querer un JSON de error estandarizado aquí.
		errorResponse := map[string]string{"error": "failed to process query with rule engine", "details": err.Error()}
		return json.Marshal(errorResponse) // Aún así devolvemos un JSON, pero con error.
	}

	utils.LogInfo("LocalExpertSystemProcessor: Rule engine returned action: %s for rule %s", actionResult.ActionType, actionResult.RuleID)

	responseBytes, err := json.Marshal(actionResult)
	if err != nil {
		utils.LogError("LocalExpertSystemProcessor: Error marshalling action result: %v", err)
		return nil, fmt.Errorf("failed to marshal action result: %w", err)
	}

	utils.LogDebug("LocalExpertSystemProcessor: Sending response: %s", string(responseBytes))
	return responseBytes, nil
}

// Ensure LocalExpertSystemProcessor implements LocalProcessor interface
var _ LocalProcessor = (*LocalExpertSystemProcessor)(nil)
