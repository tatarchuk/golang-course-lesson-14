#!/usr/bin/env bash
# Renders the ER diagram docs/er.puml to docs/er.svg with a local PlantUML
# (brew install plantuml, or any plantuml on PATH). Run from the repository root.
set -euo pipefail
cd "$(dirname "$0")/.."
command -v plantuml >/dev/null || { echo "plantuml not found: brew install plantuml" >&2; exit 1; }
plantuml -tsvg -o . docs/er.puml
echo "rendered docs/er.svg"
