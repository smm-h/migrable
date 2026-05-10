package ops

import (
	"fmt"

	tomledit "github.com/smm-h/go-toml-edit"
)

type Migration struct {
	Description string
	Structure   []Op
	Data        []Op
}

func ParseMigration(data []byte) (*Migration, error) {
	var raw struct {
		Description string           `toml:"description"`
		Structure   []map[string]any `toml:"structure"`
		Data        []map[string]any `toml:"data"`
	}
	if err := tomledit.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse migration TOML: %w", err)
	}

	m := &Migration{Description: raw.Description}

	for i, entry := range raw.Structure {
		op, err := parseOp(entry, "structure")
		if err != nil {
			return nil, fmt.Errorf("structure[%d]: %w", i, err)
		}
		m.Structure = append(m.Structure, op)
	}

	for i, entry := range raw.Data {
		op, err := parseOp(entry, "data")
		if err != nil {
			return nil, fmt.Errorf("data[%d]: %w", i, err)
		}
		m.Data = append(m.Data, op)
	}

	return m, nil
}

var validOpTypes = map[OpType]bool{
	OpAddField:           true,
	OpRemoveField:        true,
	OpRenameField:        true,
	OpMoveField:          true,
	OpAddCollection:      true,
	OpDropCollection:     true,
	OpSetValue:           true,
	OpSetValueWhere:      true,
	OpRemoveWhere:        true,
	OpAppend:             true,
	OpTransform:          true,
	OpMergeDefaults:      true,
	OpMergeDefaultsByKey: true,
	OpRaw:                true,
}

func parseOp(entry map[string]any, section string) (Op, error) {
	opStr, ok := entry["op"].(string)
	if !ok {
		return Op{}, fmt.Errorf("missing or invalid \"op\" field")
	}
	opType := OpType(opStr)
	if !validOpTypes[opType] {
		return Op{}, fmt.Errorf("unknown op type %q", opStr)
	}

	op := Op{
		Type:    opType,
		Section: section,
	}

	if v, ok := entry["path"].(string); ok {
		op.Path = v
	}

	if err := parseOpFields(entry, &op); err != nil {
		return Op{}, err
	}

	if downRaw, ok := entry["down"]; ok {
		down, err := parseDown(downRaw)
		if err != nil {
			return Op{}, fmt.Errorf("down: %w", err)
		}
		op.Down = down
	}

	if err := validateRequiredFields(op); err != nil {
		return Op{}, err
	}

	return op, nil
}

func parseOpFields(entry map[string]any, op *Op) error {
	switch op.Type {
	case OpAddField:
		if v, ok := entry["type"].(string); ok {
			op.FieldType = v
		}
		if v, ok := entry["default"]; ok {
			op.Default = v
		}

	case OpRemoveField:
		// path only

	case OpRenameField:
		if v, ok := entry["from"].(string); ok {
			op.From = v
		}
		if v, ok := entry["to"].(string); ok {
			op.To = v
		}

	case OpMoveField:
		if v, ok := entry["from"].(string); ok {
			op.From = v
		}
		if v, ok := entry["to"].(string); ok {
			op.To = v
		}

	case OpAddCollection:
		if v, ok := entry["fields"].(map[string]any); ok {
			op.Fields = v
		}

	case OpDropCollection:
		// path only

	case OpSetValue:
		if v, ok := entry["value"]; ok {
			op.Value = v
		}

	case OpSetValueWhere:
		if v, ok := entry["where"]; ok {
			op.Where = v
		}
		if v, ok := entry["match_mode"].(string); ok {
			op.MatchMode = v
		}
		if v, ok := entry["set"].(map[string]any); ok {
			op.Set = v
		}

	case OpRemoveWhere:
		if v, ok := entry["where"]; ok {
			op.Where = v
		}
		if v, ok := entry["match_mode"].(string); ok {
			op.MatchMode = v
		}

	case OpAppend:
		if v, ok := entry["value"]; ok {
			op.Value = v
		}

	case OpTransform:
		if v, ok := entry["expr"].(string); ok {
			op.Expr = v
		}

	case OpMergeDefaults:
		if v, ok := entry["value"]; ok {
			op.Value = v
		}

	case OpMergeDefaultsByKey:
		if v, ok := entry["match_field"].(string); ok {
			op.MatchField = v
		}
		if v, ok := entry["defaults"]; ok {
			op.Defaults = toMapSlice(v)
		}

	case OpRaw:
		if v, ok := entry["content"].(string); ok {
			op.Content = v
		}
	}

	return nil
}

func parseDown(raw any) (*DownOp, error) {
	switch v := raw.(type) {
	case string:
		if v == "irreversible" {
			return &DownOp{Irreversible: true}, nil
		}
		return nil, fmt.Errorf("invalid down string %q (expected \"irreversible\")", v)

	case map[string]any:
		op, err := parseOp(v, "")
		if err != nil {
			return nil, err
		}
		return &DownOp{Ops: []Op{op}}, nil

	case []any:
		var ops []Op
		for i, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("[%d]: expected table, got %T", i, item)
			}
			op, err := parseOp(m, "")
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			ops = append(ops, op)
		}
		return &DownOp{Ops: ops}, nil

	default:
		return nil, fmt.Errorf("unsupported down type %T", raw)
	}
}

func validateRequiredFields(op Op) error {
	switch op.Type {
	case OpAddField:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
		if op.FieldType == "" {
			return fmt.Errorf("%s: missing required field \"type\"", op.Type)
		}
	case OpRemoveField:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
	case OpRenameField:
		if op.From == "" {
			return fmt.Errorf("%s: missing required field \"from\"", op.Type)
		}
		if op.To == "" {
			return fmt.Errorf("%s: missing required field \"to\"", op.Type)
		}
	case OpMoveField:
		if op.From == "" {
			return fmt.Errorf("%s: missing required field \"from\"", op.Type)
		}
		if op.To == "" {
			return fmt.Errorf("%s: missing required field \"to\"", op.Type)
		}
	case OpAddCollection:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
	case OpDropCollection:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
	case OpSetValue:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
	case OpSetValueWhere:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
	case OpRemoveWhere:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
	case OpAppend:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
	case OpTransform:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
		if op.Expr == "" {
			return fmt.Errorf("%s: missing required field \"expr\"", op.Type)
		}
	case OpMergeDefaults:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
	case OpMergeDefaultsByKey:
		if op.Path == "" {
			return fmt.Errorf("%s: missing required field \"path\"", op.Type)
		}
		if op.MatchField == "" {
			return fmt.Errorf("%s: missing required field \"match_field\"", op.Type)
		}
	case OpRaw:
		if op.Content == "" {
			return fmt.Errorf("%s: missing required field \"content\"", op.Type)
		}
	}
	return nil
}

func toMapSlice(v any) []map[string]any {
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	var result []map[string]any
	for _, item := range slice {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}
