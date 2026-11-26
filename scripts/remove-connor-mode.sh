#!/bin/sh
# Remove the temporary Connor Mode theme and leave the permanent themes intact.
set -eu

repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

rm -f internal/web/connor_mode.go internal/web/connor_mode_test.go

# Strip the blocks marked BEGIN/END Connor Mode in the stylesheet, script, and
# layout. The markers only exist while the theme exists.
perl -0pi -e 's{[ \t]*/\*\s*BEGIN Connor Mode\s*\*/.*?/\*\s*END Connor Mode\s*\*/\n?}{}gs' web/tailwind/input.css
perl -0pi -e 's{[ \t]*// BEGIN Connor Mode\n.*?// END Connor Mode\n?}{}gs' web/static/app.js
perl -0pi -e 's{[ \t]*\{\{/\* BEGIN Connor Mode \*/\}\}.*?\{\{/\* END Connor Mode \*/\}\}\n?}{}gs' web/templates/layout.html

# Drop the theme's reference page row and prose block.
perl -0pi -e 's{<!-- BEGIN Connor Mode -->.*?<!-- END Connor Mode -->\n?}{}gs' docs/reference/routes.md
perl -ni -e 'print unless /^\| `connor-mode`/' docs/reference/routes.md
perl -0pi -e 's/\n{3,}/\n\n/g' docs/reference/routes.md

# Rebuild the compiled stylesheet without the theme's CSS.
if command -v tailwindcss >/dev/null 2>&1; then
	tailwindcss -i ./web/tailwind/input.css -o ./web/static/app.css --minify
else
	mise exec -- tailwindcss -i ./web/tailwind/input.css -o ./web/static/app.css --minify
fi

gofmt -w internal/web
go test ./internal/web/...

echo "remove-connor-mode: removed Connor Mode; remaining themes are intact"
