package expert_system

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestRulesFile(t *testing.T, content string) string {
	tempDir := t.TempDir()
	tmpFilePath := filepath.Join(tempDir, "test_rules.json")
	if err := os.WriteFile(tmpFilePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp rules file: %v", err)
	}
	return tmpFilePath
}

func TestRuleEngine_LoadAndEvaluate(t *testing.T) {
	rulesContent := `
[
  {
    "id": "R_HIGH_PRIO",
    "priority": 100,
    "conditions": [{"fact": "InputText", "operator": "equals", "value": "high"}],
    "action": {"type": "high_action", "responsePayload": "high_payload"}
  },
  {
    "id": "R_LOW_PRIO",
    "priority": 10,
    "conditions": [{"fact": "InputText", "operator": "contains", "value": "low"}],
    "action": {"type": "low_action", "responsePayload": "low_payload"}
  },
  {
    "id": "R_MID_PRIO_FACTMAP",
    "priority": 50,
    "conditions": [
      {"fact": "data_type", "operator": "equals", "value": "urgent"},
      {"fact": "value", "operator": "gt", "value": 100}
    ],
    "action": {"type": "mid_action", "responsePayload": "mid_payload_urgent_gt100"}
  }
]`
	rulesFilePath := setupTestRulesFile(t, rulesContent)

	engine := NewRuleEngine()
	err := engine.LoadRules(rulesFilePath)
	if err != nil {
		t.Fatalf("Failed to load rules: %v", err)
	}

	if len(engine.rules) != 3 {
		t.Fatalf("Expected 3 rules to be loaded, got %d", len(engine.rules))
	}
	// Check sorting by priority
	if engine.rules[0].ID != "R_HIGH_PRIO" || engine.rules[1].ID != "R_MID_PRIO_FACTMAP" || engine.rules[2].ID != "R_LOW_PRIO" {
		t.Errorf("Rules not sorted by priority correctly. Order: %s, %s, %s", engine.rules[0].ID, engine.rules[1].ID, engine.rules[2].ID)
	}

	testCases := []struct {
		name          string
		input         QueryInput
		expectedRuleID string
		expectedPayload interface{}
		expectError   bool
	}{
		{
			name:          "Match high priority rule",
			input:         QueryInput{InputText: "high"},
			expectedRuleID: "R_HIGH_PRIO",
			expectedPayload: "high_payload",
			expectError:   false,
		},
		{
			name:          "Match low priority rule (high doesn't match)",
			input:         QueryInput{InputText: "some low string"},
			expectedRuleID: "R_LOW_PRIO",
			expectedPayload: "low_payload",
			expectError:   false,
		},
		{
			name: "Match mid priority factmap rule",
			input: QueryInput{
				FactMap: map[string]interface{}{"data_type": "urgent", "value": float64(150)},
			},
			expectedRuleID: "R_MID_PRIO_FACTMAP",
			expectedPayload: "mid_payload_urgent_gt100",
			expectError:   false,
		},
		{
			name: "No rule matches",
			input: QueryInput{
				InputText: "unknown",
				FactMap:   map[string]interface{}{"data_type": "normal"},
			},
			expectedRuleID: "",
			expectError:   true,
		},
		{
			name: "Mid priority rule condition not fully met",
			input: QueryInput{
				FactMap: map[string]interface{}{"data_type": "urgent", "value": float64(50)}, // value <= 100
			},
			expectedRuleID: "", // Should not match R_MID_PRIO_FACTMAP
			expectError:   true, // Expecting no rule match overall
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.Evaluate(tc.input)
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got none. Result: %+v", result)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
				if result.RuleID != tc.expectedRuleID {
					t.Errorf("Expected rule ID '%s', but got '%s'", tc.expectedRuleID, result.RuleID)
				}
				if result.ResponsePayload != tc.expectedPayload {
					t.Errorf("Expected payload '%v', but got '%v'", tc.expectedPayload, result.ResponsePayload)
				}
			}
		})
	}
}

func TestRuleEngine_NoRulesLoaded(t *testing.T) {
	engine := NewRuleEngine() // No rules loaded
	_, err := engine.Evaluate(QueryInput{InputText: "test"})
	if err == nil {
		t.Error("Expected error when no rules are loaded, got nil")
	}
}

func TestRuleEngine_LoadRules_FileNotFound(t *testing.T) {
	engine := NewRuleEngine()
	err := engine.LoadRules("nonexistent_rules.json")
	if err == nil {
		t.Error("Expected error when loading non-existent rules file, got nil")
	}
}

func TestRuleEngine_LoadRules_InvalidJSON(t *testing.T) {
	invalidJSONContent := `[{"id": "BAD_RULE", "priority": "not_a_number"}]` // Priority should be int
	rulesFilePath := setupTestRulesFile(t, invalidJSONContent)

	engine := NewRuleEngine()
	err := engine.LoadRules(rulesFilePath)
	if err == nil {
		t.Error("Expected error when loading invalid JSON rules file, got nil")
	}
}
