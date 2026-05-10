#!/usr/bin/env bash
# Post-release hook. Runs after a successful release (non-fatal).
# Environment: RLSBL_VERSION is set to the released version.
# Customize this for your project (e.g., local install, deploy, notify).

set -euo pipefail

echo "Post-release: v$RLSBL_VERSION"

if [ -f ~/Projects/.selfdoc.env ]; then
  set -a && source ~/Projects/.selfdoc.env && set +a
fi
echo "Building and deploying docs..."
selfdoc build && selfdoc deploy
