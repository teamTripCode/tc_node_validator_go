package expert_system

import (
	"fmt"
	// "tripcodechain_go/utils" // If logging is needed
)

// FallbackProcessor defines the interface for handling cases where primary rule evaluation fails
// or no rules are matched.
type FallbackProcessor interface {
	Process(input QueryInput) (ActionResult, error)
}

// DefaultFallbackProcessor provides a default hardcoded response.
type DefaultFallbackProcessor struct {
	DefaultAction ActionResult
}

// NewDefaultFallbackProcessor creates a new DefaultFallbackProcessor.
// It initializes a generic fallback action.
func NewDefaultFallbackProcessor() *DefaultFallbackProcessor {
	return &DefaultFallbackProcessor{
		DefaultAction: ActionResult{
			ActionType:      "generate_fallback_response",
			ResponsePayload: "We are unable to process your request with the current expert system rules. Please try again later or contact support.",
			RuleID:          "FALLBACK_DEFAULT_001",
			// Parameters could be added if needed
		},
	}
}

// Process implements the FallbackProcessor interface.
// It returns the predefined default action.
func (dfp *DefaultFallbackProcessor) Process(input QueryInput) (ActionResult, error) {
	// utils.LogInfo("DefaultFallbackProcessor: Providing default fallback response for input: %+v", input)
	// The input might be used in more sophisticated fallbacks to customize the response slightly.
	// For now, it returns a static response.
	if dfp == nil { // Should not happen if constructed with NewDefaultFallbackProcessor
		return ActionResult{}, fmt.Errorf("DefaultFallbackProcessor is not initialized")
	}
	return dfp.DefaultAction, nil
}

// Ensure DefaultFallbackProcessor implements FallbackProcessor
var _ FallbackProcessor = (*DefaultFallbackProcessor)(nil)

// NilFallbackProcessor is a fallback processor that does nothing or indicates no fallback is available.
type NilFallbackProcessor struct{}

// Process for NilFallbackProcessor returns an error indicating no fallback action was taken.
func (nfp *NilFallbackProcessor) Process(input QueryInput) (ActionResult, error) {
	return ActionResult{}, NewError(ErrorTypeNoRuleMatched, "no rule matched and no fallback action configured")
}
var _ FallbackProcessor = (*NilFallbackProcessor)(nil)
