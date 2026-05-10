package scenario

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	tomledit "github.com/smm-h/go-toml-edit"
	"github.com/smm-h/migrable/config"
	"github.com/smm-h/migrable/engine"
	"github.com/smm-h/migrable/internal/cli"
)

// Scenario represents an isolated migration test environment.
type Scenario struct {
	t           *testing.T
	dir         string
	files       map[string]string // key -> relative path
	versionFile string
	inputs      map[string]string // key -> TOML content
	migrations  map[string]string // version -> TOML content
	materialized bool             // true after files are written to disk
}

// Result holds the outcome of a scenario execution step.
type Result struct {
	scenario    *Scenario
	err         error
	applied     int
	fromVer     string
	toVer       string
	currentVer  string
	pending     int
	preSnapshot map[string][]byte
}

// New creates a scenario with an isolated temp directory.
func New(t *testing.T) *Scenario {
	t.Helper()
	dir := t.TempDir()

	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.Mkdir(migrationsDir, 0o755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	return &Scenario{
		t:          t,
		dir:        dir,
		files:      make(map[string]string),
		inputs:     make(map[string]string),
		migrations: make(map[string]string),
	}
}

// SingleFile configures a single-file project.
func (s *Scenario) SingleFile(fileKey, relativePath string) *Scenario {
	s.files[fileKey] = relativePath
	s.versionFile = ""
	return s
}

// MultiFile configures a multi-file project.
func (s *Scenario) MultiFile(files map[string]string, versionFile string) *Scenario {
	for k, v := range files {
		s.files[k] = v
	}
	s.versionFile = versionFile
	return s
}

// Input writes initial content for a file key.
func (s *Scenario) Input(fileKey, tomlContent string) *Scenario {
	s.inputs[fileKey] = tomlContent
	return s
}

// Migration places a migration file in the migrations directory.
func (s *Scenario) Migration(version, content string) *Scenario {
	s.migrations[version] = content
	return s
}

// Env sets an environment variable (restored after test).
func (s *Scenario) Env(key, value string) *Scenario {
	s.t.Setenv(key, value)
	return s
}

// materialize writes input files and migrations to disk once.
func (s *Scenario) materialize() {
	s.t.Helper()
	if s.materialized {
		return
	}
	s.materialized = true

	// Write input files.
	for key, content := range s.inputs {
		rel, ok := s.files[key]
		if !ok {
			s.t.Fatalf("Input called for unknown file key %q", key)
		}
		absPath := filepath.Join(s.dir, rel)
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			s.t.Fatalf("failed to create directory for %s: %v", rel, err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			s.t.Fatalf("failed to write input file %s: %v", rel, err)
		}
	}

	// Write migration files.
	migrationsDir := filepath.Join(s.dir, "migrations")
	for version, content := range s.migrations {
		migPath := filepath.Join(migrationsDir, version+".toml")
		if err := os.WriteFile(migPath, []byte(content), 0o644); err != nil {
			s.t.Fatalf("failed to write migration %s: %v", version, err)
		}
	}
}

// buildConfig constructs a config.Config, materializing files if needed.
func (s *Scenario) buildConfig() *config.Config {
	s.t.Helper()
	s.materialize()

	cfg := &config.Config{
		MigrationsDir: "migrations",
		VersionFile:   s.versionFile,
		Files:         make(map[string]string, len(s.files)),
		BaseDir:       s.dir,
	}
	for k, v := range s.files {
		cfg.Files[k] = v
	}

	return cfg
}

// snapshot captures current file contents for untouched assertions.
func (s *Scenario) snapshot() map[string][]byte {
	snap := make(map[string][]byte, len(s.files))
	for key, rel := range s.files {
		absPath := filepath.Join(s.dir, rel)
		data, err := os.ReadFile(absPath)
		if err != nil {
			// File may not exist yet (fresh install).
			continue
		}
		snap[key] = data
	}
	return snap
}

// Migrate runs forward migration.
func (s *Scenario) Migrate() *Result {
	s.t.Helper()
	cfg := s.buildConfig()
	pre := s.snapshot()

	res, err := engine.Migrate(cfg, false)

	r := &Result{
		scenario:    s,
		err:         err,
		preSnapshot: pre,
	}
	if res != nil {
		r.applied = res.Applied
		r.fromVer = res.FromVersion.String()
		r.toVer = res.ToVersion.String()
	}
	return r
}

// MigrateDryRun runs migration in dry-run mode.
func (s *Scenario) MigrateDryRun() *Result {
	s.t.Helper()
	cfg := s.buildConfig()
	pre := s.snapshot()

	res, err := engine.Migrate(cfg, true)

	r := &Result{
		scenario:    s,
		err:         err,
		preSnapshot: pre,
	}
	if res != nil {
		r.applied = res.Applied
		r.fromVer = res.FromVersion.String()
		r.toVer = res.ToVersion.String()
	}
	return r
}

// Rollback rolls back the last applied migration.
func (s *Scenario) Rollback() *Result {
	s.t.Helper()
	cfg := s.buildConfig()
	pre := s.snapshot()

	res, err := engine.Rollback(cfg, false)

	r := &Result{
		scenario:    s,
		err:         err,
		preSnapshot: pre,
	}
	if res != nil {
		r.applied = res.Applied
		r.fromVer = res.FromVersion.String()
		r.toVer = res.ToVersion.String()
	}
	return r
}

// RollbackDryRun runs rollback in dry-run mode.
func (s *Scenario) RollbackDryRun() *Result {
	s.t.Helper()
	cfg := s.buildConfig()
	pre := s.snapshot()

	res, err := engine.Rollback(cfg, true)

	r := &Result{
		scenario:    s,
		err:         err,
		preSnapshot: pre,
	}
	if res != nil {
		r.applied = res.Applied
		r.fromVer = res.FromVersion.String()
		r.toVer = res.ToVersion.String()
	}
	return r
}

// Status computes current migration status.
func (s *Scenario) Status() *Result {
	s.t.Helper()
	cfg := s.buildConfig()
	pre := s.snapshot()

	migrationsDir := filepath.Join(s.dir, "migrations")
	allMigrations, err := engine.DiscoverMigrations(migrationsDir)
	if err != nil {
		return &Result{scenario: s, err: err, preSnapshot: pre}
	}

	// Determine version file key.
	versionKey := cfg.VersionFile
	if versionKey == "" {
		for k := range cfg.Files {
			versionKey = k
			break
		}
	}
	versionFilePath := cfg.Files[versionKey]
	if !filepath.IsAbs(versionFilePath) {
		versionFilePath = filepath.Join(cfg.BaseDir, versionFilePath)
	}

	currentVersion, err := engine.ReadSchemaVersion(versionFilePath)
	if err != nil {
		return &Result{scenario: s, err: err, preSnapshot: pre}
	}

	pending := 0
	for _, m := range allMigrations {
		if m.Version.GreaterThan(currentVersion) {
			pending++
		}
	}

	return &Result{
		scenario:    s,
		currentVer:  currentVersion.String(),
		pending:     pending,
		preSnapshot: pre,
	}
}

// --- Assertion methods on Result ---

// AssertSuccess asserts no error.
func (r *Result) AssertSuccess() *Result {
	r.scenario.t.Helper()
	if r.err != nil {
		r.scenario.t.Errorf("expected success, got error: %v", r.err)
	}
	return r
}

// AssertApplied asserts the number of migrations applied.
func (r *Result) AssertApplied(n int) *Result {
	r.scenario.t.Helper()
	if r.applied != n {
		r.scenario.t.Errorf("applied = %d, want %d", r.applied, n)
	}
	return r
}

// AssertVersion checks _schema_version in a file.
func (r *Result) AssertVersion(fileKey, version string) *Result {
	r.scenario.t.Helper()
	rel, ok := r.scenario.files[fileKey]
	if !ok {
		r.scenario.t.Errorf("AssertVersion: unknown file key %q", fileKey)
		return r
	}
	absPath := filepath.Join(r.scenario.dir, rel)
	data, err := os.ReadFile(absPath)
	if err != nil {
		r.scenario.t.Errorf("AssertVersion: failed to read %s: %v", absPath, err)
		return r
	}
	doc, err := tomledit.Parse(data)
	if err != nil {
		r.scenario.t.Errorf("AssertVersion: failed to parse %s: %v", absPath, err)
		return r
	}
	verStr, ok := doc.GetString("_schema_version")
	if !ok {
		r.scenario.t.Errorf("AssertVersion: _schema_version not found in %s", fileKey)
		return r
	}
	if verStr != version {
		r.scenario.t.Errorf("AssertVersion: _schema_version in %s = %q, want %q", fileKey, verStr, version)
	}
	return r
}

// AssertKey checks a TOML key's value at a dot-path.
func (r *Result) AssertKey(fileKey, path string, expected any) *Result {
	r.scenario.t.Helper()
	rel, ok := r.scenario.files[fileKey]
	if !ok {
		r.scenario.t.Errorf("AssertKey: unknown file key %q", fileKey)
		return r
	}
	absPath := filepath.Join(r.scenario.dir, rel)
	data, err := os.ReadFile(absPath)
	if err != nil {
		r.scenario.t.Errorf("AssertKey: failed to read %s: %v", absPath, err)
		return r
	}
	doc, err := tomledit.Parse(data)
	if err != nil {
		r.scenario.t.Errorf("AssertKey: failed to parse %s: %v", absPath, err)
		return r
	}
	node := doc.Get(path)
	if node == nil {
		r.scenario.t.Errorf("AssertKey: path %q not found in %s", path, fileKey)
		return r
	}
	actual := node.Value()
	if !valuesEqual(actual, expected) {
		r.scenario.t.Errorf("AssertKey: %s.%s = %v (%T), want %v (%T)", fileKey, path, actual, actual, expected, expected)
	}
	return r
}

// AssertHasKey checks a key exists.
func (r *Result) AssertHasKey(fileKey, path string) *Result {
	r.scenario.t.Helper()
	rel, ok := r.scenario.files[fileKey]
	if !ok {
		r.scenario.t.Errorf("AssertHasKey: unknown file key %q", fileKey)
		return r
	}
	absPath := filepath.Join(r.scenario.dir, rel)
	data, err := os.ReadFile(absPath)
	if err != nil {
		r.scenario.t.Errorf("AssertHasKey: failed to read %s: %v", absPath, err)
		return r
	}
	doc, err := tomledit.Parse(data)
	if err != nil {
		r.scenario.t.Errorf("AssertHasKey: failed to parse %s: %v", absPath, err)
		return r
	}
	if doc.Get(path) == nil {
		r.scenario.t.Errorf("AssertHasKey: path %q not found in %s", path, fileKey)
	}
	return r
}

// AssertMissingKey checks a key does NOT exist.
func (r *Result) AssertMissingKey(fileKey, path string) *Result {
	r.scenario.t.Helper()
	rel, ok := r.scenario.files[fileKey]
	if !ok {
		r.scenario.t.Errorf("AssertMissingKey: unknown file key %q", fileKey)
		return r
	}
	absPath := filepath.Join(r.scenario.dir, rel)
	data, err := os.ReadFile(absPath)
	if err != nil {
		// File not existing means key is missing.
		if os.IsNotExist(err) {
			return r
		}
		r.scenario.t.Errorf("AssertMissingKey: failed to read %s: %v", absPath, err)
		return r
	}
	doc, err := tomledit.Parse(data)
	if err != nil {
		r.scenario.t.Errorf("AssertMissingKey: failed to parse %s: %v", absPath, err)
		return r
	}
	if doc.Get(path) != nil {
		r.scenario.t.Errorf("AssertMissingKey: path %q exists in %s but should not", path, fileKey)
	}
	return r
}

// AssertFileExists checks the file exists on disk.
func (r *Result) AssertFileExists(fileKey string) *Result {
	r.scenario.t.Helper()
	rel, ok := r.scenario.files[fileKey]
	if !ok {
		r.scenario.t.Errorf("AssertFileExists: unknown file key %q", fileKey)
		return r
	}
	absPath := filepath.Join(r.scenario.dir, rel)
	if _, err := os.Stat(absPath); err != nil {
		r.scenario.t.Errorf("AssertFileExists: file %s does not exist", fileKey)
	}
	return r
}

// AssertFileNotExists checks the file does NOT exist on disk.
func (r *Result) AssertFileNotExists(fileKey string) *Result {
	r.scenario.t.Helper()
	rel, ok := r.scenario.files[fileKey]
	if !ok {
		r.scenario.t.Errorf("AssertFileNotExists: unknown file key %q", fileKey)
		return r
	}
	absPath := filepath.Join(r.scenario.dir, rel)
	if _, err := os.Stat(absPath); err == nil {
		r.scenario.t.Errorf("AssertFileNotExists: file %s exists but should not", fileKey)
	}
	return r
}

// AssertUntouched checks file content is unchanged from before the step.
func (r *Result) AssertUntouched(fileKey string) *Result {
	r.scenario.t.Helper()
	rel, ok := r.scenario.files[fileKey]
	if !ok {
		r.scenario.t.Errorf("AssertUntouched: unknown file key %q", fileKey)
		return r
	}
	absPath := filepath.Join(r.scenario.dir, rel)
	before, hasBefore := r.preSnapshot[fileKey]
	after, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) && !hasBefore {
			return r // didn't exist before, doesn't exist now
		}
		r.scenario.t.Errorf("AssertUntouched: failed to read %s: %v", absPath, err)
		return r
	}
	if !hasBefore {
		r.scenario.t.Errorf("AssertUntouched: file %s did not exist before but exists now", fileKey)
		return r
	}
	if !bytes.Equal(before, after) {
		r.scenario.t.Errorf("AssertUntouched: file %s was modified", fileKey)
	}
	return r
}

