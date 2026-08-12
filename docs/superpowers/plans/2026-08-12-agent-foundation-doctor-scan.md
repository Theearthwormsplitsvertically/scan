# Linux Asset Agent Foundation, Doctor, and Scan Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first independently testable Linux asset-agent milestone with `version`, `doctor`, and one-shot `scan`, producing local JSON for host, network, process, TCP/UDP socket, and socket-to-process evidence.

**Architecture:** A small CLI delegates to an orchestration layer. Collectors read Linux facts through narrow filesystem and network interfaces, return normalized model objects plus independent status records, and never terminate the whole scan because one domain fails. Pure parsers remain platform-neutral so tests run on Windows; Linux-only wiring is selected with build tags and cross-compiled into one static executable.

**Tech Stack:** Go 1.26.5, Go standard library, fixture-driven `testing` package tests, Linux `/proc`, `/sys`, and `net.Interfaces`.

## Global Constraints

- Target Linux only; the Agent automatically identifies distribution, kernel, and architecture.
- The PoC may run as `root`, but every collector is read-only.
- Do not listen on a network port, upload data, access the Internet, mutate host state, install software, load kernel modules, or execute commands through a shell.
- Do not read or emit process environment variables; redact passwords, tokens, secrets, authorization values, cookies, private keys, and URL credentials from command lines.
- Do not recursively scan disks. Restrict reads to explicitly allowed `/etc`, `/proc`, and `/sys` facts.
- A collector failure is represented as `partial`, `degraded`, `failed`, or `timeout` and must not abort successful collectors.
- Output schema version is exactly `1.0`; explicit `scan --output <path>` files are never auto-deleted.
- Directories managed by later daemon milestones use `0700`; report files use `0600`.
- Planned incremental scanning remains 72 hours and ordinary full scanning remains 168 hours, but scheduling and `watch` are outside this milestone.
- This milestone has no CMDB, receiver, remote-control, or downstream-system integration.

---

### Task 1: Go Module and CLI Contract

**Files:**
- Create: `go.mod`
- Create: `cmd/asset-agent/main.go`
- Create: `internal/buildinfo/version.go`
- Create: `internal/cli/run.go`
- Test: `internal/cli/run_test.go`

**Interfaces:**
- Produces: `cli.Run(ctx context.Context, args []string, stdout, stderr io.Writer, runtime agent.Runtime) int`.
- Produces: build variables `buildinfo.Version`, `buildinfo.Commit`, and `buildinfo.BuildTime` with safe development defaults.
- Consumes: `agent.Runtime`, introduced initially as the exact interface below and implemented in later tasks.

- [ ] **Step 1: Initialize the module and write the failing CLI tests**

Create `go.mod` with module `github.com/Theearthwormsplitsvertically/scan` and Go language version `1.26.5`. In `internal/cli/run_test.go`, use an in-memory fake runtime and assert literal behavior:

```go
func TestRunVersionWritesMachineReadableVersion(t *testing.T) {
    var stdout, stderr bytes.Buffer
    code := Run(context.Background(), []string{"version"}, &stdout, &stderr, fakeRuntime{})
    if code != 0 { t.Fatalf("code = %d", code) }
    if !strings.Contains(stdout.String(), `"name":"asset-agent"`) { t.Fatalf("stdout = %s", stdout.String()) }
    if stderr.Len() != 0 { t.Fatalf("stderr = %s", stderr.String()) }
}

func TestRunRejectsUnknownCommand(t *testing.T) {
    var stdout, stderr bytes.Buffer
    code := Run(context.Background(), []string{"destroy"}, &stdout, &stderr, fakeRuntime{})
    if code != 2 { t.Fatalf("code = %d", code) }
    if !strings.Contains(stderr.String(), "unknown command") { t.Fatalf("stderr = %s", stderr.String()) }
}
```

