package ops

import (
	"strings"
	"testing"
)

func TestExecRaw(t *testing.T) {
	t.Run("insert key-value pairs at root", func(t *testing.T) {
		doc := mustParse(t, "existing = true\n")
		err := ExecRaw(doc, Op{
			Content: "new_key = \"hello\"\ncount = 42\n",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "existing", true)
		assertNodeValue(t, doc, "new_key", "hello")
		assertNodeValue(t, doc, "count", int64(42))
	})

	t.Run("insert at a specific path", func(t *testing.T) {
		doc := mustParse(t, "[server]\nhost = \"localhost\"\n")
		err := ExecRaw(doc, Op{
			Path:    "server",
			Content: "port = 8080\ndebug = false\n",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "server.host", "localhost")
		assertNodeValue(t, doc, "server.port", int64(8080))
		assertNodeValue(t, doc, "server.debug", false)
	})

	t.Run("overwrite existing keys", func(t *testing.T) {
		doc := mustParse(t, "name = \"old\"\ncount = 1\n")
		err := ExecRaw(doc, Op{
			Content: "name = \"new\"\n",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "name", "new")
		assertNodeValue(t, doc, "count", int64(1))
	})

	t.Run("multiple key-value pairs", func(t *testing.T) {
		doc := mustParse(t, "")
		err := ExecRaw(doc, Op{
			Content: "a = 1\nb = 2\nc = 3\n",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "a", int64(1))
		assertNodeValue(t, doc, "b", int64(2))
		assertNodeValue(t, doc, "c", int64(3))
	})

	t.Run("nested content", func(t *testing.T) {
		doc := mustParse(t, "")
		err := ExecRaw(doc, Op{
			Content: "[database]\nhost = \"db.local\"\nport = 5432\n",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "database.host", "db.local")
		assertNodeValue(t, doc, "database.port", int64(5432))
	})

	t.Run("nested content at path", func(t *testing.T) {
		doc := mustParse(t, "[app]\nname = \"myapp\"\n")
		err := ExecRaw(doc, Op{
			Path:    "app",
			Content: "[db]\nhost = \"localhost\"\n",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "app.name", "myapp")
		assertNodeValue(t, doc, "app.db.host", "localhost")
	})

	t.Run("error on invalid TOML content", func(t *testing.T) {
		doc := mustParse(t, "")
		err := ExecRaw(doc, Op{
			Content: "invalid = [unterminated",
		})
		if err == nil {
			t.Fatal("expected error for invalid TOML")
		}
		if !strings.Contains(err.Error(), "parse") {
			t.Errorf("error = %q, want it to mention parsing", err.Error())
		}
	})
}
