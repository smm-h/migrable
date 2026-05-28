#!/usr/bin/env bash
set -euo pipefail
<<<<<<< /home/m/Projects/migrable/tmp6q2wto3h.ours

echo "Running pre-release checks..."

echo "  Updating CLI schema..."
go run . --dump-schema

if [ -f go.mod ]; then
  echo "  Go: vet + build + test"
  go vet ./...
  go build ./...
  go test ./... -race -short -count=1
fi

if [ -f pyproject.toml ]; then
  echo "  Python: pytest"
  if command -v uv &>/dev/null; then
    uv run pytest
  elif command -v pytest &>/dev/null; then
    pytest
  fi
fi

if [ -f package.json ] && node -e "process.exit(require('./package.json').scripts?.test ? 0 : 1)" 2>/dev/null; then
  echo "  npm: test"
  npm test
fi

echo "Pre-release checks passed."
=======
# Project-specific pre-release checks.
# Built-in checks (tests, lint) run automatically before this hook.
# Add custom validation here, e.g.:
#   - Check for uncommitted documentation
#   - Verify external service connectivity
#   - Run integration tests not covered by the test suite
>>>>>>> /home/m/Projects/migrable/tmpmisxp72k.theirs
