package cel

import (
	"github.com/smm-h/stricttest/go/hygiene"
	"testing"
	"time"
)

func TestEvaluateStringOps(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("concatenation", func(t *testing.T) {
		result, err := Evaluate(`value + "-suffix"`, "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "hello-suffix" {
			t.Errorf("got %v, want %q", result, "hello-suffix")
		}
	})

	t.Run("conditional", func(t *testing.T) {
		result, err := Evaluate(`value == "yes" ? "enabled" : "disabled"`, "yes")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "enabled" {
			t.Errorf("got %v, want %q", result, "enabled")
		}
	})

	t.Run("size", func(t *testing.T) {
		result, err := Evaluate(`size(value)`, "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != int64(5) {
			t.Errorf("got %v (%T), want int64(5)", result, result)
		}
	})
}

func TestEvaluateStringExtensions(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("contains", func(t *testing.T) {
		result, err := Evaluate(`value.contains("world")`, "hello world")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != true {
			t.Errorf("got %v, want true", result)
		}
	})

	t.Run("replace", func(t *testing.T) {
		result, err := Evaluate(`value.replace("old", "new")`, "old-value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "new-value" {
			t.Errorf("got %v, want %q", result, "new-value")
		}
	})

	t.Run("split", func(t *testing.T) {
		result, err := Evaluate(`value.split(",")`, "a,b,c")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// CEL returns []string for string list results.
		parts, ok := result.([]string)
		if !ok {
			t.Fatalf("got %T, want []string", result)
		}
		if len(parts) != 3 {
			t.Errorf("got %d parts, want 3", len(parts))
		}
		if parts[0] != "a" || parts[1] != "b" || parts[2] != "c" {
			t.Errorf("got %v, want [a b c]", parts)
		}
	})

	t.Run("upperAscii", func(t *testing.T) {
		result, err := Evaluate(`value.upperAscii()`, "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "HELLO" {
			t.Errorf("got %v, want %q", result, "HELLO")
		}
	})

	t.Run("lowerAscii", func(t *testing.T) {
		result, err := Evaluate(`value.lowerAscii()`, "HELLO")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "hello" {
			t.Errorf("got %v, want %q", result, "hello")
		}
	})
}

func TestEvaluateIntegerOps(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("arithmetic", func(t *testing.T) {
		result, err := Evaluate(`value * 2 + 1`, int64(10))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != int64(21) {
			t.Errorf("got %v (%T), want int64(21)", result, result)
		}
	})

	t.Run("comparison", func(t *testing.T) {
		result, err := Evaluate(`value > 5`, int64(10))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != true {
			t.Errorf("got %v, want true", result)
		}
	})
}

func TestEvaluateFloatOps(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("multiplication", func(t *testing.T) {
		result, err := Evaluate(`value * 1.5`, float64(2.0))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != float64(3.0) {
			t.Errorf("got %v (%T), want float64(3.0)", result, result)
		}
	})

	t.Run("division", func(t *testing.T) {
		result, err := Evaluate(`value / 2.0`, float64(10.0))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != float64(5.0) {
			t.Errorf("got %v (%T), want float64(5.0)", result, result)
		}
	})
}

func TestEvaluateBooleanOps(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("negation", func(t *testing.T) {
		result, err := Evaluate(`!value`, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != false {
			t.Errorf("got %v, want false", result)
		}
	})

	t.Run("conditional", func(t *testing.T) {
		result, err := Evaluate(`value ? "on" : "off"`, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "off" {
			t.Errorf("got %v, want %q", result, "off")
		}
	})
}

func TestEvaluateListOps(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("size", func(t *testing.T) {
		result, err := Evaluate(`size(value)`, []any{"a", "b", "c"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != int64(3) {
			t.Errorf("got %v (%T), want int64(3)", result, result)
		}
	})

	t.Run("indexing", func(t *testing.T) {
		result, err := Evaluate(`value[1]`, []any{"a", "b", "c"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "b" {
			t.Errorf("got %v, want %q", result, "b")
		}
	})
}

func TestEvaluateMapOps(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("key access", func(t *testing.T) {
		result, err := Evaluate(`value.name`, map[string]any{"name": "test", "count": int64(5)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "test" {
			t.Errorf("got %v, want %q", result, "test")
		}
	})
}

func TestEvaluateTimeHandling(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("timestamp passthrough", func(t *testing.T) {
		input := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
		result, err := Evaluate(`value`, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, ok := result.(time.Time)
		if !ok {
			t.Fatalf("got %T, want time.Time", result)
		}
		if !got.Equal(input) {
			t.Errorf("got %v, want %v", got, input)
		}
	})
}

func TestEvaluateErrors(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))
	t.Run("invalid expression", func(t *testing.T) {
		_, err := Evaluate(`value +`, "hello")
		if err == nil {
			t.Fatal("expected error for invalid expression")
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		_, err := Evaluate(`value + 1`, "hello")
		if err == nil {
			t.Fatal("expected error for type mismatch")
		}
	})

	t.Run("undefined variable", func(t *testing.T) {
		_, err := Evaluate(`unknown_var + 1`, int64(1))
		if err == nil {
			t.Fatal("expected error for undefined variable")
		}
	})
}