- [ ] **Step 2: Run the tests and verify the package fails to compile because `Run` and `agent.Runtime` do not exist**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/cli`

Expected: FAIL with undefined symbols from the missing CLI contract.

- [ ] **Step 3: Implement the minimal command router**

Define the initial boundary in `internal/agent/runtime.go`:

```go
type Runtime interface {
    Doctor(context.Context) (model.DoctorReport, error)
    Scan(context.Context) (model.Snapshot, error)
}
```

Parse exactly `version`, `doctor`, and `scan`; reserve `watch` with an explicit “not implemented in this milestone” error and exit code 2. Encode `version` as JSON. `main.go` passes `os.Args[1:]`, `os.Stdout`, and `os.Stderr` to `cli.Run`.

- [ ] **Step 4: Run the CLI tests and all current tests**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./...`

Expected: PASS with no warnings.

- [ ] **Step 5: Commit the CLI contract**

```powershell
git add -- go.mod cmd/asset-agent internal/buildinfo internal/cli internal/agent/runtime.go
git commit -m "feat: add asset agent CLI contract"
```

### Task 2: Stable Output Model and Atomic JSON Reporter

**Files:**
- Create: `internal/model/model.go`
- Create: `internal/report/json.go`
- Test: `internal/report/json_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`

**Interfaces:**
- Produces: `report.WriteJSON(w io.Writer, value any) error`.
- Produces: `report.WriteJSONFile(path string, value any) error`, using a same-directory temporary file, `0600`, flush, close, and rename.
- Produces: normalized `model.DoctorReport`, `model.Snapshot`, `model.CollectorStatus`, `model.Process`, `model.Socket`, and `model.Relationship` structures with the schema field names in the design.

- [ ] **Step 1: Write failing reporter tests**

Assert that writer output decodes as JSON, ends with one newline, contains `"schema_version":"1.0"`, and never HTML-escapes ordinary strings. For file output, pre-create a temporary directory, call `WriteJSONFile`, assert mode permission bits equal `0600` on non-Windows platforms, and assert no `.tmp-*` file remains.

- [ ] **Step 2: Run reporter tests and verify failure due to missing package**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/report -run Test -v`

Expected: FAIL because `report.WriteJSON` and `report.WriteJSONFile` are undefined.

- [ ] **Step 3: Implement the output model and reporter**

Use these exact status values:

```go
const (
    StatusOK       Status = "ok"
    StatusPartial  Status = "partial"
    StatusDegraded Status = "degraded"
    StatusFailed   Status = "failed"
    StatusTimeout  Status = "timeout"
    StatusUnsupported Status = "unsupported"
)
```

`Snapshot` must serialize every design top-level key, using empty arrays instead of `null`. `WriteJSONFile` must create its temporary file inside `filepath.Dir(path)`, call `Chmod(0600)`, `Sync`, `Close`, then `Rename`; on any error it closes and removes only the exact temporary file it created.

- [ ] **Step 4: Add `scan --output <path>` parsing tests and implementation**

Test `scan` with omitted `--output` writing JSON to stdout, `scan --output -` writing JSON to stdout, a real temporary path writing a JSON file, missing option value returning exit code 2, and an unknown option returning exit code 2.

- [ ] **Step 5: Run all tests and commit**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./...`

Expected: PASS.

```powershell
git add -- internal/model internal/report internal/cli
git commit -m "feat: add snapshot model and atomic JSON output"
```

### Task 3: Linux Capability Detection and `doctor`

**Files:**
- Create: `internal/capability/detect.go`
- Create: `internal/capability/parse.go`
- Test: `internal/capability/parse_test.go`
- Create: `internal/platform/platform_linux.go`
- Create: `internal/platform/platform_unsupported.go`
- Create: `internal/platform/files.go`
- Modify: `cmd/asset-agent/main.go`

**Interfaces:**
- Produces: `capability.Detect(ctx context.Context, root platform.Root) model.DoctorReport`.
- Produces: `capability.ParseOSRelease([]byte) map[string]string` and `capability.ParseSelfStatus([]byte) SelfStatus`.
- Produces: `platform.NewRuntime() agent.Runtime`; Linux uses root `/`, unsupported platforms return a clear target-platform error.

- [ ] **Step 1: Write failing pure-parser tests**

Use literal fixtures to verify quoted os-release values, escaped quotes, blank lines, comments, malformed lines, `CapEff`, `Uid`, and `Gid`. A malformed optional field must not erase valid fields.

