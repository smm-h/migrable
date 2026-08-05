package ops

import (
	"github.com/smm-h/stricttest/go/hygiene"
	"strings"
	"testing"

	tomledit "github.com/smm-h/go-toml-edit"
)

func mustParse(t *testing.T, src string) *tomledit.DocumentNode {
	t.Helper()
	doc, err := tomledit.Parse([]byte(src))
	if err != nil {
		t.Fatalf("failed to parse test TOML: %v", err)
	}
	return doc
}

func assertNodeValue(t *testing.T, doc *tomledit.DocumentNode, path string, want any) {
	t.Helper()
	node := doc.Get(path)
	if node == nil {
		t.Errorf("expected node at %q, got nil", path)
		return
	}
	got := node.Value()
	if got != want {
		t.Errorf("node at %q = %v (%T), want %v (%T)", path, got, got, want, want)
	}
}

func assertNodeNil(t *testing.T, doc *tomledit.DocumentNode, path string) {
	t.Helper()
	node := doc.Get(path)
	if node != nil {
		t.Errorf("expected nil node at %q, got %v", path, node.Value())
	}
}

func TestExecAddField(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("creates field with default", func(t *testing.T) {
		doc := mustParse(t, "existing = true\n")
		err := ExecAddField(doc, Op{Path: "new_key", Default: "hello"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "new_key", "hello")
	})

	t.Run("no-op when field exists", func(t *testing.T) {
		doc := mustParse(t, "name = \"original\"\n")
		err := ExecAddField(doc, Op{Path: "name", Default: "replaced"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "name", "original")
	})

	t.Run("creates intermediate tables", func(t *testing.T) {
		doc := mustParse(t, "")
		err := ExecAddField(doc, Op{Path: "a.b.c", Default: int64(99)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "a.b.c", int64(99))
	})

	t.Run("skips when no default", func(t *testing.T) {
		doc := mustParse(t, "")
		err := ExecAddField(doc, Op{Path: "optional_field", Default: nil})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeNil(t, doc, "optional_field")
	})
}

func TestExecRemoveField(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("removes existing field", func(t *testing.T) {
		doc := mustParse(t, "keep = 1\nremove_me = 2\n")
		err := ExecRemoveField(doc, Op{Path: "remove_me"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeNil(t, doc, "remove_me")
		assertNodeValue(t, doc, "keep", int64(1))
	})

	t.Run("no-op when missing", func(t *testing.T) {
		doc := mustParse(t, "keep = 1\n")
		err := ExecRemoveField(doc, Op{Path: "nonexistent"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "keep", int64(1))
	})
}

func TestExecRenameField(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("renames key", func(t *testing.T) {
		doc := mustParse(t, "[server]\nhost = \"localhost\"\nport = 8080\n")
		err := ExecRenameField(doc, Op{From: "server.host", To: "hostname"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "server.hostname", "localhost")
		assertNodeNil(t, doc, "server.host")
	})

	t.Run("error if target exists", func(t *testing.T) {
		doc := mustParse(t, "[server]\nhost = \"localhost\"\nport = 8080\n")
		err := ExecRenameField(doc, Op{From: "server.host", To: "port"})
		if err == nil {
			t.Fatal("expected error when target exists")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "already exists")
		}
	})

	t.Run("no-op if source missing", func(t *testing.T) {
		doc := mustParse(t, "[server]\nport = 8080\n")
		err := ExecRenameField(doc, Op{From: "server.host", To: "hostname"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestExecMoveField(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("moves value between tables", func(t *testing.T) {
		doc := mustParse(t, "[source]\nkey = \"value\"\n[dest]\nother = 1\n")
		err := ExecMoveField(doc, Op{From: "source.key", To: "dest.key"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeNil(t, doc, "source.key")
		assertNodeValue(t, doc, "dest.key", "value")
	})

	t.Run("error if target exists", func(t *testing.T) {
		doc := mustParse(t, "[source]\nkey = \"value\"\n[dest]\nkey = \"existing\"\n")
		err := ExecMoveField(doc, Op{From: "source.key", To: "dest.key"})
		if err == nil {
			t.Fatal("expected error when target exists")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("no-op if source missing", func(t *testing.T) {
		doc := mustParse(t, "[dest]\nother = 1\n")
		err := ExecMoveField(doc, Op{From: "source.key", To: "dest.key"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("value preserved", func(t *testing.T) {
		doc := mustParse(t, "old_place = 42\n")
		err := ExecMoveField(doc, Op{From: "old_place", To: "new_place"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeNil(t, doc, "old_place")
		assertNodeValue(t, doc, "new_place", int64(42))
	})
}

func TestExecAddCollection(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("creates new table with fields", func(t *testing.T) {
		doc := mustParse(t, "existing = true\n")
		err := ExecAddCollection(doc, Op{
			Path:   "logging",
			Fields: map[string]any{"level": "info", "enabled": true},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "logging.level", "info")
		assertNodeValue(t, doc, "logging.enabled", true)
	})

	t.Run("error if table exists", func(t *testing.T) {
		doc := mustParse(t, "[logging]\nlevel = \"debug\"\n")
		err := ExecAddCollection(doc, Op{
			Path:   "logging",
			Fields: map[string]any{"level": "info"},
		})
		if err == nil {
			t.Fatal("expected error when table exists")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("creates without fields", func(t *testing.T) {
		doc := mustParse(t, "")
		err := ExecAddCollection(doc, Op{Path: "empty_section"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		node := doc.Get("empty_section")
		if node == nil {
			t.Error("expected table to exist")
		}
	})
}

func TestExecDropCollection(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("removes existing table", func(t *testing.T) {
		doc := mustParse(t, "[keep]\na = 1\n[remove]\nb = 2\n")
		err := ExecDropCollection(doc, Op{Path: "remove"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeNil(t, doc, "remove")
		assertNodeValue(t, doc, "keep.a", int64(1))
	})

	t.Run("no-op when missing", func(t *testing.T) {
		doc := mustParse(t, "[keep]\na = 1\n")
		err := ExecDropCollection(doc, Op{Path: "nonexistent"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "keep.a", int64(1))
	})
}

func TestExecuteDispatch(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("dispatches structure ops", func(t *testing.T) {
		doc := mustParse(t, "")
		err := Execute(doc, Op{Type: OpAddField, Path: "key", Default: "val"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "key", "val")
	})

	t.Run("dispatches transform op", func(t *testing.T) {
		doc := mustParse(t, "key = \"hello\"\n")
		err := Execute(doc, Op{Type: OpTransform, Path: "key", Expr: `value + "!"`})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "key", "hello!")
	})

	t.Run("dispatches raw op", func(t *testing.T) {
		doc := mustParse(t, "")
		err := Execute(doc, Op{Type: OpRaw, Content: "new_key = 42\n"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "new_key", int64(42))
	})

	t.Run("returns error for unknown op", func(t *testing.T) {
		doc := mustParse(t, "")
		err := Execute(doc, Op{Type: "fake_op"})
		if err == nil {
			t.Fatal("expected error for unknown op type")
		}
		if !strings.Contains(err.Error(), "unknown op type") {
			t.Errorf("error = %q", err.Error())
		}
	})
}
