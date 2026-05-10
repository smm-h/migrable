package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/smm-h/migrable/internal/ops"
)

// ValidationIssue describes a single problem found during validation.
type ValidationIssue struct {
	File    string
	Message string
}

// ValidationResult collects all errors and warnings from validation.
type ValidationResult struct {
	FileCount int
	Errors    []ValidationIssue
	Warnings  []ValidationIssue
}

// Validate checks all migration files in migrationsDir (and next/ if it exists)
// for correctness. It collects all issues rather than stopping at the first.
func Validate(migrationsDir string) (*ValidationResult, error) {
	result := &ValidationResult{}

	// Validate versioned migration files.
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory %s: %w", migrationsDir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		files = append(files, filepath.Join(migrationsDir, entry.Name()))
	}
	sort.Strings(files)

	for _, f := range files {
		validateFile(f, result)
	}

	result.FileCount = len(files)

	// Validate staging files in next/ if the directory exists.
	nextDir := filepath.Join(migrationsDir, "next")
	if info, statErr := os.Stat(nextDir); statErr == nil && info.IsDir() {
		nextEntries, readErr := os.ReadDir(nextDir)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read next/ directory %s: %w", nextDir, readErr)
		}

		var nextFiles []string
		for _, entry := range nextEntries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".toml") {
				continue
			}
			nextFiles = append(nextFiles, filepath.Join(nextDir, entry.Name()))
		}
		sort.Strings(nextFiles)

		for _, f := range nextFiles {
			validateFile(f, result)
		}
		result.FileCount += len(nextFiles)
	}

	return result, nil
}

func validateFile(filePath string, result *ValidationResult) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		result.Errors = append(result.Errors, ValidationIssue{
			File:    filePath,
			Message: fmt.Sprintf("failed to read file: %v", err),
		})
		return
	}

	migration, err := ops.ParseMigration(data)
	if err != nil {
		result.Errors = append(result.Errors, ValidationIssue{
			File:    filePath,
			Message: fmt.Sprintf("parse error: %v", err),
		})
		return
	}

	if migration.Description == "" {
		result.Warnings = append(result.Warnings, ValidationIssue{
			File:    filePath,
			Message: "missing description",
		})
	}

	for i, op := range migration.Structure {
		validateOp(filePath, fmt.Sprintf("structure[%d]", i), op, result)
	}
	for i, op := range migration.Data {
		validateOp(filePath, fmt.Sprintf("data[%d]", i), op, result)
	}
}

func validateOp(filePath string, location string, op ops.Op, result *ValidationResult) {
	// Check that down op is present.
	if op.Down == nil {
		result.Errors = append(result.Errors, ValidationIssue{
			File:    filePath,
			Message: fmt.Sprintf("%s (%s): missing down op", location, op.Type),
		})
	}

	// Type check for add_field: verify default matches declared type.
	if op.Type == ops.OpAddField && op.Default != nil && op.FieldType != "" {
		if err := checkDefaultType(op.FieldType, op.Default); err != nil {
			result.Errors = append(result.Errors, ValidationIssue{
				File:    filePath,
				Message: fmt.Sprintf("%s (%s): %v", location, op.Type, err),
			})
		}
	}
}

func checkDefaultType(declaredType string, defaultVal any) error {
	switch declaredType {
	case "string":
		if _, ok := defaultVal.(string); !ok {
			return fmt.Errorf("default value has type %T, expected string", defaultVal)
		}
	case "integer":
		switch defaultVal.(type) {
		case int64, int:
			// ok
		default:
			return fmt.Errorf("default value has type %T, expected integer", defaultVal)
		}
	case "float":
		if _, ok := defaultVal.(float64); !ok {
			return fmt.Errorf("default value has type %T, expected float", defaultVal)
		}
	case "boolean":
		if _, ok := defaultVal.(bool); !ok {
			return fmt.Errorf("default value has type %T, expected boolean", defaultVal)
		}
	case "datetime":
		if _, ok := defaultVal.(time.Time); !ok {
			return fmt.Errorf("default value has type %T, expected datetime", defaultVal)
		}
	case "array":
		if _, ok := defaultVal.([]any); !ok {
			return fmt.Errorf("default value has type %T, expected array", defaultVal)
		}
	case "table":
		if _, ok := defaultVal.(map[string]any); !ok {
			return fmt.Errorf("default value has type %T, expected table", defaultVal)
		}
	}
	return nil
}
