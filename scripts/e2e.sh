#!/bin/sh
# Runs the Cypress suite against a throwaway server: own database, one flat DEM tile, Mailpit.
set -eu
cd "$(dirname "$0")/.."

: "${E2E_BIN:=./radiopath}"
: "${E2E_BASE_URL:=http://localhost:8081}"
: "${E2E_DATABASE_URL:=postgres://radiopath:radiopath@localhost:5432/radiopath_e2e}"
: "${E2E_SMTP_HOST:=localhost}"
: "${E2E_MAILPIT_URL:=http://localhost:8025}"
: "${E2E_ADMIN_TOKEN:=e2e-only-admin-token-not-a-secret-1234}"
export E2E_BIN E2E_BASE_URL E2E_DATABASE_URL E2E_MAILPIT_URL E2E_ADMIN_TOKEN

[ -x "$E2E_BIN" ] || { echo "$E2E_BIN missing, run make build" >&2; exit 1; }

dem=tmp/e2e/dem
mkdir -p "$dem"
[ -s "$dem/N47E009.hgt" ] || head -c 25934402 /dev/zero >"$dem/N47E009.hgt"

[ -n "${CI:-}" ] || docker compose exec -T db createdb -U radiopath radiopath_e2e 2>/dev/null || true

# env -i: the Makefile exports .env, which must not leak into the test server
start() {
	env -i PATH="$PATH" HOME="$HOME" \
		DATABASE_URL="$E2E_DATABASE_URL" \
		RADIOPATH_LISTEN=":${E2E_BASE_URL##*:}" \
		RADIOPATH_METRICS_LISTEN= \
		RADIOPATH_DEM_DIR="$dem" \
		RADIOPATH_WORKERS=1 \
		RADIOPATH_LOG_LEVEL=warn \
		RADIOPATH_REGISTRATION=true \
		RADIOPATH_SMTP_HOST="$E2E_SMTP_HOST" \
		RADIOPATH_SMTP_PORT=1025 \
		RADIOPATH_SMTP_STARTTLS=false \
		RADIOPATH_MAIL_FROM=radiopath@example.test \
		RADIOPATH_BASE_URL="$E2E_BASE_URL" \
		RADIOPATH_ADMIN_TOKEN="$E2E_ADMIN_TOKEN" \
		"$E2E_BIN" >>tmp/e2e/app.log 2>&1 &
	pid=$!
}

: >tmp/e2e/app.log
start
trap 'kill $pid 2>/dev/null' EXIT

# node instead of curl: the Cypress image has no curl
up() {
	node -e 'fetch(process.argv[1]).then(r => process.exit(r.ok ? 0 : 1), () => process.exit(1))' "$E2E_BASE_URL/healthz"
}

i=0
until up; do
	i=$((i + 1))
	if [ "$i" -gt 60 ]; then
		echo "server did not come up:" >&2
		cat tmp/e2e/app.log >&2
		exit 1
	fi
	kill -0 "$pid" 2>/dev/null || start
	sleep 1
done

if [ -n "${E2E_OPEN:-}" ]; then
	npx cypress open --e2e
else
	npx cypress run
fi
