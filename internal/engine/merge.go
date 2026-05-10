package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/smm-h/migrable/internal/ops"
)

// Merge combines staging files from next/ into a single versioned migration file.
// Returns the output file path.
func Merge(migrationsDir string, version string) (string, error) {
	outPath := filepath.Join(migrationsDir, version+".toml")

	// Check that the target file does not already exist.
	if _, err := os.Stat(outPath); err == nil {
		return "", fmt.Errorf("migration file %s already exists", outPath)
	}

	nextDir := filepath.Join(migrationsDir, "next")
	entries, err := os.ReadDir(nextDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("next/ directory does not exist")
		}
		return "", fmt.Errorf("failed to read next/ directory: %w", err)
	}

	var stagingFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		stagingFiles = append(stagingFiles, entry.Name())
	}
	sort.Strings(stagingFiles)

	if len(stagingFiles) == 0 {
		return "", fmt.Errorf("no staging files in next/")
	}

	// Parse all staging files.
	type parsedFile struct {
		name      string
		migration *ops.Migration
	}
	var parsed []parsedFile
	for _, name := range stagingFiles {
		data, readErr := os.ReadFile(filepath.Join(nextDir, name))
		if readErr != nil {
			return "", fmt.Errorf("failed to read %s: %w", name, readErr)
		}
		migration, parseErr := ops.ParseMigration(data)
		if parseErr != nil {
			return "", fmt.Errorf("%s: %w", name, parseErr)
		}
		parsed = append(parsed, parsedFile{name: name, migration: migration})
	}

	// Combine descriptions.
	var descriptions []string
	for _, p := range parsed {
		if p.migration.Description != "" {
			descriptions = append(descriptions, p.migration.Description)
		}
	}
	combinedDesc := strings.Join(descriptions, "; ")

	// Combine structure and data ops.
	var allStructure []ops.Op
	var allData []ops.Op
	for _, p := range parsed {
		allStructure = append(allStructure, p.migration.Structure...)
		allData = append(allData, p.migration.Data...)
	}

	// Serialize to TOML.
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("description = %q\n", combinedDesc))

	for _, op := range allStructure {
		buf.WriteString("\n[[structure]]\n")
		serializeOp(&buf, op)
	}

	for _, op := range allData {
		buf.WriteString("\n[[data]]\n")
		serializeOp(&buf, op)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := WriteFileAtomic(outPath, []byte(buf.String())); err != nil {
		return "", fmt.Errorf("failed to write merged migration: %w", err)
	}

	// Delete staging files (keep the next/ directory).
	for _, name := range stagingFiles {
		if err := os.Remove(filepath.Join(nextDir, name)); err != nil {
			return "", fmt.Errorf("failed to remove staging file %s: %w", name, err)
		}
	}

	return outPath, nil
}

