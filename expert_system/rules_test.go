package expert_system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRules(t *testing.T) {
	// Crear un archivo de reglas temporal para la prueba
	tempDir := t.TempDir()
	rulesContent := `
[
  {
    "id": "TEST_RULE_001",
    "description": "Test rule",
    "priority": 10,
    "conditions": [
      {"fact": "InputText", "operator": "equals", "value": "test"}
    ],
    "action": {
      "type": "test_action",
      "responsePayload": "test_payload"
    }
  }
]`
	tmpFilePath := filepath.Join(tempDir, "test_rules.json")
	if err := os.WriteFile(tmpFilePath, []byte(rulesContent), 0644); err != nil {
		t.Fatalf("Failed to write temp rules file: %v", err)
	}

	rules, err := LoadRules(tmpFilePath)
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	if len(rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(rules))
	}

	if rules[0].ID != "TEST_RULE_001" {
		t.Errorf("Expected rule ID 'TEST_RULE_001', got '%s'", rules[0].ID)
	}
	if rules[0].Action.ResponsePayload != "test_payload" {
		t.Errorf("Expected action responsePayload 'test_payload', got '%s'", rules[0].Action.ResponsePayload)
	}

	// Probar carga de archivo inexistente
	_, err = LoadRules("non_existent_rules.json")
	if err == nil {
		t.Errorf("Expected error when loading non-existent file, got nil")
	}
}

func TestCondition_evaluate(t *testing.T) {
	tests := []struct {
		name      string
		condition Condition
		input     QueryInput
		want      bool
		wantErr   bool
	}{
		{
			name:      "InputText equals - match",
			condition: Condition{Fact: "InputText", Operator: "equals", Value: "hello"},
			input:     QueryInput{InputText: "hello"},
			want:      true,
			wantErr:   false,
		},
		{
			name:      "InputText equals - no match",
			condition: Condition{Fact: "InputText", Operator: "equals", Value: "hello"},
			input:     QueryInput{InputText: "world"},
			want:      false,
			wantErr:   false,
		},
		{
			name:      "InputText equals - case insensitive match",
			condition: Condition{Fact: "InputText", Operator: "equals", Value: "Hello"},
			input:     QueryInput{InputText: "hello"},
			want:      true,
			wantErr:   false,
		},
		{
			name:      "FactMap contains - match",
			condition: Condition{Fact: "status", Operator: "contains", Value: "active"},
			input:     QueryInput{FactMap: map[string]interface{}{"status": "system is active"}},
			want:      true,
			wantErr:   false,
		},
		{
			name:      "FactMap contains - no match",
			condition: Condition{Fact: "status", Operator: "contains", Value: "inactive"},
			input:     QueryInput{FactMap: map[string]interface{}{"status": "system is active"}},
			want:      false,
			wantErr:   false,
		},
		{
			name:      "FactMap gt - match",
			condition: Condition{Fact: "count", Operator: "gt", Value: float64(5)},
			input:     QueryInput{FactMap: map[string]interface{}{"count": float64(10)}},
			want:      true,
			wantErr:   false,
		},
		{
			name:      "FactMap gt - no match",
			condition: Condition{Fact: "count", Operator: "gt", Value: float64(10)},
			input:     QueryInput{FactMap: map[string]interface{}{"count": float64(10)}},
			want:      false,
			wantErr:   false,
		},
		{
			name:      "FactMap lt - match",
			condition: Condition{Fact: "count", Operator: "lt", Value: float64(10)},
			input:     QueryInput{FactMap: map[string]interface{}{"count": float64(5)}},
			want:      true,
			wantErr:   false,
		},
		{
			name:      "gval expression - true",
			condition: Condition{Fact: "value", Operator: "evaluate_gval", Value: "value > 10 && value < 20"},
			input:     QueryInput{FactMap: map[string]interface{}{"value": float64(15)}},
			want:      true,
			wantErr:   false,
		},
		{
			name:      "gval expression - false",
			condition: Condition{Fact: "value", Operator: "evaluate_gval", Value: "value > 10 && value < 20"},
			input:     QueryInput{FactMap: map[string]interface{}{"value": float64(25)}},
			want:      false,
			wantErr:   false,
		},
		{
			name:      "gval expression - uses InputText (not configured by default, should fail or be adapted)",
			condition: Condition{Fact: "InputText", Operator: "evaluate_gval", Value: "len(InputText) > 3"}, // This setup of gval in rules.go doesn't automatically add InputText to gval context
			input:     QueryInput{InputText: "hello world"},
			want:      false, // Expecting an error or specific handling if InputText not in gval context
			wantErr:   true,  // Error because 'InputText' is not in gval context unless explicitly added.
		},
		{
			name:      "unknown operator",
			condition: Condition{Fact: "InputText", Operator: "unknown_op", Value: "test"},
			input:     QueryInput{InputText: "test"},
			want:      false,
			wantErr:   true,
		},
		{
			name:      "fact not found",
			condition: Condition{Fact: "nonexistentFact", Operator: "equals", Value: "test"},
			input:     QueryInput{InputText: "test"},
			want:      false,
			wantErr:   true,
		},
		{
			name:      "InputText matches_regex - match",
			condition: Condition{Fact: "InputText", Operator: "matches_regex", Value: "^hello.*world$"},
			input:     QueryInput{InputText: "hello amazing world"},
			want:      true,
			wantErr:   false,
		},
		{
			name:      "InputText matches_regex - no match",
			condition: Condition{Fact: "InputText", Operator: "matches_regex", Value: "^hello world$"},
			input:     QueryInput{InputText: "hello_world"},
			want:      false,
			wantErr:   false,
		},
		{
			name:      "InputText matches_regex - invalid regex pattern",
			condition: Condition{Fact: "InputText", Operator: "matches_regex", Value: "[{invalid_regex"},
			input:     QueryInput{InputText: "test"},
			want:      false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.condition.evaluate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Condition.evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Condition.evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}