- [ ] **Step 2: Run the parser tests and observe the expected missing-function failure**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/capability -v`

Expected: FAIL because parsers are undefined.

- [ ] **Step 3: Implement parser and read-only detector**

Read only these fixed paths: `/etc/os-release`, `/proc/sys/kernel/osrelease`, `/proc/self/status`, `/proc/1/comm`, `/proc/1/cgroup`, `/proc/filesystems`, `/proc/self/mountinfo`, `/sys/fs/cgroup/cgroup.controllers`, `/sys/fs/selinux/enforce`, `/sys/module/apparmor/parameters/enabled`, `/var/run/docker.sock`, and `/run/containerd/containerd.sock`. Report every requested capability with `ok`, `degraded`, or `unsupported`; declare SockDiag and Process Connector `degraded` with the explicit `/proc` fallback until their direct probes are added in a later milestone.

- [ ] **Step 4: Add a fixture-root detector test**

Build a temporary Linux-like directory, populate selected fixed files, and assert exact output for distribution, kernel, root identity, init, cgroup v2, Docker socket absence, SELinux absence, and AppArmor presence. Missing optional files must produce capability records rather than a top-level error.

- [ ] **Step 5: Wire `doctor` through the platform runtime and run tests**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./...`

Expected: PASS on Windows using pure parsers and unsupported-platform wiring.

- [ ] **Step 6: Commit capability detection**

```powershell
git add -- internal/capability internal/platform cmd/asset-agent/main.go
git commit -m "feat: add Linux capability doctor"
```

### Task 4: Host and Network Collection

**Files:**
- Create: `internal/collect/host/collector.go`
- Create: `internal/collect/host/parse.go`
- Test: `internal/collect/host/parse_test.go`
- Create: `internal/collect/network/collector.go`
- Create: `internal/collect/network/routes.go`
- Test: `internal/collect/network/routes_test.go`

**Interfaces:**
- Produces: `host.Collect(ctx context.Context, root platform.Root) (model.Host, model.CollectorStatus)`.
- Produces: `network.Collect(ctx context.Context, root platform.Root, interfaces network.InterfaceSource) ([]model.NetworkInterface, []model.Address, []model.Route, model.CollectorStatus)`.
- Consumes: fixed Linux fact files through `platform.Root`; no command execution.

- [ ] **Step 1: Write failing host parser tests**

Test `/proc/meminfo` values in KiB, the first CPU model from both x86 `model name` and ARM `Hardware`, machine-id trimming, missing DMI values, and a truncated numeric line. Expected bytes are hand-derived literals such as `MemTotal: 2048 kB` becoming `2097152`.

- [ ] **Step 2: Run host tests, implement minimal parsers and collector, then rerun to green**

Run RED and GREEN with:

`D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/collect/host -v`

The collector returns `partial` when optional DMI is unavailable and `failed` only when core OS and kernel facts are both unavailable.

- [ ] **Step 3: Write failing IPv4 route parser tests**

Use `/proc/net/route` fixture lines for a default route and a `192.168.1.0/24` route. Assert little-endian address conversion, prefix length from the mask, interface, gateway, metric, and malformed-line skipping with a partial warning.

- [ ] **Step 4: Implement interface/address collection and route parsing**

Wrap `net.Interfaces` and each interface's `Addrs` behind `InterfaceSource` so tests use deterministic real-shaped values. Preserve IPv4 and IPv6 CIDRs, interface index, name, MTU, MAC, and flags. DNS output is a SHA-256 digest of normalized non-comment `/etc/resolv.conf` directives, never the raw file.

