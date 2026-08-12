#!/usr/bin/env sh
set -eu

agent_binary=${1:-./asset-agent}

if [ ! -x "$agent_binary" ]; then
  printf '%s\n' "Agent binary is not executable: $agent_binary" >&2
  exit 2
fi

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/asset-agent-verify.XXXXXX")
cleanup() {
  rm -rf "$work_dir"
}
trap cleanup EXIT HUP INT TERM

"$agent_binary" version
"$agent_binary" doctor > "$work_dir/doctor.json"
"$agent_binary" scan --output "$work_dir/snapshot.json"

printf '%s\n' 'Agent listening sockets:'
if command -v jq >/dev/null 2>&1; then
  jq -r '.sockets[] | select(.state == "LISTEN") | "\(.protocol) \(.local_address):\(.local_port) inode=\(.inode) pids=\(.pids | join(","))"' "$work_dir/snapshot.json"
else
  printf '%s\n' 'jq is not installed; inspect the JSON report manually.'
fi

printf '%s\n' 'Native operating-system reference:'
uname -a
if [ -r /etc/os-release ]; then
  sed -n '1,20p' /etc/os-release
fi
if command -v ss >/dev/null 2>&1; then
  ss -lntup || true
else
  printf '%s\n' 'ss is not installed; native socket comparison skipped.'
fi

printf '%s\n' 'Verification completed. Temporary reports were removed.'

