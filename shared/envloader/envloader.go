package envloader

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// LoadEnvFile loads environment variables from a .env file in the current directory
// If envFile is empty, it will try to load from .env in the current directory
// Returns an error if the file is not found or cannot be loaded
func LoadEnvFile(envFile string) error {
	// If no file specified, default to .env
	if envFile == "" {
		envFile = ".env"
	}

	// Load the environment file - fail if it doesn't exist
	if err := godotenv.Load(envFile); err != nil {
		return fmt.Errorf("failed to load environment file %s: %w", envFile, err)
	}

	return nil
}

// GetEnvAsString reads an environment variable as a string
// If required is true and the variable is not set or empty, returns an error
func GetEnvAsString(key string, required bool) (string, error) {
	value := os.Getenv(key)
	if required && value == "" {
		return "", fmt.Errorf("missing required environment variable: %s", key)
	}
	return value, nil
}

// GetEnvAsStringWithDefault reads an environment variable as a string or returns a default value
func GetEnvAsStringWithDefault(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvAsInt reads an environment variable as an integer
// If required is true and the variable is not set or empty, returns an error
// If the variable is set but not a valid integer, returns an error
func GetEnvAsInt(key string, required bool) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		if required {
			return 0, fmt.Errorf("missing required environment variable: %s", key)
		}
		return 0, nil
	}

	intVal, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %w", key, err)
	}

	return intVal, nil
}

// GetEnvAsIntWithDefault reads an environment variable as an integer or returns a default value
// If the variable is set but not a valid integer, returns the default value
func GetEnvAsIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// ValidateRequired checks that all required environment variables are set and non-empty
// Returns an error with the first missing variable, or nil if all are present
func ValidateRequired(keys []string) error {
	for _, key := range keys {
		if os.Getenv(key) == "" {
			return fmt.Errorf("missing required environment variable: %s", key)
		}
	}
	return nil
}
