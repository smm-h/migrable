package ops

import (
	"fmt"

	tomledit "github.com/smm-h/go-toml-edit"
)

func ExecRaw(doc *tomledit.DocumentNode, op Op) error {
	contentDoc, err := tomledit.Parse([]byte(op.Content))
	if err != nil {
		return fmt.Errorf("raw: failed to parse content: %w", err)
	}

	var walkErr error
	contentDoc.Walk(func(path string, node tomledit.Node) error {
		targetPath := path
		if op.Path != "" {
			targetPath = op.Path + "." + path
		}
		val := scalarValue(node)
		if err := doc.SetCreate(targetPath, val); err != nil {
			walkErr = fmt.Errorf("raw: failed to set %q: %w", targetPath, err)
			return walkErr
		}
		return nil
	}, tomledit.WalkLeaves)

	return walkErr
}
