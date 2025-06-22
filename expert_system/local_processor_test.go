package expert_system

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// MockRuleEngine para simular el comportamiento del motor de reglas.
type MockRuleEngine struct {
	EvaluateFunc func(input QueryInput) (ActionResult, error)
	LoadRulesFunc func(filePath string) error
}

func (m *MockRuleEngine) Evaluate(input QueryInput) (ActionResult, error) {
	if m.EvaluateFunc != nil {
		return m.EvaluateFunc(input)
	}
	return ActionResult{}, fmt.Errorf("EvaluateFunc not set")
}

func (m *MockRuleEngine) LoadRules(filePath string) error {
	if m.LoadRulesFunc != nil {
		return m.LoadRulesFunc(filePath)
	}
	return nil // Por defecto, no hacer nada y no devolver error
}

var _ RuleEngineInterface = (*MockRuleEngine)(nil)


func TestLocalExpertSystemProcessor_ProcessQuery(t *testing.T) {
	tests := []struct {
		name             string
		queryPayload     []byte
		mockEvaluateFunc func(input QueryInput) (ActionResult, error)
		wantResponse     []byte
		wantErr          bool
	}{
		{
			name:         "Valid query - success",
			queryPayload: []byte(`{"inputText": "hello"}`),
			mockEvaluateFunc: func(input QueryInput) (ActionResult, error) {
				if input.InputText == "hello" {
					return ActionResult{
						ActionType: "greet",
						ResponsePayload: "world",
						RuleID:     "RULE_HELLO",
					}, nil
				}
				return ActionResult{}, fmt.Errorf("no match")
			},
			wantResponse: []byte(`{"actionType":"greet","responsePayload":"world","ruleId":"RULE_HELLO"}`),
			wantErr:      false,
		},
		{
			name:         "Engine returns error",
			queryPayload: []byte(`{"inputText": "test error"}`),
			mockEvaluateFunc: func(input QueryInput) (ActionResult, error) {
				return ActionResult{}, fmt.Errorf("engine evaluation error")
			},
			// Esperamos una respuesta JSON de error estandarizada del procesador
			wantResponse: []byte(`{"error":"failed to process query with rule engine","details":"engine evaluation error"}`),
			wantErr:      false, // El procesador maneja el error y devuelve una respuesta JSON de error, no un error Go al llamador de ProcessQuery
		},
		{
			name:         "Invalid query payload - unmarshal error",
			queryPayload: []byte(`{not_json`),
			mockEvaluateFunc: func(input QueryInput) (ActionResult, error) {
				// No debería llamarse
				return ActionResult{}, nil
			},
			wantResponse: nil, // O un JSON de error específico si se implementa así
			wantErr:      true,
		},
		{
			name:         "Nil engine provided to processor", // Este caso se testea en New, pero aquí para ProcessQuery
			queryPayload: []byte(`{"inputText": "hello"}`),
			// mockEvaluateFunc no se usa, el procesador se crea con nil engine
			wantResponse: nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var processor *LocalExpertSystemProcessor
			if tt.name == "Nil engine provided to processor" {
				processor = NewLocalExpertSystemProcessor(nil)
			} else {
				mockEngine := &MockRuleEngine{EvaluateFunc: tt.mockEvaluateFunc}
				processor = NewLocalExpertSystemProcessor(mockEngine)
			}

			gotResponse, err := processor.ProcessQuery(tt.queryPayload)

			if (err != nil) != tt.wantErr {
				t.Errorf("LocalExpertSystemProcessor.ProcessQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Si se espera un error, no comparar el cuerpo de la respuesta
			if tt.wantErr {
				return
			}

			// Comparar JSONs puede ser complicado por el orden de los campos. Deserializar y comparar.
			var got, want map[string]interface{}
			if err := json.Unmarshal(gotResponse, &got); err != nil {
				t.Fatalf("Failed to unmarshal actual response: %v, response: %s", err, string(gotResponse))
			}
			if err := json.Unmarshal(tt.wantResponse, &want); err != nil {
				t.Fatalf("Failed to unmarshal expected response: %v, response: %s", err, string(tt.wantResponse))
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("LocalExpertSystemProcessor.ProcessQuery() gotResponse = %s, want %s", string(gotResponse), string(tt.wantResponse))
			}
		})
	}
}

func TestNewLocalExpertSystemProcessor_NilEngine(t *testing.T) {
	// Esto es más una prueba de sanidad para el constructor, aunque ProcessQuery también lo cubre.
	processor := NewLocalExpertSystemProcessor(nil)
	if processor == nil {
		t.Fatal("NewLocalExpertSystemProcessor(nil) returned nil, expected a non-nil processor")
	}
	// La llamada a ProcessQuery con un engine nil debe fallar, como se prueba arriba.
}