- [ ] **Step 5: Run both collector packages and commit**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/collect/host ./internal/collect/network -v`

Expected: PASS.

```powershell
git add -- internal/collect/host internal/collect/network
git commit -m "feat: collect Linux host and network facts"
```

### Task 5: Process Identity and Command-Line Redaction

**Files:**
- Create: `internal/security/redact.go`
- Test: `internal/security/redact_test.go`
- Create: `internal/collect/process/collector.go`
- Create: `internal/collect/process/parse.go`
- Test: `internal/collect/process/parse_test.go`

**Interfaces:**
- Produces: `security.RedactArgs(args []string) []string` without mutating input.
- Produces: `process.ParseStat([]byte) (Stat, error)` and `process.Collect(ctx context.Context, root platform.Root, bootID string) ([]model.Process, model.CollectorStatus)`.

- [ ] **Step 1: Write redaction tests before production code**

Cover `--password=value`, `--token value`, `Authorization: Bearer value`, `API_KEY=value`, case-insensitive keys, URL user-info, an ordinary `--port=8080`, and preservation of argument count. Assert secret literals do not occur anywhere in the joined result.

- [ ] **Step 2: Run the redaction tests to RED, implement, and rerun to GREEN**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/security -v`

Expected final result: PASS.

- [ ] **Step 3: Write process parser tests**

Use a `/proc/<pid>/stat` line whose command name contains spaces and parentheses. Assert PID, command, state, PPID, and field 22 starttime. Test NUL-separated cmdline, status UID/GID, cgroup lines, and malformed stat rejection.

- [ ] **Step 4: Implement bounded read-only process collection**

Enumerate only numeric direct children of `/proc`. Read `stat`, `status`, `cmdline` with a 1 MiB maximum, and fixed symlinks `exe`, `cwd`, `root`, `ns/mnt`, `ns/net`, and `ns/pid`. Construct identity as `<boot-id>:<pid>:<starttime>`. Do not open `environ`. If a process exits mid-read, retry that PID once; if it still races, keep a partial collector warning and continue.

- [ ] **Step 5: Run process and security tests and commit**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/security ./internal/collect/process -v`

Expected: PASS.

```powershell
git add -- internal/security internal/collect/process
git commit -m "feat: collect and redact Linux processes"
```

### Task 6: TCP/UDP Socket Parsing and PID Correlation

**Files:**
- Create: `internal/collect/socket/parse.go`
- Create: `internal/collect/socket/collector.go`
- Create: `internal/collect/socket/correlate.go`
- Test: `internal/collect/socket/parse_test.go`
- Test: `internal/collect/socket/correlate_test.go`

**Interfaces:**
- Produces: `socket.ParseProcNet(r io.Reader, protocol string, family int, netns string) ([]model.Socket, []error)`.
- Produces: `socket.Collect(ctx context.Context, root platform.Root, processes []model.Process) ([]model.Socket, []model.Relationship, model.CollectorStatus)`.

- [ ] **Step 1: Write failing parser tests with hand-checked endpoints**

Assert `0100007F:1F90` becomes `127.0.0.1:8080`, `00000000:0000` becomes `0.0.0.0:0`, IPv6 loopback becomes `[::1]:443`, TCP state `0A` becomes `LISTEN`, TCP state `01` becomes `ESTABLISHED`, UDP state remains represented, inode is preserved, and malformed rows are skipped with errors.

- [ ] **Step 2: Run parser tests to RED, implement parsing, and rerun to GREEN**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/collect/socket -run Parse -v`

Expected final result: PASS.

- [ ] **Step 3: Write failing correlation tests**

Provide a fake `Readlink` map containing `socket:[12345]`, a non-socket descriptor, a vanished descriptor, and two PIDs sharing inode `12345`. Assert both process identities appear on the socket and two exact `socket_process` relationships are emitted with source `/proc/<pid>/fd`, collector `socket`, and confidence `exact`.

- [ ] **Step 4: Implement bounded inode discovery and relationship creation**

Inspect only direct entries below `/proc/<pid>/fd`; accept only symlink targets matching `socket:[<decimal>]`; deduplicate PID/inode pairs; sort sockets, PIDs, and relationships deterministically. Read `/proc/self/ns/net` for the network namespace identity and parse `/proc/net/tcp`, `tcp6`, `udp`, and `udp6` independently so one unavailable file yields `partial`, not total failure.

