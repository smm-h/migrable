package cli

import (
	"sort"
	"testing"

	"github.com/smm-h/strictcli/go/strictcli"
	"github.com/smm-h/stricttest/go/hygiene"
)

// classification pins every command's strictcli effect classification and its
// `consequential` declaration. The table is the specification: changing a row
// here is the deliberate edit that a reclassification requires, and adding a
// command without adding its row fails the test.
//
// Reasoning, per the strictcli effects contract (§1: `read_only` means no
// user-visible or consequential mutation; §8.1: `consequential` means the act
// is worth interrupting someone for):
//
//   - init -- mutating. Writes migrable.toml and the migrations directory into
//     the target directory.
//   - migrate -- mutating. Rewrites the configured TOML files in place and
//     advances their recorded schema version, or with --rollback reverses the
//     most recently applied migration.
//   - merge -- mutating. Writes a new versioned migration file and consumes
//     the next/ staging files that fed it.
//   - status -- read_only. Reads the config, the migrations directory and the
//     recorded schema version, and prints them.
//   - validate -- read_only. Parses and checks the migration files.
//
// Nothing here is consequential. The bar is: destructive on a remote, creates
// a named external resource a rerun cannot un-create, or makes something
// public or live that was not before. `migrate` is the closest call and still
// fails every clause -- it edits local TOML files, the framework's --dry-run
// previews the exact diff first, and --rollback is a built-in undo.
//
// `man` is deliberately absent: it is a deprecated entry with no handler, and
// deprecated commands are classification-exempt (contract §1.1).
var classification = map[string]struct {
	effect        string
	consequential bool
}{
	"init":     {strictcli.EffectMutating, false},
	"migrate":  {strictcli.EffectMutating, false},
	"merge":    {strictcli.EffectMutating, false},
	"status":   {strictcli.EffectReadOnly, false},
	"validate": {strictcli.EffectReadOnly, false},
}

// collectCommands flattens the app's command tree into dotted paths.
func collectCommands(app *strictcli.App) map[string]*strictcli.Command {
	out := map[string]*strictcli.Command{}
	for name, cmd := range app.Commands() {
		out[name] = cmd
	}
	var walk func(prefix string, g *strictcli.Group)
	walk = func(prefix string, g *strictcli.Group) {
		for name, cmd := range g.Commands {
			out[prefix+name] = cmd
		}
		for name, sub := range g.Groups {
			walk(prefix+name+".", sub)
		}
	}
	for name, g := range app.Groups() {
		walk(name+".", g)
	}
	return out
}

func TestCommandClassificationIsPinned(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))

	cmds := collectCommands(NewApp("test"))

	var names []string
	for name := range cmds {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		want, ok := classification[name]
		if !ok {
			t.Errorf("command %q is registered but has no pinned classification; add a row to classification with the reasoning", name)
			continue
		}
		if cmds[name].Effect != want.effect {
			t.Errorf("command %q: effect = %q, pinned %q", name, cmds[name].Effect, want.effect)
		}
		if cmds[name].Consequential != want.consequential {
			t.Errorf("command %q: consequential = %v, pinned %v", name, cmds[name].Consequential, want.consequential)
		}
	}
	for name := range classification {
		if _, ok := cmds[name]; !ok {
			t.Errorf("classification pins %q but no such command is registered", name)
		}
	}
}

// TestNoReservedGlobalFlagNames guards the framework's reserved quartet at the
// app level. migrable used to declare --quiet and --verbose as globals and
// --dry-run on `migrate`; all three are now framework-owned and read off the
// Context. Command-level flags are covered implicitly: strictcli panics at
// registration for a reserved name anywhere, and NewApp registers everything.
func TestNoReservedGlobalFlagNames(t *testing.T) {
	hygiene.Isolate(t, hygiene.Preserve(hygiene.GoPath, hygiene.GoModCache, hygiene.GoCache))

	reserved := map[string]bool{"dry-run": true, "yes": true, "quiet": true, "verbose": true}
	for _, f := range NewApp("test").GlobalFlags() {
		if reserved[f.Name] {
			t.Errorf("global flag %q is reserved by the framework", f.Name)
		}
	}
}
