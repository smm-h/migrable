package ops

import (
	"fmt"

	tomledit "github.com/smm-h/go-toml-edit"
)

func ExecSetValue(doc *tomledit.DocumentNode, op Op) error {
	return doc.SetCreate(op.Path, op.Value)
}

func ExecSetValueWhere(doc *tomledit.DocumentNode, op Op) error {
	node := doc.Get(op.Path)
	if node == nil {
		return fmt.Errorf("set_value_where: path %q not found", op.Path)
	}

	arrayLen := doc.Len(op.Path)
	if arrayLen < 0 {
		return fmt.Errorf("set_value_where: path %q is not an array", op.Path)
	}

	mode := MatchMode(op.MatchMode)
	var where any = op.Where

	for i, item := range doc.Items(op.Path) {
		itemMap := extractMap(item)
		if itemMap == nil {
			continue
		}
		if !Matches(itemMap, where, mode, i, arrayLen) {
			continue
		}
		for field, val := range op.Set {
			fieldPath := fmt.Sprintf("%s[%d].%s", op.Path, i, field)
			if err := doc.SetCreate(fieldPath, val); err != nil {
				return fmt.Errorf("set_value_where: failed to set %q: %w", fieldPath, err)
			}
		}
	}
	return nil
}

func ExecRemoveWhere(doc *tomledit.DocumentNode, op Op) error {
	node := doc.Get(op.Path)
	if node == nil {
		return fmt.Errorf("remove_where: path %q not found", op.Path)
	}

	arrayLen := doc.Len(op.Path)
	if arrayLen < 0 {
		return fmt.Errorf("remove_where: path %q is not an array", op.Path)
	}

	mode := MatchMode(op.MatchMode)
	var where any = op.Where

	var matchingIndices []int
	for i, item := range doc.Items(op.Path) {
		itemMap := extractMap(item)
		if itemMap == nil {
			continue
		}
		if Matches(itemMap, where, mode, i, arrayLen) {
			matchingIndices = append(matchingIndices, i)
		}
	}

	for j := len(matchingIndices) - 1; j >= 0; j-- {
		idx := matchingIndices[j]
		deletePath := fmt.Sprintf("%s[%d]", op.Path, idx)
		if err := doc.Delete(deletePath); err != nil {
			return fmt.Errorf("remove_where: failed to delete %q: %w", deletePath, err)
		}
	}
	return nil
}

func ExecAppend(doc *tomledit.DocumentNode, op Op) error {
	node := doc.Get(op.Path)
	if node == nil {
		return fmt.Errorf("append: path %q not found", op.Path)
	}

	children, ok := node.Value().([]tomledit.Node)
	if !ok {
		return fmt.Errorf("append: path %q is not an array", op.Path)
	}

	vals := make([]any, len(children))
	for i, elem := range children {
		vals[i] = scalarValue(elem)
	}
	vals = append(vals, op.Value)
	return doc.Set(op.Path, vals)
}

func ExecMergeDefaults(doc *tomledit.DocumentNode, op Op) error {
	defaults, ok := op.Value.(map[string]any)
	if !ok {
		return fmt.Errorf("merge_defaults: value must be a map, got %T", op.Value)
	}
	return doc.MergeDefaults(op.Path, defaults)
}

func ExecMergeDefaultsByKey(doc *tomledit.DocumentNode, op Op) error {
	arrayLen := doc.Len(op.Path)
	if arrayLen < 0 {
		return fmt.Errorf("merge_defaults_by_key: path %q is not an array", op.Path)
	}

	for i := range doc.Items(op.Path) {
		itemKeyNode := doc.Get(fmt.Sprintf("%s[%d].%s", op.Path, i, op.MatchField))
		if itemKeyNode == nil {
			continue
		}
		itemKeyVal := scalarValue(itemKeyNode)

		for _, def := range op.Defaults {
			defKeyVal, exists := def[op.MatchField]
			if !exists {
				continue
			}
			if fmt.Sprintf("%v", itemKeyVal) != fmt.Sprintf("%v", defKeyVal) {
				continue
			}
			for field, val := range def {
				if field == op.MatchField {
					continue
				}
				fieldPath := fmt.Sprintf("%s[%d].%s", op.Path, i, field)
				if doc.Get(fieldPath) != nil {
					continue
				}
				if err := doc.SetCreate(fieldPath, val); err != nil {
					return fmt.Errorf("merge_defaults_by_key: failed to set %q: %w", fieldPath, err)
				}
			}
			break
		}
	}
	return nil
}

// extractMap converts a table-like Node (ArrayTableNode, InlineTableNode,
// TableNode) to a flat map[string]any by reading its KV children.
func extractMap(node tomledit.Node) map[string]any {
	var children []tomledit.Node

	switch n := node.(type) {
	case *tomledit.ArrayTableNode:
		children = n.Children
	case *tomledit.InlineTableNode:
		children = n.Children
	case *tomledit.TableNode:
		children = n.Children
	default:
		return nil
	}

	result := make(map[string]any)
	for _, child := range children {
		kv, ok := child.(*tomledit.KeyValueNode)
		if !ok {
			continue
		}
		if len(kv.Key.Parts) == 1 {
			result[kv.Key.Parts[0]] = scalarValue(kv.Val)
		}
	}
	return result
}

// scalarValue extracts a Go value from a Node. For scalar nodes it returns the
// typed value; for containers it returns the raw Node value.
func scalarValue(node tomledit.Node) any {
	switch n := node.(type) {
	case *tomledit.StringNode:
		return n.Val
	case *tomledit.IntegerNode:
		return n.Val
	case *tomledit.FloatNode:
		return n.Val
	case *tomledit.BooleanNode:
		return n.Val
	default:
		return n.Value()
	}
}
