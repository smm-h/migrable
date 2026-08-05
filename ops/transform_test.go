package ops

import (
	"github.com/smm-h/stricttest/go/hygiene"
	"strings"
	"testing"
)

func TestExecTransform(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("transform string value", func(t *testing.T) {
		doc := mustParse(t, `name = "hello"`)
		err := ExecTransform(doc, Op{Path: "name", Expr: `value + "-world"`})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "name", "hello-world")
	})

	t.Run("transform integer value", func(t *testing.T) {
		doc := mustParse(t, `port = 8080`)
		err := ExecTransform(doc, Op{Path: "port", Expr: `value * 2`})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "port", int64(16160))
	})

	t.Run("transform with conditional expression", func(t *testing.T) {
		doc := mustParse(t, `level = "debug"`)
		err := ExecTransform(doc, Op{
			Path: "level",
			Expr: `value == "debug" ? "info" : value`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "level", "info")
	})

	t.Run("error if path missing", func(t *testing.T) {
		doc := mustParse(t, `key = "val"`)
		err := ExecTransform(doc, Op{Path: "nonexistent", Expr: `value + "x"`})
		if err == nil {
			t.Fatal("expected error for missing path")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "not found")
		}
	})

	t.Run("error if expression fails", func(t *testing.T) {
		doc := mustParse(t, `name = "hello"`)
		err := ExecTransform(doc, Op{Path: "name", Expr: `value / 2`})
		if err == nil {
			t.Fatal("expected error for type mismatch in expression")
		}
	})

	t.Run("transform nested path", func(t *testing.T) {
		doc := mustParse(t, "[server]\nhost = \"localhost\"\n")
		err := ExecTransform(doc, Op{
			Path: "server.host",
			Expr: `value.upperAscii()`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "server.host", "LOCALHOST")
	})
}
