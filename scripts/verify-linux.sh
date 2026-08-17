#!/usr/bin/env sh
set -eu

agent_binary=${1:-./asset-agent}

if [ "$(id -u)" -ne 0 ]; then
  printf '%s\n' '必须以 root 运行真实 Linux 验证。' >&2
  exit 2
fi
if [ ! -x "$agent_binary" ]; then
  printf '%s\n' "Agent 不可执行: $agent_binary" >&2
  exit 2
fi
if ! command -v jq >/dev/null 2>&1; then
  printf '%s\n' '缺少 jq，无法校验 manifest。' >&2
  exit 2
fi
if command -v sha256sum >/dev/null 2>&1; then
  sha256_file() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  sha256_file() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  printf '%s\n' '缺少 sha256sum 或 shasum。' >&2
  exit 2
fi

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/asset-agent-verify.XXXXXX")
chmod 0700 "$work_dir"
installed_agent="$work_dir/asset-agent"
cp "$agent_binary" "$installed_agent"
chmod 0755 "$installed_agent"
output_root="$work_dir/output"

validate_batch() {
  batch_dir=$1
  manifest="$batch_dir/manifest.json"
  [ -d "$batch_dir" ] || { printf '%s\n' "正式批次目录不存在: $batch_dir" >&2; exit 1; }
  [ -f "$manifest" ] || { printf '%s\n' "manifest 不存在: $manifest" >&2; exit 1; }
  [ "$(stat -c '%a' "$batch_dir")" = '700' ] || { printf '%s\n' "批次目录权限不是 0700: $batch_dir" >&2; exit 1; }
  [ "$(stat -c '%a' "$manifest")" = '600' ] || { printf '%s\n' "manifest 权限不是 0600: $manifest" >&2; exit 1; }
  jq -e '.schema_name == "asset-agent.batch-manifest" and .schema_version == "2.0"' "$manifest" >/dev/null

  jq -r '.files[] | [.name, .records, .bytes, .sha256] | @tsv' "$manifest" |
  while IFS="$(printf '\t')" read -r file_name expected_records expected_bytes expected_sha256; do
    case "$file_name" in
      ''|*/*|*\\*) printf '%s\n' "manifest 包含不安全文件名: $file_name" >&2; exit 1 ;;
    esac
    shard="$batch_dir/$file_name"
    [ -f "$shard" ] || { printf '%s\n' "manifest 分片不存在: $shard" >&2; exit 1; }
    [ "$(stat -c '%a' "$shard")" = '600' ] || { printf '%s\n' "分片权限不是 0600: $shard" >&2; exit 1; }
    actual_records=$(wc -l < "$shard" | tr -d ' ')
    actual_bytes=$(wc -c < "$shard" | tr -d ' ')
    actual_sha256=$(sha256_file "$shard")
    [ "$actual_records" = "$expected_records" ] || { printf '%s\n' "记录数不匹配: $shard" >&2; exit 1; }
    [ "$actual_bytes" = "$expected_bytes" ] || { printf '%s\n' "字节数不匹配: $shard" >&2; exit 1; }
    [ "$actual_sha256" = "$expected_sha256" ] || { printf '%s\n' "SHA-256 不匹配: $shard" >&2; exit 1; }
  done
}

scan_and_get_batch() {
  summary_file=$1
  shift
  "$installed_agent" "$@" -output "$output_root" | tee "$summary_file" >&2
  batch_dir=$(sed -n 's/^Output: //p' "$summary_file" | tail -n 1)
  [ -n "$batch_dir" ] || { printf '%s\n' "摘要缺少 Output: $summary_file" >&2; exit 1; }
  printf '%s\n' "$batch_dir"
}

expect_usage_error() {
  set +e
  "$installed_agent" "$@" >/dev/null 2>&1
  code=$?
  set -e
  [ "$code" -eq 2 ] || { printf '%s\n' "旧语法未返回 2: $* ($code)" >&2; exit 1; }
}

"$installed_agent" version > "$work_dir/version.json"
"$installed_agent" doctor > "$work_dir/doctor.json"
"$installed_agent" modules > "$work_dir/modules.json"

host_batch=$(scan_and_get_batch "$work_dir/host.txt" -host)
network_batch=$(scan_and_get_batch "$work_dir/network.txt" -network)
process_batch=$(scan_and_get_batch "$work_dir/process.txt" -process)
port_batch=$(scan_and_get_batch "$work_dir/port.txt" -port)
connection_batch=$(scan_and_get_batch "$work_dir/connection.txt" -connection)
service_batch=$(scan_and_get_batch "$work_dir/service.txt" -service)
package_batch=$(scan_and_get_batch "$work_dir/package.txt" -package)
container_batch=$(scan_and_get_batch "$work_dir/container.txt" -container)
multi_batch=$(scan_and_get_batch "$work_dir/multi.txt" -network -port)
snapshot_batch=$(scan_and_get_batch "$work_dir/snapshot.txt" scan)

[ "$(stat -c '%a' "$output_root")" = '700' ]
[ "$(stat -c '%a' "$output_root/inbox")" = '700' ]
for batch_dir in "$host_batch" "$network_batch" "$process_batch" "$port_batch" "$connection_batch" "$service_batch" "$package_batch" "$container_batch" "$multi_batch" "$snapshot_batch"; do
  validate_batch "$batch_dir"
done

latest_snapshot=$(find "$output_root/inbox" -mindepth 1 -maxdepth 1 -type d -name 'snapshot-*' | sort | tail -n 1)
[ "$latest_snapshot" = "$snapshot_batch" ] || { printf '%s\n' '最新正式快照目录与命令返回值不一致。' >&2; exit 1; }
jq -e '[.modules[].module] | sort == ["connection","container","host","network","package","port","process","service"]' "$snapshot_batch/manifest.json" >/dev/null
jq -e '.batch_type == "module" and .requested_module == "multi"' "$multi_batch/manifest.json" >/dev/null
jq -e '.batch_type == "snapshot" and .requested_module == "all"' "$snapshot_batch/manifest.json" >/dev/null

expect_usage_error host scan
expect_usage_error all scan
expect_usage_error scan host
expect_usage_error scan socket
expect_usage_error -host -o x

if find "$output_root/inbox" -mindepth 1 -maxdepth 1 -type d -name '.partial-*' | grep -q .; then
  printf '%s\n' '验证后仍存在 .partial 批次。' >&2
  exit 1
fi

printf '%s\n' '真实 Linux 协议 2.0 验证通过。'
printf '%s\n' "验证产物保留在: $work_dir"
