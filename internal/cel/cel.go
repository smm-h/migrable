// Package cel integrates CEL expression evaluation for transform ops.
package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Evaluate compiles and evaluates a CEL expression with the given value bound
// as a variable named "value". The result is converted back to a native Go type.
func Evaluate(expr string, value any) (any, error) {
	env, err := cel.NewEnv(
		cel.Variable("value", cel.DynType),
		ext.Strings(),
	)
	if err != nil {
		return nil, fmt.Errorf("cel: failed to create environment: %w", err)
	}

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("cel: compile error: %w", issues.Err())
	}

	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("cel: program creation error: %w", err)
	}

	out, _, err := prg.Eval(map[string]any{"value": value})
	if err != nil {
		return nil, fmt.Errorf("cel: evaluation error: %w", err)
	}

	result := out.Value()

	// CEL returns timestamppb.Timestamp for time values; convert back to time.Time.
	if ts, ok := result.(*timestamppb.Timestamp); ok {
		return ts.AsTime(), nil
	}

	return result, nil
}
