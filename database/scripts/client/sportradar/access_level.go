package sportradar

import "fmt"

// AccessLevel represents the Sportradar API access level
type AccessLevel string

const (
	// AccessLevelTrial represents trial API access
	AccessLevelTrial AccessLevel = "trial"
	// AccessLevelProduction represents production API access
	AccessLevelProduction AccessLevel = "production"
)

// Validate checks if the access level is valid
func (a AccessLevel) Validate() error {
	switch a {
	case AccessLevelTrial, AccessLevelProduction:
		return nil
	default:
		return fmt.Errorf("invalid access level '%s': must be 'trial' or 'production'", a)
	}
}

// String returns the string representation of the access level
func (a AccessLevel) String() string {
	return string(a)
}
