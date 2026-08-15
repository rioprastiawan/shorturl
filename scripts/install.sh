#!/usr/bin/env bash
#
# Prepare a .env for a production deployment: check prerequisites, generate the
# secrets, and report what still needs a human decision.
#
# Safe to re-run. Values that are already set are never overwritten, so this
# will not rotate your secrets or lock you out of an existing deployment.
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ENV_FILE=".env"
GENERATED=()
KEPT=()

bold()  { printf '\033[1m%s\033[0m\n' "$1"; }
ok()    { printf '  \033[32m✓\033[0m %s\n' "$1"; }
warn()  { printf '  \033[33m!\033[0m %s\n' "$1"; }
fail()  { printf '  \033[31m✗\033[0m %s\n' "$1"; }

# --- Prerequisites -----------------------------------------------------------
bold "Checking prerequisites"

missing=0
if command -v docker >/dev/null 2>&1; then
  ok "docker $(docker version --format '{{.Server.Version}}' 2>/dev/null || echo '(daemon not reachable)')"
  if ! docker info >/dev/null 2>&1; then
    fail "the Docker daemon is not running or this user cannot reach it"
    missing=1
  fi
else
  fail "docker is not installed - see https://docs.docker.com/engine/install/"
  missing=1
fi

if docker compose version >/dev/null 2>&1; then
  ok "docker compose $(docker compose version --short)"
else
  fail "docker compose v2 is not available"
  missing=1
fi

if command -v openssl >/dev/null 2>&1; then
  ok "openssl"
else
  fail "openssl is required to generate secrets"
  missing=1
fi

if [[ $missing -eq 1 ]]; then
  echo
  echo "Install the missing prerequisites and run this script again."
  exit 1
fi

# --- .env --------------------------------------------------------------------
echo
bold "Preparing $ENV_FILE"

if [[ ! -f "$ENV_FILE" ]]; then
  cp .env.example "$ENV_FILE"
  ok "created $ENV_FILE from .env.example"
else
  ok "$ENV_FILE already exists, leaving existing values alone"
fi

# .env holds every secret for the deployment.
chmod 600 "$ENV_FILE"

current_value() {
  # Last assignment wins, matching how docker compose reads the file.
  grep -E "^$1=" "$ENV_FILE" | tail -n1 | cut -d= -f2- || true
}

set_value() {
  local key="$1" value="$2" tmp
  tmp="$(mktemp)"
  if grep -qE "^$key=" "$ENV_FILE"; then
    # Written via awk rather than sed -i, whose syntax differs between GNU and
    # BSD/macOS. index()/substr() avoid regex-escaping the generated value.
    awk -v key="$key" -v value="$value" '
      index($0, key "=") == 1 { print key "=" value; next }
      { print }
    ' "$ENV_FILE" > "$tmp"
  else
    cat "$ENV_FILE" > "$tmp"
    printf '%s=%s\n' "$key" "$value" >> "$tmp"
  fi
  cat "$tmp" > "$ENV_FILE"
  rm -f "$tmp"
}

# A value counts as unset if it is empty or still the .env.example placeholder.
needs_secret() {
  local value
  value="$(current_value "$1")"
  [[ -z "$value" || "$value" == "change-this" ]]
}

generate_secret() {
  local key="$1"
  if needs_secret "$key"; then
    set_value "$key" "$(openssl rand -hex 32)"
    GENERATED+=("$key")
  else
    KEPT+=("$key")
  fi
}

generate_password() {
  local key="$1"
  if needs_secret "$key"; then
    # base64 then strip non-alphanumerics: the result goes into a URL-form
    # DATABASE_URL, where +, / and = would need percent-encoding.
    set_value "$key" "$(openssl rand -base64 36 | tr -dc 'A-Za-z0-9' | head -c 32)"
    GENERATED+=("$key")
  else
    KEPT+=("$key")
  fi
}

generate_password POSTGRES_PASSWORD
generate_password REDIS_PASSWORD
generate_secret  SESSION_SECRET
generate_secret  IP_HASH_SECRET

if [[ ${#GENERATED[@]} -gt 0 ]]; then
  ok "generated: ${GENERATED[*]}"
fi
if [[ ${#KEPT[@]} -gt 0 ]]; then
  ok "kept existing: ${KEPT[*]}"
fi

# --- Values only a human can decide ------------------------------------------
echo
bold "Checking deployment settings"

todo=0

app_domain="$(current_value APP_DOMAIN)"
if [[ -z "$app_domain" || "$app_domain" == "short.example.com" ]]; then
  warn "APP_DOMAIN is still the example value - set it to your real hostname"
  todo=1
else
  ok "APP_DOMAIN=$app_domain"
  if command -v dig >/dev/null 2>&1; then
    resolved="$(dig +short "$app_domain" A | tail -n1)"
    if [[ -z "$resolved" ]]; then
      warn "$app_domain has no A record yet - Let's Encrypt will fail until it does"
      todo=1
    else
      ok "$app_domain resolves to $resolved"
    fi
  fi
fi

acme_email="$(current_value TRAEFIK_ACME_EMAIL)"
if [[ -z "$acme_email" || "$acme_email" == "admin@example.com" ]]; then
  warn "TRAEFIK_ACME_EMAIL is still the example value - set a real address"
  todo=1
else
  ok "TRAEFIK_ACME_EMAIL=$acme_email"
fi

app_url="$(current_value APP_URL)"
if [[ -n "$app_domain" && "$app_url" != "https://$app_domain" ]]; then
  warn "APP_URL ($app_url) does not match https://$app_domain"
  todo=1
fi

# --- Next steps --------------------------------------------------------------
echo
if [[ $todo -eq 1 ]]; then
  bold "Next steps"
  echo "  1. Edit .env and fix the warnings above."
  echo "  2. Point your domain's A record at this server."
  echo "  3. docker compose up -d"
else
  bold "Ready"
  echo "  docker compose up -d"
  echo
  echo "  Then watch the certificate get issued:"
  echo "  docker compose logs -f traefik"
fi
echo
echo "  Tip: while testing, set ACME_CA_SERVER to the Let's Encrypt staging URL"
echo "  in .env. Its rate limits are much higher and a failed run costs nothing."
echo
