// utils/logger.go
package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"
)

// Global verbose flag
var Verbose = true

// LogInfo logs an info message
func LogInfo(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

// LogDebug logs a debug message if verbose mode is enabled
func LogDebug(format string, args ...interface{}) {
	if Verbose {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// LogError logs an error message
func LogError(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}

// LogSecurityEvent logs a security-related event with structured data
func LogSecurityEvent(eventType string, data map[string]interface{}) {
	// Add timestamp to the data
	data["timestamp"] = time.Now().Format(time.RFC3339)
	data["event_type"] = eventType

	// Convert to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		LogError("Failed to marshal security event data: %v", err)
		return
	}

	// Log the security event
	log.Printf("[SECURITY] Event: %s - Data: %s", eventType, string(jsonData))
}

// SetVerbose sets the verbose logging mode
func SetVerbose(v bool) {
	Verbose = v
}

// GetVerbose returns the current verbose logging mode
func GetVerbose() bool {
	return Verbose
}

// Contains checks if a string is in a slice
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// NewSeededRand creates a new seeded random number generator
func NewSeededRand(seed int64) *rand.Rand {
	source := rand.NewSource(seed)
	return rand.New(source)
}

// PrintStartupMessage prints a formatted startup message
func PrintStartupMessage(nodeID string, port int) {
	fmt.Println("---------------------------------------------------")
	fmt.Printf("| Blockchain Node Started                          |\n")
	fmt.Printf("| Node ID: %-38s |\n", nodeID)
	fmt.Printf("| Port: %-41d |\n", port)
	fmt.Printf("| Mode: %-41s |\n", fmt.Sprintf("HTTP Server (:%d)", port))
	fmt.Println("---------------------------------------------------")
}
