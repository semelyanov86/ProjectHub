#!/usr/bin/env bash
# Prints the CHANGELOG.md section of one version (used as GitHub release notes),
# followed by installation instructions for the .deb.
# Usage: scripts/release-notes.sh <version>
set -euo pipefail
cd "$(dirname "$0")/.."

version=${1:?usage: release-notes.sh <version>}
notes=$(awk -v heading="## [$version]" '
	index($0, heading) == 1 { found = 1; next }
	found && /^## \[/ { exit }
	found && /^\[[^]]+\]: / { exit }
	found { print }
' CHANGELOG.md)
if [[ -z ${notes//[[:space:]]/} ]]; then
	echo "CHANGELOG.md has no section for $version" >&2
	exit 1
fi
printf '%s\n' "$notes"
cat <<EOT

## Installation (Ubuntu 24.04 / 26.04, amd64)

Download \`projecthub_${version}_amd64.deb\` and \`SHA256SUMS\` from this release, then:

\`\`\`bash
sha256sum --check --ignore-missing SHA256SUMS
sudo apt install ./projecthub_${version}_amd64.deb
\`\`\`

Start ProjectHub from the applications menu or run \`projecthub\`. Remove it with \`sudo apt remove projecthub\`.
EOT
