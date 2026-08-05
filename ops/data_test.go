package ops

import (
	"github.com/smm-h/stricttest/go/hygiene"
	"strings"
	"testing"
)

func TestExecSetValue(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("overwrites existing value", func(t *testing.T) {
		doc := mustParse(t, "key = \"old\"\n")
		err := ExecSetValue(doc, Op{Path: "key", Value: "new"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "key", "new")
	})

	t.Run("creates new key with intermediates", func(t *testing.T) {
		doc := mustParse(t, "")
		err := ExecSetValue(doc, Op{Path: "a.b.c", Value: int64(42)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "a.b.c", int64(42))
	})

	t.Run("creates key in existing table", func(t *testing.T) {
		doc := mustParse(t, "[server]\nhost = \"localhost\"\n")
		err := ExecSetValue(doc, Op{Path: "server.port", Value: int64(8080)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "server.port", int64(8080))
		assertNodeValue(t, doc, "server.host", "localhost")
	})
}

func TestExecSetValueWhere(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	const servers = `
[[servers]]
name = "web"
host = "web.example.com"
port = 8080

[[servers]]
name = "db"
host = "db.example.com"
port = 5432

[[servers]]
name = "cache"
host = "cache.example.com"
port = 6379
`

	t.Run("matches and sets on matching items", func(t *testing.T) {
		doc := mustParse(t, servers)
		err := ExecSetValueWhere(doc, Op{
			Path:      "servers",
			Where:     map[string]any{"name": "db"},
			MatchMode: "subset",
			Set:       map[string]any{"port": int64(5433)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "servers[1].port", int64(5433))
		assertNodeValue(t, doc, "servers[1].name", "db")
	})

	t.Run("leaves non-matching items alone", func(t *testing.T) {
		doc := mustParse(t, servers)
		err := ExecSetValueWhere(doc, Op{
			Path:      "servers",
			Where:     map[string]any{"name": "db"},
			MatchMode: "subset",
			Set:       map[string]any{"port": int64(5433)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "servers[0].port", int64(8080))
		assertNodeValue(t, doc, "servers[2].port", int64(6379))
	})

	t.Run("error if path not found", func(t *testing.T) {
		doc := mustParse(t, "key = 1\n")
		err := ExecSetValueWhere(doc, Op{
			Path:      "nonexistent",
			Where:     map[string]any{"name": "a"},
			MatchMode: "subset",
			Set:       map[string]any{"x": "y"},
		})
		if err == nil {
			t.Fatal("expected error for missing path")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("error if not an array", func(t *testing.T) {
		doc := mustParse(t, "[server]\nname = \"web\"\n")
		err := ExecSetValueWhere(doc, Op{
			Path:      "server",
			Where:     map[string]any{"name": "web"},
			MatchMode: "subset",
			Set:       map[string]any{"port": int64(80)},
		})
		if err == nil {
			t.Fatal("expected error for non-array path")
		}
		if !strings.Contains(err.Error(), "not an array") {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("adds new field to matching items", func(t *testing.T) {
		doc := mustParse(t, servers)
		err := ExecSetValueWhere(doc, Op{
			Path:      "servers",
			Where:     map[string]any{"name": "web"},
			MatchMode: "subset",
			Set:       map[string]any{"ssl": true},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "servers[0].ssl", true)
		assertNodeNil(t, doc, "servers[1].ssl")
	})
}

func TestExecRemoveWhere(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	const servers = `
[[servers]]
name = "web"
port = 8080

[[servers]]
name = "db"
port = 5432

[[servers]]
name = "cache"
port = 6379
`

	t.Run("removes matching items", func(t *testing.T) {
		doc := mustParse(t, servers)
		err := ExecRemoveWhere(doc, Op{
			Path:      "servers",
			Where:     map[string]any{"name": "db"},
			MatchMode: "subset",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Len("servers") != 2 {
			t.Errorf("expected 2 servers, got %d", doc.Len("servers"))
		}
		assertNodeValue(t, doc, "servers[0].name", "web")
		assertNodeValue(t, doc, "servers[1].name", "cache")
	})

	t.Run("leaves non-matching items", func(t *testing.T) {
		doc := mustParse(t, servers)
		err := ExecRemoveWhere(doc, Op{
			Path:      "servers",
			Where:     map[string]any{"name": "nonexistent"},
			MatchMode: "subset",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Len("servers") != 3 {
			t.Errorf("expected 3 servers, got %d", doc.Len("servers"))
		}
	})

	t.Run("error if path not found", func(t *testing.T) {
		doc := mustParse(t, "key = 1\n")
		err := ExecRemoveWhere(doc, Op{
			Path:      "nonexistent",
			Where:     map[string]any{"name": "a"},
			MatchMode: "subset",
		})
		if err == nil {
			t.Fatal("expected error for missing path")
		}
	})

	t.Run("error if not an array", func(t *testing.T) {
		doc := mustParse(t, "[server]\nname = \"web\"\n")
		err := ExecRemoveWhere(doc, Op{
			Path:      "server",
			Where:     map[string]any{"name": "web"},
			MatchMode: "subset",
		})
		if err == nil {
			t.Fatal("expected error for non-array path")
		}
	})

	t.Run("removes multiple matching items", func(t *testing.T) {
		doc := mustParse(t, servers)
		err := ExecRemoveWhere(doc, Op{
			Path:      "servers",
			MatchMode: "all",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Len("servers") != -1 {
			t.Errorf("expected empty array (len -1), got %d", doc.Len("servers"))
		}
	})
}

func TestExecAppend(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("adds to existing array", func(t *testing.T) {
		doc := mustParse(t, "tags = [\"web\", \"prod\"]\n")
		err := ExecAppend(doc, Op{Path: "tags", Value: "v2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "tags[2]", "v2")
		assertNodeValue(t, doc, "tags[0]", "web")
		assertNodeValue(t, doc, "tags[1]", "prod")
	})

	t.Run("error if path missing", func(t *testing.T) {
		doc := mustParse(t, "")
		err := ExecAppend(doc, Op{Path: "tags", Value: "v1"})
		if err == nil {
			t.Fatal("expected error for missing path")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("error if not an array", func(t *testing.T) {
		doc := mustParse(t, "tags = \"not-an-array\"\n")
		err := ExecAppend(doc, Op{Path: "tags", Value: "v1"})
		if err == nil {
			t.Fatal("expected error for non-array")
		}
		if !strings.Contains(err.Error(), "not an array") {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("appends integer to integer array", func(t *testing.T) {
		doc := mustParse(t, "nums = [1, 2, 3]\n")
		err := ExecAppend(doc, Op{Path: "nums", Value: int64(4)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "nums[3]", int64(4))
	})
}

func TestExecMergeDefaults(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("merges missing keys", func(t *testing.T) {
		doc := mustParse(t, "[server]\nhost = \"localhost\"\n")
		err := ExecMergeDefaults(doc, Op{
			Path:  "server",
			Value: map[string]any{"host": "default.com", "port": int64(8080), "debug": false},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "server.host", "localhost")
		assertNodeValue(t, doc, "server.port", int64(8080))
		assertNodeValue(t, doc, "server.debug", false)
	})

	t.Run("does not overwrite existing", func(t *testing.T) {
		doc := mustParse(t, "[server]\nhost = \"custom\"\nport = 9090\n")
		err := ExecMergeDefaults(doc, Op{
			Path:  "server",
			Value: map[string]any{"host": "default.com", "port": int64(8080)},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "server.host", "custom")
		assertNodeValue(t, doc, "server.port", int64(9090))
	})

	t.Run("handles nested tables", func(t *testing.T) {
		doc := mustParse(t, "[server]\nhost = \"localhost\"\n[server.tls]\ncert = \"my.crt\"\n")
		err := ExecMergeDefaults(doc, Op{
			Path: "server",
			Value: map[string]any{
				"port": int64(443),
				"tls":  map[string]any{"cert": "default.crt", "key": "default.key"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertNodeValue(t, doc, "server.port", int64(443))
		assertNodeValue(t, doc, "server.tls.cert", "my.crt")
		assertNodeValue(t, doc, "server.tls.key", "default.key")
	})

	t.Run("error if value is not a map", func(t *testing.T) {
		doc := mustParse(t, "")
		err := ExecMergeDefaults(doc, Op{
			Path:  "server",
			Value: "not-a-map",
		})
		if err == nil {
			t.Fatal("expected error for non-map value")
		}
	})
}

func TestExecMergeDefaultsByKey(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	const plugins = `
[[plugins]]
name = "auth"
enabled = true

[[plugins]]
name = "logging"
level = "info"
`

	t.Run("matches by key and adds missing attrs", func(t *testing.T) {
		doc := mustParse(t, plugins)
		err := ExecMergeDefaultsByKey(doc, Op{
			Path:       "plugins",
			MatchField: "name",
			Defaults: []map[string]any{
				{"name": "auth", "timeout": int64(30), "enabled": false},
				{"name": "logging", "level": "debug", "format": "json"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// auth: enabled stays true (not overwritten), timeout added
		assertNodeValue(t, doc, "plugins[0].enabled", true)
		assertNodeValue(t, doc, "plugins[0].timeout", int64(30))
		// logging: level stays "info" (not overwritten), format added
		assertNodeValue(t, doc, "plugins[1].level", "info")
		assertNodeValue(t, doc, "plugins[1].format", "json")
	})

	t.Run("does not add unmatched defaults", func(t *testing.T) {
		doc := mustParse(t, plugins)
		err := ExecMergeDefaultsByKey(doc, Op{
			Path:       "plugins",
			MatchField: "name",
			Defaults: []map[string]any{
				{"name": "metrics", "endpoint": "/metrics"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Len("plugins") != 2 {
			t.Errorf("expected 2 plugins, got %d", doc.Len("plugins"))
		}
	})

	t.Run("handles empty array", func(t *testing.T) {
		doc := mustParse(t, "plugins = []\n")
		err := ExecMergeDefaultsByKey(doc, Op{
			Path:       "plugins",
			MatchField: "name",
			Defaults: []map[string]any{
				{"name": "auth", "timeout": int64(30)},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if doc.Len("plugins") != 0 {
			t.Errorf("expected 0 plugins, got %d", doc.Len("plugins"))
		}
	})

	t.Run("error if not an array", func(t *testing.T) {
		doc := mustParse(t, "[server]\nname = \"web\"\n")
		err := ExecMergeDefaultsByKey(doc, Op{
			Path:       "server",
			MatchField: "name",
			Defaults: []map[string]any{
				{"name": "web", "port": int64(80)},
			},
		})
		if err == nil {
			t.Fatal("expected error for non-array path")
		}
	})
}
