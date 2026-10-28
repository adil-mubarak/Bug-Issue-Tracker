#!/usr/bin/env bash
set -Eeuo pipefail

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin"

cat >"$tmp/bin/mock" <<'MOCK'
#!/usr/bin/env bash
name="$(basename "$0")"
case "$name" in
  migrate)
    [[ -e "$TEST_TMP/gateway-started" ]] || { echo 'gateway was not started before migration' >&2; exit 9; }
    [[ "$*" == *'x-tls-ca=%2Ftmp%2Ftrackline-mysql-ca.pem'* && "$*" == *'tls=trackline'* ]] || {
      echo "migration CA path encoding or verified TLS setting was missing" >&2
      exit 10
    }
    touch "$TEST_TMP/migration-started"
    if [[ "${MIGRATE_FAIL:-}" == 1 ]]; then
      echo 'dial mysql://tracker:TOP_SECRET@tcp(db:3306)/auth_db: connection refused' >&2
      exit 1
    fi
    exit 0
    ;;
  gateway)
    touch "$TEST_TMP/gateway-started"
    exec /bin/sleep 60
    ;;
  auth|project|issue|activity|nats-server)
    exec /bin/sleep 60
    ;;
  nc|curl)
    exit 0
    ;;
  sleep)
    exit 0
    ;;
  *)
    echo "unexpected mock command: $name" >&2
    exit 1
    ;;
esac
MOCK
chmod +x "$tmp/bin/mock"
for command in migrate gateway auth project issue activity nats-server nc curl sleep; do
  ln -s mock "$tmp/bin/$command"
done

export PATH="$tmp/bin:$PATH" TEST_TMP="$tmp"
export PORT=10000 JWT_SECRET="01234567890123456789012345678901"
export CORS_ALLOWED_ORIGINS=https://example.test MYSQL_TLS_CA_PEM=placeholder
export AUTH_DATABASE_URL='mysql://auth:secret@tcp(db:3306)/auth_db'
export AUTH_MIGRATE_DATABASE_URL='mysql://tracker:TOP_SECRET@tcp(db:3306)/auth_db?multiStatements=true&tls=trackline&x-tls-ca=/tmp/trackline-mysql-ca.pem'
export PROJECT_DATABASE_URL='mysql://project:secret@tcp(db:3306)/project_db'
export PROJECT_MIGRATE_DATABASE_URL='mysql://tracker:secret@tcp(db:3306)/project_db?multiStatements=true&tls=trackline&x-tls-ca=/tmp/trackline-mysql-ca.pem'
export ISSUE_DATABASE_URL='mysql://issue:secret@tcp(db:3306)/issue_db'
export ISSUE_MIGRATE_DATABASE_URL='mysql://tracker:secret@tcp(db:3306)/issue_db?multiStatements=true&tls=trackline&x-tls-ca=/tmp/trackline-mysql-ca.pem'
export ACTIVITY_DATABASE_URL='mysql://activity:secret@tcp(db:3306)/activity_db'
export ACTIVITY_MIGRATE_DATABASE_URL='mysql://tracker:secret@tcp(db:3306)/activity_db?multiStatements=true&tls=trackline&x-tls-ca=/tmp/trackline-mysql-ca.pem'

# Success case: the migration mock refuses to proceed until the gateway was
# launched, proving the public listener is started before remote migrations.
"$repo_root/deploy/render/start.sh" >"$tmp/success.log" 2>&1 &
startup_pid=$!
for _ in {1..100}; do
  [[ -e "$tmp/migration-started" ]] && break
  /bin/sleep 0.02
done
if [[ ! -e "$tmp/migration-started" ]]; then
  cat "$tmp/success.log" >&2
  kill "$startup_pid" 2>/dev/null || true
  exit 1
fi
kill "$startup_pid" 2>/dev/null || true
wait "$startup_pid" 2>/dev/null || true

# Failure case: actionable migrate output is shown, but its DSN password is not.
rm -f "$tmp/gateway-started" "$tmp/migration-started"
export MIGRATE_FAIL=1
if "$repo_root/deploy/render/start.sh" >"$tmp/failure.log" 2>&1; then
  echo "expected startup to fail when migration fails" >&2
  exit 1
fi
grep -q 'connection refused' "$tmp/failure.log"
grep -q 'credentials redacted' "$tmp/failure.log"
grep -q 'mysql://tracker:\[REDACTED\]@' "$tmp/failure.log"
if grep -q 'TOP_SECRET' "$tmp/failure.log"; then
  echo "migration diagnostics leaked a database password" >&2
  exit 1
fi
echo "Render startup tests passed"
