package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidationError is a single field-level problem.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors collects multiple problems so the user sees everything at once.
type ValidationErrors []ValidationError

func (es ValidationErrors) Error() string {
	if len(es) == 0 {
		return "config is valid"
	}
	var b strings.Builder
	b.WriteString("config validation failed:\n")
	for _, e := range es {
		b.WriteString("  • ")
		b.WriteString(e.Error())
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (es ValidationErrors) Empty() bool {
	return len(es) == 0
}

// Validate checks the Config against the platform schema.
// It returns a ValidationErrors value (possibly empty).
// Callers should treat a non-empty result as failure.
//
// Design rule: every new field that Auth / Queue / Deploy / etc. depend on
// must be validated here. Do not scatter checks across packages.
func (c *Config) Validate() ValidationErrors {
	var errs ValidationErrors

	// environment
	if c.Environment == "" {
		errs = append(errs, ValidationError{"environment", "must not be empty"})
	} else if !contains(ValidEnvironments, c.Environment) {
		errs = append(errs, ValidationError{
			"environment",
			fmt.Sprintf("must be one of %s (got %q)", strings.Join(ValidEnvironments, "|"), c.Environment),
		})
	}

	// log_level
	if c.LogLevel == "" {
		errs = append(errs, ValidationError{"log_level", "must not be empty"})
	} else if !contains(ValidLogLevels, c.LogLevel) {
		errs = append(errs, ValidationError{
			"log_level",
			fmt.Sprintf("must be one of %s (got %q)", strings.Join(ValidLogLevels, "|"), c.LogLevel),
		})
	}

	// data_dir — if set, should look like a path
	if c.DataDir != "" {
		if strings.Contains(c.DataDir, "\x00") {
			errs = append(errs, ValidationError{"data_dir", "contains invalid characters"})
		}
	}

	// services
	seen := map[string]bool{}
	for i, svc := range c.Services {
		field := fmt.Sprintf("services[%d]", i)
		name := strings.TrimSpace(svc.Name)
		if name == "" {
			errs = append(errs, ValidationError{field + ".name", "must not be empty"})
			continue
		}
		if seen[name] {
			errs = append(errs, ValidationError{field + ".name", fmt.Sprintf("duplicate service name %q", name)})
		}
		seen[name] = true

		if svc.Path != "" {
			if filepath.IsAbs(svc.Path) {
				errs = append(errs, ValidationError{field + ".path", "must be relative to the project root"})
			}
			if strings.Contains(svc.Path, "..") {
				errs = append(errs, ValidationError{field + ".path", "must not contain .."})
			}
		}
	}

	// project_name — optional but if present should be a sensible identifier
	if c.ProjectName != "" {
		if len(c.ProjectName) > 128 {
			errs = append(errs, ValidationError{"project_name", "must be ≤ 128 characters"})
		}
		for _, r := range c.ProjectName {
			if !isAllowedProjectNameChar(r) {
				errs = append(errs, ValidationError{
					"project_name",
					"may only contain letters, digits, hyphens, underscores and dots",
				})
				break
			}
		}
	}

	return errs
}

// ValidateStrict is a convenience that returns a normal error if invalid.
func (c *Config) ValidateStrict() error {
	errs := c.Validate()
	if errs.Empty() {
		return nil
	}
	return errs
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func isAllowedProjectNameChar(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '-' || r == '_' || r == '.'
}
