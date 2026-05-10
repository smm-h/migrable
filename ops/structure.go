package ops

import (
	"fmt"

	tomledit "github.com/smm-h/go-toml-edit"
)

func ExecAddField(doc *tomledit.DocumentNode, op Op) error {
	if doc.Get(op.Path) != nil {
		return nil
	}
	if op.Default == nil {
		return nil
	}
	return doc.SetCreate(op.Path, op.Default)
}

func ExecRemoveField(doc *tomledit.DocumentNode, op Op) error {
	return doc.Delete(op.Path)
}

func ExecRenameField(doc *tomledit.DocumentNode, op Op) error {
	if doc.Get(op.From) == nil {
		return nil
	}
	return doc.Rename(op.From, op.To)
}

func ExecMoveField(doc *tomledit.DocumentNode, op Op) error {
	srcNode := doc.Get(op.From)
	if srcNode == nil {
		return nil
	}
	if doc.Get(op.To) != nil {
		return fmt.Errorf("move_field: target path %q already exists", op.To)
	}
	val := srcNode.Value()
	if err := doc.SetCreate(op.To, val); err != nil {
		return fmt.Errorf("move_field: failed to set %q: %w", op.To, err)
	}
	if err := doc.Delete(op.From); err != nil {
		return fmt.Errorf("move_field: failed to delete %q: %w", op.From, err)
	}
	return nil
}

func ExecAddCollection(doc *tomledit.DocumentNode, op Op) error {
	if doc.Get(op.Path) != nil {
		return fmt.Errorf("add_collection: path %q already exists", op.Path)
	}
	if err := doc.NewTable(op.Path); err != nil {
		return fmt.Errorf("add_collection: %w", err)
	}
	for key, val := range op.Fields {
		fieldPath := op.Path + "." + key
		if err := doc.SetCreate(fieldPath, val); err != nil {
			return fmt.Errorf("add_collection: failed to set field %q: %w", fieldPath, err)
		}
	}
	return nil
}

func ExecDropCollection(doc *tomledit.DocumentNode, op Op) error {
	return doc.Delete(op.Path)
}
