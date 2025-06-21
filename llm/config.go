package llm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"tripcodechain_go/utils" // Assuming utils for logging
)

// LLMConfig holds the configuration for the LLM.
type LLMConfig struct {
	Model string `json:"model"`
}

// LoadLLMConfig loads the LLM configuration from the specified file path.
// It defaults to "llm/config.json" if an empty path is provided.
func LoadLLMConfig(filePath string) (LLMConfig, error) {
	var config LLMConfig

	if filePath == "" {
		filePath = "llm/config.json" // Default path
	}

	// Ensure the path is absolute or relative to the execution directory
	// This might need adjustment based on how/where the binary is run.
	// For simplicity, let's assume it's relative to the project root or where `main.go` is.
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		utils.LogError("Error getting absolute path for config file %s: %v", filePath, err)
		return config, err
	}
	utils.LogInfo("Attempting to load LLM config from: %s", absPath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		utils.LogError("Error reading LLM config file %s: %v", absPath, err)
		return config, err
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		utils.LogError("Error unmarshalling LLM config from %s: %v", absPath, err)
		return config, err
	}
	if config.Model == "" {
		utils.LogInfo("LLM model is not specified in the config file: %s (Consider this a warning)", absPath)
		// Optionally return an error here if model is mandatory
		// return config, fmt.Errorf("LLM model is mandatory and not found in config: %s", absPath)
	} else {
		utils.LogInfo("LLM Config loaded successfully: Model='%s'", config.Model)
	}

	return config, nil
}
