#!/usr/bin/env sh
set -eu

agent_binary=${1:-./asset-agent}

if [ ! -x "$agent_binary" ]; then
  printf '%s\n' "Agent binary is not executable: $agent_binary" >&2
  exit 2
fi

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/asset-agent-verify.XXXXXX")
chmod 0700 "$work_dir"
installed_agent="$work_dir/asset-agent"
cp "$agent_binary" "$installed_agent"
chmod 0755 "$installed_agent"

"$installed_agent" version > "$work_dir/version.json"
"$installed_agent" doctor > "$work_dir/doctor.json"
"$installed_agent" scan host
"$installed_agent" scan network
"$installed_agent" scan process
"$installed_agent" scan socket
"$installed_agent" scan all

output_dir="$work_dir/output"
snapshot_file=$(find "$output_dir" -maxdepth 1 -type f -name 'all-*.json' | sort | tail -n 1)

printf '%s\n' 'Agent listening sockets:'
if command -v jq >/dev/null 2>&1; then
  jq -r '.sockets[] | select(.state == "LISTEN") | "\(.protocol) \(.local_address):\(.local_port) inode=\(.inode) pids=\(.pids | join(","))"' "$snapshot_file"
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

printf '%s\n' "Verification completed. Reports retained at: $work_dir"
