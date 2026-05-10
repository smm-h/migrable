package ops

type OpType string

const (
	OpAddField          OpType = "add_field"
	OpRemoveField       OpType = "remove_field"
	OpRenameField       OpType = "rename_field"
	OpMoveField         OpType = "move_field"
	OpAddCollection     OpType = "add_collection"
	OpDropCollection    OpType = "drop_collection"
	OpSetValue          OpType = "set_value"
	OpSetValueWhere     OpType = "set_value_where"
	OpRemoveWhere       OpType = "remove_where"
	OpAppend            OpType = "append"
	OpTransform         OpType = "transform"
	OpMergeDefaults     OpType = "merge_defaults"
	OpMergeDefaultsByKey OpType = "merge_defaults_by_key"
	OpRaw               OpType = "raw"
)

type Op struct {
	Type    OpType
	Section string // "structure" or "data"

	// Common
	Path string
	Down *DownOp

	// add_field
	FieldType string // "type" field from TOML
	Default   any

	// rename_field, move_field
	From string
	To   string

	// add_collection
	Fields map[string]any

	// set_value, raw
	Value   any
	Content string

	// set_value_where, remove_where
	Where     map[string]any
	MatchMode string
	Set       map[string]any

	// transform
	Expr string

	// merge_defaults_by_key
	MatchField string
	Defaults   []map[string]any
}

type DownOp struct {
	Irreversible bool
	Ops          []Op
}
