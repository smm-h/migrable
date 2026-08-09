// Deprecated: retired; superseded by strictspec.
module github.com/smm-h/migrable

go 1.25.7

require (
	github.com/Masterminds/semver/v3 v3.5.0
	github.com/google/cel-go v0.28.0
	github.com/smm-h/go-toml-edit v0.2.2
	github.com/smm-h/strictcli/go v0.29.0
	github.com/smm-h/stricttest/go v0.1.1
	google.golang.org/protobuf v1.36.10
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240826202546-f6391c0de4c7 // indirect
)

// The module is retired; use strictspec instead. The closed interval starts at
// v0.0.0-0 (the lowest possible pre-release, so pseudo-versions of untagged
// commits are inside it too) and ends at this final v0.7.1 release itself, so
// every published version and every proxy-synthesised pseudo-version below it
// is retracted.
retract [v0.0.0-0, v0.7.1]