// AssertExitCode checks the exit code.
func (r *Result) AssertExitCode(code int) *Result {
	r.scenario.t.Helper()
	actual := exitCodeFromError(r.err)
	if actual != code {
		r.scenario.t.Errorf("exit code = %d, want %d (error: %v)", actual, code, r.err)
	}
	return r
}

// AssertError checks the error message contains a substring.
func (r *Result) AssertError(substring string) *Result {
	r.scenario.t.Helper()
	if r.err == nil {
		r.scenario.t.Errorf("expected error containing %q, got nil", substring)
		return r
	}
	if !strings.Contains(r.err.Error(), substring) {
		r.scenario.t.Errorf("error %q does not contain %q", r.err.Error(), substring)
	}
	return r
}

// AssertPending checks the number of pending migrations (for Status results).
func (r *Result) AssertPending(n int) *Result {
	r.scenario.t.Helper()
	if r.pending != n {
		r.scenario.t.Errorf("pending = %d, want %d", r.pending, n)
	}
	return r
}

// exitCodeFromError maps an error to an exit code.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *cli.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	var rbErr *engine.RollbackBlockedError
	if errors.As(err, &rbErr) {
		return cli.ExitRollbackBlocked
	}
	return 1
}

// valuesEqual compares two values, handling type mismatches between int64/int/float64.
func valuesEqual(actual, expected any) bool {
	if actual == expected {
		return true
	}
	// Handle numeric type mismatches.
	if a, ok := toFloat64(actual); ok {
		if b, ok := toFloat64(expected); ok {
			return a == b
		}
	}
	// Handle bool comparison (go-toml-edit returns bool directly).
	if a, ok := actual.(bool); ok {
		if b, ok := expected.(bool); ok {
			return a == b
		}
	}
	// Fall back to string comparison.
	return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// Ensure semver is imported for go.sum consistency.
var _ = semver.MustParse("0.0.0")
