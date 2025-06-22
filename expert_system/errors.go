package expert_system

import "fmt"

// ErrorType define el tipo de error ocurrido en el sistema experto.
type ErrorType string

const (
	ErrorTypeValidation      ErrorType = "VALIDATION_ERROR"
	ErrorTypeRuleEvaluation  ErrorType = "RULE_EVALUATION_ERROR"
	ErrorTypeTimeout         ErrorType = "TIMEOUT_ERROR"
	ErrorTypeInternal        ErrorType = "INTERNAL_ERROR"
	ErrorTypeRuleLoading     ErrorType = "RULE_LOADING_ERROR"
	ErrorTypeNoRuleMatched   ErrorType = "NO_RULE_MATCHED"
	ErrorTypeGvalExecution   ErrorType = "GVAL_EXECUTION_ERROR"
	ErrorTypeResourceLimit   ErrorType = "RESOURCE_LIMIT_ERROR" // For future use (e.g. rate limits)
)

// ExpertSystemError es una estructura de error personalizada para el sistema experto.
type ExpertSystemError struct {
	Type    ErrorType `json:"errorType"`         // Tipo de error
	Message string    `json:"message"`           // Mensaje descriptivo del error
	Code    string    `json:"code,omitempty"`    // Código de error específico (opcional)
	RuleID  string    `json:"ruleId,omitempty"`  // ID de la regla asociada al error (opcional)
	Details string    `json:"details,omitempty"` // Detalles adicionales o error subyacente (opcional)
	err     error     // Error original envuelto (opcional, no serializado directamente)
}

// Error implementa la interfaz error.
func (e *ExpertSystemError) Error() string {
	if e.RuleID != "" {
		return fmt.Sprintf("[%s] (Rule: %s) %s", e.Type, e.RuleID, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap devuelve el error envuelto, para compatibilidad con errors.Is/As.
func (e *ExpertSystemError) Unwrap() error {
	return e.err
}

// NewError crea una nueva instancia de ExpertSystemError.
func NewError(errorType ErrorType, message string) *ExpertSystemError {
	return &ExpertSystemError{Type: errorType, Message: message}
}

func NewErrorf(errorType ErrorType, format string, args ...interface{}) *ExpertSystemError {
	return &ExpertSystemError{Type: errorType, Message: fmt.Sprintf(format, args...)}
}

func (e *ExpertSystemError) WithCode(code string) *ExpertSystemError {
	e.Code = code
	return e
}

func (e *ExpertSystemError) WithRuleID(ruleID string) *ExpertSystemError {
	e.RuleID = ruleID
	return e
}

func (e *ExpertSystemError) WithDetails(details string) *ExpertSystemError {
	e.Details = details
	return e
}

func (e *ExpertSystemError) Wrap(err error) *ExpertSystemError {
	e.err = err
	if e.Details == "" && err != nil { // Populate details from wrapped error if not set
		e.Details = err.Error()
	}
	return e
}
