package conformance

import (
	"bytes"
	"fmt"
	"github.com/smm-h/stricttest/go/hygiene"
	"os"
	"path/filepath"
	"testing"

	tomledit "github.com/smm-h/go-toml-edit"
	"github.com/smm-h/migrable/ops"
)

type conformanceTest struct {
	Description   string `toml:"description"`
	Input         string `toml:"input"`
	Expected      string `toml:"expected"`
	ExpectedBytes string `toml:"expected_bytes"`
	TestRollback  bool   `toml:"test_rollback"`
}

func TestConformance(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	files, err := filepath.Glob("*.toml")
	if err != nil {
		t.Fatalf("failed to glob test files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no conformance test files found")
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("failed to read %s: %v", file, err)
			}

			var ct conformanceTest
			if err := tomledit.Unmarshal(data, &ct); err != nil {
				t.Fatalf("failed to unmarshal test file: %v", err)
			}

			migration, err := extractMigration(data)
			if err != nil {
				t.Fatalf("failed to extract migration: %v", err)
			}

			doc, err := tomledit.Parse([]byte(ct.Input))
			if err != nil {
				t.Fatalf("failed to parse input: %v", err)
			}

			applyOps(t, doc, migration.Structure)
			applyOps(t, doc, migration.Data)

			expectedDoc, err := tomledit.Parse([]byte(ct.Expected))
			if err != nil {
				t.Fatalf("failed to parse expected: %v", err)
			}

			changes := tomledit.Diff(expectedDoc, doc)
			if len(changes) > 0 {
				t.Errorf("data equivalence failed (%d differences):", len(changes))
				for _, c := range changes {
					t.Errorf("  %s %s: old=%v new=%v", c.Kind, c.Path, c.OldValue, c.NewValue)
				}
				t.Errorf("got:\n%s", doc.Bytes())
				t.Errorf("expected:\n%s", ct.Expected)
			}

			if ct.ExpectedBytes != "" {
				got := doc.Bytes()
				if !bytes.Equal(got, []byte(ct.ExpectedBytes)) {
					t.Errorf("byte comparison failed:\ngot:\n%s\nexpected:\n%s", got, ct.ExpectedBytes)
				}
			}

			if ct.TestRollback {
				testRollback(t, ct, migration)
			}
		})
	}
}

// extractMigration parses the test file AST to find [[migration.structure]]
// and [[migration.data]] array table nodes plus their sub-tables (e.g.
// [migration.data.where]), re-serializes them as top-level [[structure]] /
// [[data]] sections with corresponding sub-tables, then calls ParseMigration.
func extractMigration(data []byte) (*ops.Migration, error) {
	doc, err := tomledit.Parse(data)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	for _, child := range doc.Children {
		switch n := child.(type) {
		case *tomledit.ArrayTableNode:
			// [[migration.structure]] or [[migration.data]]
			if len(n.KeyPath) != 2 || n.KeyPath[0] != "migration" {
				continue
			}
			section := n.KeyPath[1]
			fmt.Fprintf(&buf, "[[%s]]\n", section)
			for _, ch := range n.Children {
				buf.Write(ch.Raw())
			}

		case *tomledit.TableNode:
			// [migration.data.where] or [migration.data.value] etc.
			if len(n.KeyPath) < 3 || n.KeyPath[0] != "migration" {
				continue
			}
			section := n.KeyPath[1]
			rest := n.KeyPath[2:]
			fmt.Fprintf(&buf, "[%s", section)
			for _, part := range rest {
				fmt.Fprintf(&buf, ".%s", part)
			}
			buf.WriteString("]\n")
			for _, ch := range n.Children {
				buf.Write(ch.Raw())
			}
		}
	}

	return ops.ParseMigration(buf.Bytes())
}

func applyOps(t *testing.T, doc *tomledit.DocumentNode, opList []ops.Op) {
	t.Helper()
	for i, op := range opList {
		if err := ops.Execute(doc, op); err != nil {
			t.Fatalf("op %d (%s) failed: %v", i, op.Type, err)
		}
	}
}

func testRollback(t *testing.T, ct conformanceTest, migration *ops.Migration) {
	t.Helper()

	doc, err := tomledit.Parse([]byte(ct.Input))
	if err != nil {
		t.Fatalf("rollback: failed to parse input: %v", err)
	}

	applyOps(t, doc, migration.Structure)
	applyOps(t, doc, migration.Data)

	var downOps []ops.Op
	for i := len(migration.Data) - 1; i >= 0; i-- {
		op := migration.Data[i]
		if op.Down != nil && !op.Down.Irreversible {
			downOps = append(downOps, op.Down.Ops...)
		}
	}
	for i := len(migration.Structure) - 1; i >= 0; i-- {
		op := migration.Structure[i]
		if op.Down != nil && !op.Down.Irreversible {
			downOps = append(downOps, op.Down.Ops...)
		}
	}

	applyOps(t, doc, downOps)

	inputDoc, err := tomledit.Parse([]byte(ct.Input))
	if err != nil {
		t.Fatalf("rollback: failed to parse original input: %v", err)
	}

	changes := tomledit.Diff(inputDoc, doc)
	if len(changes) > 0 {
		t.Errorf("rollback failed (%d differences):", len(changes))
		for _, c := range changes {
			t.Errorf("  %s %s: old=%v new=%v", c.Kind, c.Path, c.OldValue, c.NewValue)
		}
		t.Errorf("after rollback:\n%s", doc.Bytes())
		t.Errorf("expected (original input):\n%s", ct.Input)
	}
}
