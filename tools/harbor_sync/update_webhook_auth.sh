#!/usr/bin/env bash
set -euo pipefail

# update_webhook_auth.sh
# Usage:
#   ./update_webhook_auth.sh apply  \
#       --host 192.168.110.1:30003 \
#       --user admin --pass-file ./pw.txt \
#       --project 1 --policy 1 \
#       --auth-header 'X-Webhook-Token: YOUR_CUSTOM_TOKEN'
#
#   ./update_webhook_auth.sh restore --backup ./backups/policy_1_1_163...json
#
# Requires: curl, jq

prog=$(basename "$0")

usage(){
  cat <<EOF
Usage: $prog <apply|restore> [options]

Commands:
  apply    Fetch policy, save backup, set targets[].auth_header, PUT updated policy
  restore  PUT a previously saved backup JSON to restore original policy

Options (apply):
  --host HOST            Harbor host (default: 192.168.110.1:30003)
  --user USER            Harbor admin user (or set HARBOR_USER)
  --pass-file FILE       File containing password (safer than command line)
  --project ID_OR_NAME   Project id or name (default: 1)
  --policy ID            Webhook policy id (default: 1)
  --auth-header STR      Header value to set (e.g. 'X-Webhook-Token: YOUR_CUSTOM_TOKEN')

Options (restore):
  --backup FILE          Backup JSON file produced by apply

Examples:
  $prog apply --host 192.168.110.1:30003 --user admin --pass-file pw.txt --project 1 --policy 1 --auth-header 'X-Webhook-Token: YOUR_CUSTOM_TOKEN'
  $prog restore --host 192.168.110.1:30003 --user admin --pass-file pw.txt --backup ./backups/policy_1_1_163.json
EOF
}

if [ $# -lt 1 ]; then usage; exit 1; fi
cmd=$1; shift

# defaults
HOST=${HARBOR_HOST:-192.168.110.1:30003}
USER=${HARBOR_USER:-}
PASS_FILE=${HARBOR_PASS_FILE:-}
PROJECT=${PROJECT:-1}
POLICY=${POLICY:-1}
AUTH_HEADER=
BACKUP_FILE=

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host) HOST="$2"; shift 2;;
    --user) USER="$2"; shift 2;;
    --pass-file) PASS_FILE="$2"; shift 2;;
    --project) PROJECT="$2"; shift 2;;
    --policy) POLICY="$2"; shift 2;;
    --auth-header) AUTH_HEADER="$2"; shift 2;;
    --backup) BACKUP_FILE="$2"; shift 2;;
    -h|--help) usage; exit 0;;
    *) echo "Unknown arg: $1"; usage; exit 1;;
  esac
done

if [ -z "$USER" ]; then
  USER=${HARBOR_USER:-}
fi

if [ -z "$PASS_FILE" ]; then
  if [ -n "${HARBOR_PASS:-}" ]; then
    PASS_FILE=$(mktemp)
    echo -n "${HARBOR_PASS}" > "$PASS_FILE"
    trap 'rm -f "$PASS_FILE"' EXIT
  else
    echo "Provide --pass-file or set HARBOR_PASS env" >&2
    exit 1
  fi
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2; exit 1
fi

API_BASE="https://${HOST}/api/v2.0"
POLICY_URL="$API_BASE/projects/${PROJECT}/webhook/policies/${POLICY}"

auth() {
  printf "--user %s:$(cat "$PASS_FILE")" "$USER"
}

timestamp() { date +%s; }

case "$cmd" in
  apply)
    if [ -z "$AUTH_HEADER" ]; then
      echo "--auth-header is required for apply" >&2; exit 1
    fi

    mkdir -p backups
    bak=backups/policy_${PROJECT}_${POLICY}_$(timestamp).json
    echo "Fetching policy from $POLICY_URL"
    curl -sk -u "$USER:$(cat "$PASS_FILE")" -H "Accept: application/json" "$POLICY_URL" -o "$bak" || { echo "GET failed"; rm -f "$bak"; exit 1; }
    echo "Saved backup: $bak"

    echo "Patching targets[].auth_header -> '$AUTH_HEADER'"
    tmp=$(mktemp)
    jq --arg a "$AUTH_HEADER" '.targets |= map(.auth_header = $a)' "$bak" > "$tmp"

    echo "Uploading patched policy"
    curl -sk -u "$USER:$(cat "$PASS_FILE")" -X PUT -H "Content-Type: application/json" -d @"$tmp" "$POLICY_URL"
    echo
    echo "Patched policy applied. Backup at: $bak"
    ;;

  restore)
    if [ -z "$BACKUP_FILE" ]; then
      echo "--backup FILE is required for restore" >&2; exit 1
    fi
    if [ ! -f "$BACKUP_FILE" ]; then echo "Backup file not found: $BACKUP_FILE" >&2; exit 1; fi
    echo "Restoring policy from $BACKUP_FILE to $POLICY_URL"
    curl -sk -u "$USER:$(cat "$PASS_FILE")" -X PUT -H "Content-Type: application/json" -d @"$BACKUP_FILE" "$POLICY_URL"
    echo
    echo "Restore request sent."
    ;;

  *) usage; exit 1;;
esac

exit 0
