package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"slices"
	"time"
)

// Global verbose flag
var Verbose = true

// Global logger instance
var globalLogger *log.Logger

// Initialize the global logger
func init() {
	globalLogger = log.New(os.Stdout, "", log.LstdFlags)
}

// InitLogger initializes the logger with verbose and silent settings
func InitLogger(verbose bool, silent bool) {
	Verbose = verbose

	if silent {
		// Set logger to discard output (silent mode)
		globalLogger = log.New(io.Discard, "", log.LstdFlags)
	} else {
		// Set logger to output to stdout
		globalLogger = log.New(os.Stdout, "", log.LstdFlags)
	}
}

// GetLogger returns the current global logger
func GetLogger() *log.Logger {
	return globalLogger
}

// SetLogger sets the global logger to the provided logger
func SetLogger(logger *log.Logger) {
	if logger != nil {
		globalLogger = logger
	}
}

// LogInfo logs an info message
func LogInfo(format string, args ...interface{}) {
	globalLogger.Printf("[INFO] "+format, args...)
}

// LogDebug logs a debug message if verbose mode is enabled
func LogDebug(format string, args ...interface{}) {
	if Verbose {
		globalLogger.Printf("[DEBUG] "+format, args...)
	}
}

// LogError logs an error message
func LogError(format string, args ...interface{}) {
	globalLogger.Printf("[ERROR] "+format, args...)
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
	globalLogger.Printf("[SECURITY] Event: %s - Data: %s", eventType, string(jsonData))
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
	return slices.Contains(slice, item)
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