- [ ] **Step 5: Run socket tests and commit**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/collect/socket -v`

Expected: PASS.

```powershell
git add -- internal/collect/socket
git commit -m "feat: correlate Linux sockets to processes"
```

### Task 7: One-Shot Scan Orchestration and Failure Isolation

**Files:**
- Create: `internal/agent/scan.go`
- Test: `internal/agent/scan_test.go`
- Modify: `internal/platform/platform_linux.go`
- Modify: `internal/model/model.go`

**Interfaces:**
- Produces: concrete runtime method `Scan(context.Context) (model.Snapshot, error)`.
- Consumes: capability, host, network, process, and socket collectors through narrow function fields in `agent.Dependencies`.

- [ ] **Step 1: Write failing orchestration tests**

Use real in-memory collector functions, not call-count mocks. Test that a host success plus socket failure returns a snapshot containing the host, empty socket array, both collector statuses, schema `1.0`, non-empty scan ID, UTC timestamps, and no top-level error. Separately test context cancellation before collection returning `context.Canceled` and no report.

- [ ] **Step 2: Run agent tests and verify the missing orchestrator failure**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/agent -v`

Expected: FAIL because the scan orchestrator is undefined.

- [ ] **Step 3: Implement sequential first-milestone orchestration**

Run capability and host first, then network and process, then socket with process results. Wrap each collector in its own timeout (`15s` host/network, `30s` process/socket), recover a collector panic into `failed`, keep array fields non-null, sort output deterministically, and measure total wall duration plus Go heap allocation before and after the scan.

- [ ] **Step 4: Run all tests and commit**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./... -count=1`

Expected: PASS without cached results.

```powershell
git add -- internal/agent internal/platform internal/model
git commit -m "feat: orchestrate isolated one-shot scans"
```

### Task 8: Linux Build, Usage Documentation, and PoC Handoff

**Files:**
- Create: `README.md`
- Create: `scripts/verify-linux.sh`
- Test: `internal/cli/run_test.go`

**Interfaces:**
- Produces: `dist/asset-agent-linux-amd64` as a local build artifact excluded by `.gitignore`.
- Produces: a read-only Linux verification script that runs `version`, `doctor`, and `scan`, then compares selected JSON fields with `uname`, `/etc/os-release`, and `ss` without mutating the host.

- [ ] **Step 1: Add a failing CLI serialization test for non-null arrays and deterministic JSON**

Run the same fake snapshot twice and assert byte-identical JSON after neutralizing scan timestamps/ID in the fixture. Assert `processes`, `sockets`, `relationships`, and every reserved future-domain array encode as `[]`.

- [ ] **Step 2: Run the test to RED, normalize slice initialization and sorting, and rerun to GREEN**

Run: `D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./internal/cli -v`

Expected final result: PASS.

- [ ] **Step 3: Write operator documentation and read-only verification commands**

Document exact commands:

```bash
sudo ./asset-agent version
sudo ./asset-agent doctor | jq .
sudo ./asset-agent scan --output ./snapshot.json
jq '.host, .addresses, [.sockets[] | select(.state == "LISTEN")]' ./snapshot.json
ss -lntup
```

State plainly that `watch`, service/package/container ownership, executable hashing, dynamic libraries, Nginx deep collection, retention, and scheduling are later milestones.

- [ ] **Step 4: Cross-compile and verify the binary format**

Run in PowerShell:

```powershell
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
D:\go\go1.26.5.windows-amd64\go\bin\go.exe build -trimpath -ldflags '-s -w' -o dist/asset-agent-linux-amd64 ./cmd/asset-agent
```

Expected: exit code 0 and a non-empty Linux amd64 executable.

- [ ] **Step 5: Run final static checks and tests**

Run:

```powershell
D:\go\go1.26.5.windows-amd64\go\bin\go.exe fmt ./...
D:\go\go1.26.5.windows-amd64\go\bin\go.exe vet ./...
D:\go\go1.26.5.windows-amd64\go\bin\go.exe test ./... -count=1
git diff --check
```

Expected: every command exits 0 with no warnings from project code.

- [ ] **Step 6: Commit the milestone documentation**

```powershell
git add -- README.md scripts/verify-linux.sh .gitignore internal/cli/run_test.go
git commit -m "docs: add Linux agent PoC verification guide"
```

