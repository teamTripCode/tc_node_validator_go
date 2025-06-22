package expert_system

// QueryInput representa la estructura esperada del payload de la consulta después de deserializar.
// Actualmente, se asume un campo "inputText" simple, pero puede extenderse.
type QueryInput struct {
	InputText string                 `json:"inputText"`
	FactMap   map[string]interface{} `json:"factMap,omitempty"` // Para hechos más estructurados
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
	Evaluate(input QueryInput) (ActionResult, error)
	LoadRules(filePath string) error
}

// LocalProcessor define la interfaz para el procesador local que usa el sistema experto.
// Esta interfaz reemplazará a p2p.LocalLLMProcessor.
type LocalProcessor interface {
	ProcessQuery(queryPayload []byte) ([]byte, error)
}
