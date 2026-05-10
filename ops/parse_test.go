package ops

import (
	"strings"
	"testing"
)

func TestParseMigration_Complete(t *testing.T) {
	input := []byte(`
description = "add user settings"

[[structure]]
op = "add_field"
path = "settings.theme"
type = "string"
default = "dark"

[[structure]]
op = "remove_field"
path = "legacy.old_key"

[[data]]
op = "set_value"
path = "app.version"
value = "2.0"
`)

	m, err := ParseMigration(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Description != "add user settings" {
		t.Errorf("description = %q, want %q", m.Description, "add user settings")
	}
	if len(m.Structure) != 2 {
		t.Fatalf("got %d structure ops, want 2", len(m.Structure))
	}
	if len(m.Data) != 1 {
		t.Fatalf("got %d data ops, want 1", len(m.Data))
	}

	op0 := m.Structure[0]
	if op0.Type != OpAddField {
		t.Errorf("structure[0].Type = %q, want %q", op0.Type, OpAddField)
	}
	if op0.Path != "settings.theme" {
		t.Errorf("structure[0].Path = %q, want %q", op0.Path, "settings.theme")
	}
	if op0.FieldType != "string" {
		t.Errorf("structure[0].FieldType = %q, want %q", op0.FieldType, "string")
	}
	if op0.Default != "dark" {
		t.Errorf("structure[0].Default = %v, want %q", op0.Default, "dark")
	}
	if op0.Section != "structure" {
		t.Errorf("structure[0].Section = %q, want %q", op0.Section, "structure")
	}

	op1 := m.Structure[1]
	if op1.Type != OpRemoveField {
		t.Errorf("structure[1].Type = %q, want %q", op1.Type, OpRemoveField)
	}

	dop := m.Data[0]
	if dop.Type != OpSetValue {
		t.Errorf("data[0].Type = %q, want %q", dop.Type, OpSetValue)
	}
	if dop.Section != "data" {
		t.Errorf("data[0].Section = %q, want %q", dop.Section, "data")
	}
}

func TestParseMigration_StructureOnly(t *testing.T) {
	input := []byte(`
[[structure]]
op = "remove_field"
path = "old.key"
`)
	m, err := ParseMigration(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Structure) != 1 {
		t.Fatalf("got %d structure ops, want 1", len(m.Structure))
	}
	if len(m.Data) != 0 {
		t.Fatalf("got %d data ops, want 0", len(m.Data))
	}
}

func TestParseMigration_DataOnly(t *testing.T) {
	input := []byte(`
[[data]]
op = "set_value"
path = "app.name"
value = "test"
`)
	m, err := ParseMigration(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Structure) != 0 {
		t.Fatalf("got %d structure ops, want 0", len(m.Structure))
	}
	if len(m.Data) != 1 {
		t.Fatalf("got %d data ops, want 1", len(m.Data))
	}
}

func TestParseMigration_DescriptionOnly(t *testing.T) {
	input := []byte(`description = "just a description"`)
	m, err := ParseMigration(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Description != "just a description" {
		t.Errorf("description = %q, want %q", m.Description, "just a description")
	}
	if len(m.Structure) != 0 {
		t.Fatalf("got %d structure ops, want 0", len(m.Structure))
	}
	if len(m.Data) != 0 {
		t.Fatalf("got %d data ops, want 0", len(m.Data))
	}
}

func TestParseMigration_DownIrreversible(t *testing.T) {
	input := []byte(`
[[structure]]
op = "add_field"
path = "new.key"
type = "string"
default = "val"
down = "irreversible"
`)
	m, err := ParseMigration(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	op := m.Structure[0]
	if op.Down == nil {
		t.Fatal("down is nil")
	}
	if !op.Down.Irreversible {
		t.Error("expected down.Irreversible = true")
	}
}

func TestParseMigration_DownInlineTable(t *testing.T) {
	input := []byte(`
[[structure]]
op = "rename_field"
from = "old.name"
to = "new_name"
down = {op = "rename_field", from = "old.new_name", to = "name"}
`)
	m, err := ParseMigration(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	op := m.Structure[0]
	if op.Down == nil {
		t.Fatal("down is nil")
	}
	if op.Down.Irreversible {
		t.Error("expected down.Irreversible = false")
	}
	if len(op.Down.Ops) != 1 {
		t.Fatalf("got %d down ops, want 1", len(op.Down.Ops))
	}
	dop := op.Down.Ops[0]
	if dop.Type != OpRenameField {
		t.Errorf("down op type = %q, want %q", dop.Type, OpRenameField)
	}
	if dop.From != "old.new_name" {
		t.Errorf("down op from = %q, want %q", dop.From, "old.new_name")
	}
	if dop.To != "name" {
		t.Errorf("down op to = %q, want %q", dop.To, "name")
	}
}

func TestParseMigration_DownArray(t *testing.T) {
	input := []byte(`
[[structure]]
op = "move_field"
from = "a.b"
to = "c.d"
down = [
  {op = "move_field", from = "c.d", to = "a.b"},
  {op = "remove_field", path = "cleanup"},
]
`)
	m, err := ParseMigration(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	op := m.Structure[0]
	if op.Down == nil {
		t.Fatal("down is nil")
	}
	if len(op.Down.Ops) != 2 {
		t.Fatalf("got %d down ops, want 2", len(op.Down.Ops))
	}
	if op.Down.Ops[0].Type != OpMoveField {
		t.Errorf("down[0].Type = %q, want %q", op.Down.Ops[0].Type, OpMoveField)
	}
	if op.Down.Ops[1].Type != OpRemoveField {
		t.Errorf("down[1].Type = %q, want %q", op.Down.Ops[1].Type, OpRemoveField)
	}
}

func TestParseMigration_AllStructureOpTypes(t *testing.T) {
	tests := []struct {
		name  string
		toml  string
		check func(t *testing.T, op Op)
	}{
		{
			name: "add_field",
			toml: `[[structure]]
op = "add_field"
path = "x.y"
type = "integer"
default = 42`,
			check: func(t *testing.T, op Op) {
				if op.Type != OpAddField {
					t.Errorf("type = %q", op.Type)
				}
				if op.FieldType != "integer" {
					t.Errorf("FieldType = %q", op.FieldType)
				}
				if op.Default != int64(42) {
					t.Errorf("Default = %v (%T)", op.Default, op.Default)
				}
			},
		},
		{
			name: "remove_field",
			toml: `[[structure]]
op = "remove_field"
path = "old.key"`,
			check: func(t *testing.T, op Op) {
				if op.Type != OpRemoveField {
					t.Errorf("type = %q", op.Type)
				}
				if op.Path != "old.key" {
					t.Errorf("path = %q", op.Path)
				}
			},
		},
		{
			name: "rename_field",
			toml: `[[structure]]
op = "rename_field"
from = "old.name"
to = "new_name"`,
			check: func(t *testing.T, op Op) {
				if op.Type != OpRenameField {
					t.Errorf("type = %q", op.Type)
				}
				if op.From != "old.name" {
					t.Errorf("from = %q", op.From)
				}
				if op.To != "new_name" {
					t.Errorf("to = %q", op.To)
				}
			},
		},
		{
			name: "move_field",
			toml: `[[structure]]
op = "move_field"
from = "a.b"
to = "c.d"`,
			check: func(t *testing.T, op Op) {
				if op.Type != OpMoveField {
					t.Errorf("type = %q", op.Type)
				}
			},
		},
		{
			name: "add_collection",
			toml: `[[structure]]
op = "add_collection"
path = "logging"
fields = {level = "info", file = "/var/log/app.log"}`,
			check: func(t *testing.T, op Op) {
				if op.Type != OpAddCollection {
					t.Errorf("type = %q", op.Type)
				}
				if op.Fields == nil {
					t.Fatal("fields is nil")
				}
				if op.Fields["level"] != "info" {
					t.Errorf("fields[level] = %v", op.Fields["level"])
				}
			},
		},
		{
			name: "drop_collection",
			toml: `[[structure]]
op = "drop_collection"
path = "deprecated"`,
			check: func(t *testing.T, op Op) {
				if op.Type != OpDropCollection {
					t.Errorf("type = %q", op.Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := ParseMigration([]byte(tt.toml))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if len(m.Structure) != 1 {
				t.Fatalf("got %d ops, want 1", len(m.Structure))
			}
			tt.check(t, m.Structure[0])
		})
	}
}

func TestParseMigration_UnknownOpType(t *testing.T) {
	input := []byte(`
[[structure]]
op = "nonexistent_op"
path = "x"
`)
	_, err := ParseMigration(input)
	if err == nil {
		t.Fatal("expected error for unknown op type")
	}
	if !strings.Contains(err.Error(), "unknown op type") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "unknown op type")
	}
}

func TestParseMigration_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		wantErr string
	}{
		{
			name: "add_field without type",
			toml: `[[structure]]
op = "add_field"
path = "x"`,
			wantErr: "missing required field \"type\"",
		},
		{
			name: "add_field without path",
			toml: `[[structure]]
op = "add_field"
type = "string"`,
			wantErr: "missing required field \"path\"",
		},
		{
			name: "remove_field without path",
			toml: `[[structure]]
op = "remove_field"`,
			wantErr: "missing required field \"path\"",
		},
		{
			name: "rename_field without from",
			toml: `[[structure]]
op = "rename_field"
to = "new"`,
			wantErr: "missing required field \"from\"",
		},
		{
			name: "rename_field without to",
			toml: `[[structure]]
op = "rename_field"
from = "old"`,
			wantErr: "missing required field \"to\"",
		},
		{
			name: "transform without expr",
			toml: `[[data]]
op = "transform"
path = "x"`,
			wantErr: "missing required field \"expr\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMigration([]byte(tt.toml))
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseMigration_MissingOpField(t *testing.T) {
	input := []byte(`
[[structure]]
path = "x"
`)
	_, err := ParseMigration(input)
	if err == nil {
		t.Fatal("expected error for missing op field")
	}
	if !strings.Contains(err.Error(), "missing or invalid \"op\" field") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestParseMigration_InvalidDownString(t *testing.T) {
	input := []byte(`
[[structure]]
op = "remove_field"
path = "x"
down = "something_else"
`)
	_, err := ParseMigration(input)
	if err == nil {
		t.Fatal("expected error for invalid down string")
	}
	if !strings.Contains(err.Error(), "invalid down string") {
		t.Errorf("error = %q", err.Error())
	}
}