func serializeOp(buf *strings.Builder, op ops.Op) {
	buf.WriteString(fmt.Sprintf("op = %q\n", string(op.Type)))

	switch op.Type {
	case ops.OpAddField:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))
		buf.WriteString(fmt.Sprintf("type = %q\n", op.FieldType))
		if op.Default != nil {
			buf.WriteString(fmt.Sprintf("default = %s\n", formatTOMLValue(op.Default)))
		}

	case ops.OpRemoveField:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))

	case ops.OpRenameField:
		buf.WriteString(fmt.Sprintf("from = %q\n", op.From))
		buf.WriteString(fmt.Sprintf("to = %q\n", op.To))

	case ops.OpMoveField:
		buf.WriteString(fmt.Sprintf("from = %q\n", op.From))
		buf.WriteString(fmt.Sprintf("to = %q\n", op.To))

	case ops.OpAddCollection:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))
		if op.Fields != nil {
			buf.WriteString(fmt.Sprintf("fields = %s\n", formatTOMLValue(op.Fields)))
		}

	case ops.OpDropCollection:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))

	case ops.OpSetValue:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))
		if op.Value != nil {
			buf.WriteString(fmt.Sprintf("value = %s\n", formatTOMLValue(op.Value)))
		}

	case ops.OpSetValueWhere:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))
		if op.Where != nil {
			buf.WriteString(fmt.Sprintf("where = %s\n", formatTOMLValue(op.Where)))
		}
		if op.MatchMode != "" {
			buf.WriteString(fmt.Sprintf("match_mode = %q\n", op.MatchMode))
		}
		if op.Set != nil {
			buf.WriteString(fmt.Sprintf("set = %s\n", formatTOMLValue(op.Set)))
		}

	case ops.OpRemoveWhere:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))
		if op.Where != nil {
			buf.WriteString(fmt.Sprintf("where = %s\n", formatTOMLValue(op.Where)))
		}
		if op.MatchMode != "" {
			buf.WriteString(fmt.Sprintf("match_mode = %q\n", op.MatchMode))
		}

	case ops.OpAppend:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))
		if op.Value != nil {
			buf.WriteString(fmt.Sprintf("value = %s\n", formatTOMLValue(op.Value)))
		}

	case ops.OpTransform:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))
		buf.WriteString(fmt.Sprintf("expr = %q\n", op.Expr))

	case ops.OpMergeDefaults:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))
		if op.Value != nil {
			buf.WriteString(fmt.Sprintf("value = %s\n", formatTOMLValue(op.Value)))
		}

	case ops.OpMergeDefaultsByKey:
		buf.WriteString(fmt.Sprintf("path = %q\n", op.Path))
		buf.WriteString(fmt.Sprintf("match_field = %q\n", op.MatchField))
		if op.Defaults != nil {
			buf.WriteString(fmt.Sprintf("defaults = %s\n", formatTOMLArray(op.Defaults)))
		}

	case ops.OpRaw:
		buf.WriteString(fmt.Sprintf("content = %q\n", op.Content))
	}

	// Serialize down op.
	if op.Down != nil {
		serializeDown(buf, op.Down)
	}
}

func serializeDown(buf *strings.Builder, down *ops.DownOp) {
	if down.Irreversible {
		buf.WriteString("down = \"irreversible\"\n")
		return
	}
	if len(down.Ops) == 1 {
		buf.WriteString(fmt.Sprintf("down = %s\n", serializeDownOpInline(down.Ops[0])))
		return
	}
	parts := make([]string, len(down.Ops))
	for i, op := range down.Ops {
		parts[i] = serializeDownOpInline(op)
	}
	buf.WriteString(fmt.Sprintf("down = [%s]\n", strings.Join(parts, ", ")))
}

func serializeDownOpInline(op ops.Op) string {
	parts := []string{fmt.Sprintf("op = %q", string(op.Type))}
	if op.Path != "" {
		parts = append(parts, fmt.Sprintf("path = %q", op.Path))
	}
	if op.From != "" {
		parts = append(parts, fmt.Sprintf("from = %q", op.From))
	}
	if op.To != "" {
		parts = append(parts, fmt.Sprintf("to = %q", op.To))
	}
	if op.FieldType != "" {
		parts = append(parts, fmt.Sprintf("type = %q", op.FieldType))
	}
	if op.Default != nil {
		parts = append(parts, fmt.Sprintf("default = %s", formatTOMLValue(op.Default)))
	}
	if op.Value != nil {
		parts = append(parts, fmt.Sprintf("value = %s", formatTOMLValue(op.Value)))
	}
	if op.Content != "" {
		parts = append(parts, fmt.Sprintf("content = %q", op.Content))
	}
	if op.Expr != "" {
		parts = append(parts, fmt.Sprintf("expr = %q", op.Expr))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatTOMLValue(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case int:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatTOMLValue(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		return formatTOMLInlineTable(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatTOMLInlineTable(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s = %s", k, formatTOMLValue(m[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatTOMLArray(maps []map[string]any) string {
	parts := make([]string, len(maps))
	for i, m := range maps {
		parts[i] = formatTOMLInlineTable(m)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
