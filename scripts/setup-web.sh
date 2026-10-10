#!/usr/bin/env bash
# One-time setup: scaffold the Next.js app in web/ and apply the Phase 0 page.
# Needs internet (npm registry). Run from anywhere: ./scripts/setup-web.sh
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -f web/package.json ]; then
  echo "web/ is already scaffolded, nothing to do."
  exit 0
fi

npx --yes create-next-app@latest web \
  --ts --tailwind --eslint --app --src-dir \
  --import-alias "@/*" --use-npm --yes

cp web-overlay/src/app/page.tsx web/src/app/page.tsx
echo "CATALOG_URL=http://localhost:8081" > web/.env.local
rm -rf web-overlay

echo
echo "Done. Next:"
echo "  docker compose up -d"
echo "  (cd services/catalog && go run .)   # terminal 1"
echo "  (cd web && npm run dev)             # terminal 2"
echo "Then open http://localhost:3000"
