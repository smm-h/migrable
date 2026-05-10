package ops

import (
	"fmt"

	celeval "github.com/smm-h/migrable/internal/cel"

	tomledit "github.com/smm-h/go-toml-edit"
)

func ExecTransform(doc *tomledit.DocumentNode, op Op) error {
	node := doc.Get(op.Path)
	if node == nil {
		return fmt.Errorf("transform: path %q not found", op.Path)
	}

	value := scalarValue(node)

	result, err := celeval.Evaluate(op.Expr, value)
	if err != nil {
		return fmt.Errorf("transform: %w", err)
	}

	return doc.Set(op.Path, result)
}
