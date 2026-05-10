package ops

import (
	"fmt"

	tomledit "github.com/smm-h/go-toml-edit"
)

func Execute(doc *tomledit.DocumentNode, op Op) error {
	switch op.Type {
	case OpAddField:
		return ExecAddField(doc, op)
	case OpRemoveField:
		return ExecRemoveField(doc, op)
	case OpRenameField:
		return ExecRenameField(doc, op)
	case OpMoveField:
		return ExecMoveField(doc, op)
	case OpAddCollection:
		return ExecAddCollection(doc, op)
	case OpDropCollection:
		return ExecDropCollection(doc, op)
	case OpSetValue, OpSetValueWhere, OpRemoveWhere, OpAppend,
		OpTransform, OpMergeDefaults, OpMergeDefaultsByKey:
		return fmt.Errorf("op %q: data ops not yet implemented", op.Type)
	case OpRaw:
		return fmt.Errorf("op %q: raw ops not yet implemented", op.Type)
	default:
		return fmt.Errorf("unknown op type %q", op.Type)
	}
}
