#!/usr/bin/env sh
set -eu

if ! command -v docker >/dev/null 2>&1; then
  echo "Error: docker is required to resolve ports from the Compose configuration." >&2
  exit 1
fi

if command -v ss >/dev/null 2>&1; then
  tcp_listeners=$(ss -H -ltn 2>/dev/null | awk '{ address = $4; sub(/^.*:/, "", address); print address }' || true)
  udp_listeners=$(ss -H -lun 2>/dev/null | awk '{ address = $4; sub(/^.*:/, "", address); print address }' || true)
elif command -v lsof >/dev/null 2>&1; then
  tcp_listeners=$(lsof -nP -iTCP -sTCP:LISTEN 2>/dev/null | awk 'NR > 1 { address = $9; sub(/^.*:/, "", address); print address }' || true)
  udp_listeners=$(lsof -nP -iUDP 2>/dev/null | awk 'NR > 1 { address = $9; sub(/^.*:/, "", address); print address }' || true)
else
  echo "Error: install either 'ss' or 'lsof' to check host ports." >&2
  exit 1
fi

published_ports=$(
  docker compose config | awk '
    /^[[:space:]]+published: / {
      gsub(/"/, "", $2)
      published = $2
    }
    /^[[:space:]]+protocol: / && published != "" {
      print published, $2
      published = ""
    }
  '
)

if [ -z "$published_ports" ]; then
  echo "No host ports are published by Docker Compose."
  exit 0
fi

is_listening() {
  port=$1
  protocol=$2

  if [ "$protocol" = "udp" ]; then
    listeners=$udp_listeners
  else
    listeners=$tcp_listeners
  fi

  printf '%s\n' "$listeners" | awk -v port="$port" '$0 == port { found = 1 } END { exit !found }'
}

busy=0
checked=0
while read -r published protocol; do
  [ -n "$published" ] || continue

  case "$published" in
    *-*)
      start=${published%-*}
      end=${published#*-}
      ;;
    *)
      start=$published
      end=$published
      ;;
  esac

  port=$start
  while [ "$port" -le "$end" ]; do
    checked=$((checked + 1))
    if is_listening "$port" "$protocol"; then
      echo "Port $port/$protocol is already in use." >&2
      busy=1
    fi
    port=$((port + 1))
  done
done <<EOF
$published_ports
EOF

if [ "$busy" -ne 0 ]; then
  echo "Build aborted: free the ports above or change their values in .env." >&2
  exit 1
fi

echo "Port check passed ($checked published host ports available)."
