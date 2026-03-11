package tester_common

import (
	"fmt"
	"os"
	"strings"

	"github.com/openbook/shared/envloader"
)

// Config holds the configuration for the testers
type Config struct {
	ServerHost     string
	ServerPort     string
	GatewayPort    string
	WebPort        string
	GatewayUserIDs []string
	GatewayLegIDs  []string
}

// LoadConfig loads and validates the configuration from .env file
func LoadConfig() (*Config, error) {
	// Load .env file from current directory (required)
	if err := envloader.LoadEnvFile(""); err != nil {
		return nil, err
	}

	// Load required configuration
	serverHost, err := envloader.GetEnvAsString("SERVER_HOST", true)
	if err != nil {
		return nil, err
	}

	serverPort, err := envloader.GetEnvAsString("SERVER_PORT", true)
	if err != nil {
		return nil, err
	}

	// Load optional configuration with defaults
	webPort := envloader.GetEnvAsStringWithDefault("WEB_PORT", "8080")
	gatewayPort := envloader.GetEnvAsStringWithDefault("GATEWAY_PORT", "50052")
	gatewayUserIDsStr := envloader.GetEnvAsStringWithDefault("GATEWAY_USER_IDS", "")
	var gatewayUserIDs []string
	if gatewayUserIDsStr != "" {
		for _, s := range strings.Split(gatewayUserIDsStr, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				gatewayUserIDs = append(gatewayUserIDs, trimmed)
			}
		}
	}
	gatewayLegIDsStr := envloader.GetEnvAsStringWithDefault("GATEWAY_LEG_IDS", "")
	var gatewayLegIDs []string
	if gatewayLegIDsStr != "" {
		for _, s := range strings.Split(gatewayLegIDsStr, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				gatewayLegIDs = append(gatewayLegIDs, trimmed)
			}
		}
	}

	return &Config{
		ServerHost:     serverHost,
		ServerPort:     serverPort,
		GatewayPort:    gatewayPort,
		WebPort:        webPort,
		GatewayUserIDs: gatewayUserIDs,
		GatewayLegIDs:  gatewayLegIDs,
	}, nil
}

// Fatal prints an error message to stderr and exits with code 1
func Fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

// SplitAndTrim splits a string by separator and trims whitespace from each element
func SplitAndTrim(s, sep string) []string {
	var result []string
	for _, item := range splitString(s, sep) {
		trimmed := TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

// TrimSpace trims leading and trailing whitespace from a string
func TrimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
