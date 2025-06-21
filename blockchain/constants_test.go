package blockchain

import (
	"os"
	"testing"
)

func TestGetNumberOfDelegates(t *testing.T) {
	// Helper function to safely get and restore environment variables
	setEnvForTest := func(t *testing.T, key, value string) {
		originalValue, wasSet := os.LookupEnv(key)
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("Failed to set env %s: %v", key, err)
		}
		t.Cleanup(func() {
			if !wasSet {
				os.Unsetenv(key)
			} else {
				if err := os.Setenv(key, originalValue); err != nil {
					t.Logf("Failed to restore env %s: %v", key, err) // Log instead of Fatal in cleanup
				}
			}
		})
	}

	unsetEnvForTest := func(t *testing.T, key string) {
		originalValue, wasSet := os.LookupEnv(key)
		if !wasSet { // Nothing to do if not set
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("Failed to unset env %s: %v", key, err)
		}
		t.Cleanup(func() {
			// Only restore if it was originally set.
			// If it wasn't set, Unsetenv did its job and we shouldn't try to Setenv to empty.
			if wasSet {
				if err := os.Setenv(key, originalValue); err != nil {
					t.Logf("Failed to restore env %s after unsetting: %v", key, err)
				}
			}
		})
	}

	t.Run("DevelopmentEnv", func(t *testing.T) {
		setEnvForTest(t, "APP_ENV", "development")
		expected := NumberOfDelegatesDev
		actual := GetNumberOfDelegates()
		if actual != expected {
			t.Errorf("Expected %d delegates for APP_ENV=development, got %d", expected, actual)
		}
	})

	t.Run("ProductionEnv", func(t *testing.T) {
		setEnvForTest(t, "APP_ENV", "production")
		expected := NumberOfDelegates
		actual := GetNumberOfDelegates()
		if actual != expected {
			t.Errorf("Expected %d delegates for APP_ENV=production, got %d", expected, actual)
		}
	})

	t.Run("NonDevelopmentEnv", func(t *testing.T) {
		setEnvForTest(t, "APP_ENV", "staging") // Any value other than "development"
		expected := NumberOfDelegates
		actual := GetNumberOfDelegates()
		if actual != expected {
			t.Errorf("Expected %d delegates for APP_ENV=staging, got %d", expected, actual)
		}
	})

	t.Run("UnsetEnv", func(t *testing.T) {
		unsetEnvForTest(t, "APP_ENV")
		expected := NumberOfDelegates
		actual := GetNumberOfDelegates()
		if actual != expected {
			t.Errorf("Expected %d delegates when APP_ENV is not set, got %d", expected, actual)
		}
	})
}
