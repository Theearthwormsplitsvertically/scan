# Host Minimal Fields Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `host` 模块收敛为十个批准字段，删除无用硬件采集，并用明确的身份降级规则阻止无身份主机记录污染 CMDB。

**Architecture:** 保留现有 CLI、动态模块注册表、Linux Provider 和批次输出边界，只缩减 `model.Host` 及 Host Provider 的事实集合。Host 模块负责稳定 ID、置信度和统一记录转换；`network`、`process` 继续通过相同的内部 `model.Host` 依赖取得稳定 `host_id` 和 `boot_id`。

**Tech Stack:** Go 1.26、标准库、procfs/sysfs、现有 Provider/Module/Batch 协议、Go testing。

## Global Constraints

- `host.attributes` 只允许：`hostname`、`distribution_name`、`distribution_id`、`distribution_version`、`kernel_release`、`architecture`、`memory_total_bytes`、`machine_id`、`boot_id`、`dmi_uuid`。
- 停止采集和输出：`id`、`vendor`、`model`、`cpu_model`、`cpu_count`；Host 记录顶层 `version` 和 `vendor` 保持空值。
- 不读取 `/proc/cpuinfo`、`/sys/class/dmi/id/sys_vendor`、`/sys/class/dmi/id/product_name`，不调用 `runtime.NumCPU()`。
- `boot_id` 不参与稳定主机 ID。
- Machine ID 与 DMI UUID 都存在时置信度为 `exact`；只有一个时为 `strong`；只有 hostname 时为 `inferred` 且非权威；三者都没有时不发布 Host 记录。
- DMI UUID 缺失但 Machine ID 存在时不能仅因此降级。
- 批次协议、manifest、JSONL 分片和其他模块的公共记录结构不改变。
- 采用 TDD：每项生产行为必须先有失败测试，再写最小实现。
- 每个实现提交完成后推送 `codex/host-minimal-fields`；最终验证后合并并推送 `main`。
- 保留工作区现有未跟踪二进制和中文设计文档，不纳入提交。

---

## File Map

- Modify: `internal/model/model.go` — 把 `Host` 缩减为十个强类型事实字段。
- Modify: `internal/collect/host/collector.go` — 只读取批准数据源并计算完整性状态。
- Modify: `internal/collect/host/parse.go` — 保留内存解析，删除 CPU 型号解析。
- Create: `internal/collect/host/collector_test.go` — 覆盖最小采集、DMI 可选和身份失败。
- Modify: `internal/collect/host/parse_test.go` — 删除已移除 CPU 解析测试。
- Modify: `internal/modules/host/module.go` — 生成最小属性、稳定 ID和三档置信度。
- Modify: `internal/modules/host/module_test.go` — 锁定公共字段集合和身份规则。
- Modify: `internal/modules/network/module_test.go` — 把 Host 依赖夹具迁移到新模型。
- Modify: `internal/modules/process/module_test.go` — 把 Host 依赖夹具迁移到新模型并继续验证 Boot ID。
- Modify: `README.md` — 更新当前 Host 能力和字段说明。

---

## Execution Preflight

执行阶段先调用 `superpowers:using-git-worktrees`。当前仓库已约定使用被忽略的 `.worktrees/`，从最新 `main` 创建 `.worktrees/host-minimal-fields` 和分支 `codex/host-minimal-fields`，然后运行：

```powershell
go mod download
go test ./... -count=1
```

只有基线测试退出码为 0 才进入 Task 1；如果失败，停止并报告基线故障，不把它归因于本次改动。

---

### Task 1: Implement the minimal Host fact and record contract

**Files:**
- Modify: `internal/model/model.go:94-111`
- Modify: `internal/collect/host/collector.go:1-71`
- Modify: `internal/collect/host/parse.go:1-51`
- Create: `internal/collect/host/collector_test.go`
- Modify: `internal/collect/host/parse_test.go:1-25`
- Modify: `internal/modules/host/module.go:44-97`
- Modify: `internal/modules/host/module_test.go:1-87`
- Modify: `internal/modules/network/module_test.go:31-33`
- Modify: `internal/modules/process/module_test.go:36-38`

**Interfaces:**
- Consumes: `platform.Root.ReadFile(path string, maximum int64) ([]byte, error)`、`provider.HostProvider.Collect(context.Context) (model.Host, model.CollectorStatus)`、`coremodule.StableRecordID(recordType string, parts ...string) string`。
- Produces: 最小 `model.Host`；保留 `hostmodule.RecordID(host model.Host) string`；新增包内 `identityConfidence(host model.Host) string`；Host `AssetRecord.Attributes` 的十字段契约。

- [ ] **Step 1: Add failing collector contract tests**

创建 `internal/collect/host/collector_test.go`，使用真实 `platform.Root` fixture，不 mock 采集逻辑：

```go
package host

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/Theearthwormsplitsvertically/scan/internal/model"
	"github.com/Theearthwormsplitsvertically/scan/internal/platform"
)

func TestCollectReturnsMinimalHostFactsAndAcceptsMissingDMI(t *testing.T) {
	rootPath := t.TempDir()
	writeHostFixture(t, rootPath, "/etc/os-release", "PRETTY_NAME=\"Example Linux 1\"\nID=example\nVERSION_ID=1\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/osrelease", "6.8.0-test\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/hostname", "server-1\n")
	writeHostFixture(t, rootPath, "/etc/machine-id", "machine-1\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/random/boot_id", "boot-1\n")
	writeHostFixture(t, rootPath, "/proc/meminfo", "MemTotal: 2048 kB\n")

	got, status := Collect(t.Context(), platform.NewRoot(rootPath))
	if status.Status != model.StatusOK {
		t.Fatalf("status = %q, errors = %v", status.Status, status.Errors)
	}
	want := model.Host{
		Hostname: "server-1", DistributionName: "Example Linux 1",
		DistributionID: "example", DistributionVersion: "1",
		KernelRelease: "6.8.0-test", Architecture: runtime.GOARCH,
		MemoryTotalBytes: 2_097_152, MachineID: "machine-1", BootID: "boot-1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host = %#v, want %#v", got, want)
	}
}

func TestCollectFailsWhenNoHostIdentityCanBeEstablished(t *testing.T) {
	rootPath := t.TempDir()
	writeHostFixture(t, rootPath, "/etc/os-release", "PRETTY_NAME=\"Example Linux 1\"\nID=example\nVERSION_ID=1\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/osrelease", "6.8.0-test\n")
	writeHostFixture(t, rootPath, "/proc/sys/kernel/random/boot_id", "boot-1\n")
	writeHostFixture(t, rootPath, "/proc/meminfo", "MemTotal: 2048 kB\n")

	_, status := Collect(t.Context(), platform.NewRoot(rootPath))
	if status.Status != model.StatusFailed || status.Objects != 0 {
		t.Fatalf("status = %+v", status)
	}
}

func writeHostFixture(t *testing.T, rootPath, absolutePath, content string) {
	t.Helper()
	path := filepath.Join(rootPath, filepath.FromSlash(absolutePath[1:]))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Add failing model and module contract tests**

在 `internal/modules/host/module_test.go` 中用以下字段集合断言替换宽松的现有记录断言：

```go
func minimalHost() model.Host {
	return model.Host{
		Hostname: "server-1", DistributionName: "Example Linux 1",
		DistributionID: "example", DistributionVersion: "1",
		KernelRelease: "6.8.0-test", Architecture: "amd64",
		MemoryTotalBytes: 2_097_152, MachineID: "machine", BootID: "boot-1", DMIUUID: "dmi",
	}
}

func TestHostModulePublishesOnlyApprovedAttributes(t *testing.T) {
	result := collectHostForTest(t, minimalHost(), model.StatusOK)
	if len(result.Data.Records) != 1 {
		t.Fatalf("records = %+v", result.Data.Records)
	}
	record := result.Data.Records[0]
	wantKeys := []string{
		"architecture", "boot_id", "distribution_id", "distribution_name",
		"distribution_version", "dmi_uuid", "hostname", "kernel_release",
		"machine_id", "memory_total_bytes",
	}
	gotKeys := make([]string, 0, len(record.Attributes))
	for key := range record.Attributes {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("attribute keys = %v, want %v", gotKeys, wantKeys)
	}
	if record.Version != "" || record.Vendor != "" {
		t.Fatalf("duplicate top-level fields: version=%q vendor=%q", record.Version, record.Vendor)
	}
}
```

增加身份表驱动测试：

```go
func TestHostModuleIdentityConfidence(t *testing.T) {
	both := minimalHost()
	machineOnly := minimalHost()
	machineOnly.DMIUUID = ""
	dmiOnly := minimalHost()
	dmiOnly.MachineID = ""
	hostnameFallback := minimalHost()
	hostnameFallback.MachineID = ""
	hostnameFallback.DMIUUID = ""
	noIdentity := minimalHost()
	noIdentity.MachineID = ""
	noIdentity.DMIUUID = ""
	noIdentity.Hostname = ""

	tests := []struct {
		name       string
		host       model.Host
		confidence string
		status     model.Status
		records    int
	}{
		{"machine and dmi", both, "exact", model.StatusComplete, 1},
		{"machine only", machineOnly, "strong", model.StatusComplete, 1},
		{"dmi only", dmiOnly, "strong", model.StatusComplete, 1},
		{"hostname fallback", hostnameFallback, "inferred", model.StatusPartial, 1},
		{"no identity", noIdentity, "", model.StatusFailed, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := collectHostForTest(t, test.host, model.StatusOK)
			if result.Data.Status != test.status || len(result.Data.Records) != test.records {
				t.Fatalf("result = %+v", result.Data)
			}
			if test.records == 1 && result.Data.Records[0].Confidence != test.confidence {
				t.Fatalf("confidence = %q", result.Data.Records[0].Confidence)
			}
			if test.status != model.StatusComplete && result.Data.Authoritative {
				t.Fatal("non-complete result is authoritative")
			}
		})
	}
}
```

增加反射测试，锁定 `model.Host` 不再包含旧字段：

```go
func TestHostModelContainsOnlyMinimalFacts(t *testing.T) {
	typ := reflect.TypeOf(model.Host{})
	want := []string{
		"Hostname", "DistributionName", "DistributionID", "DistributionVersion",
		"KernelRelease", "Architecture", "MachineID", "BootID", "DMIUUID", "MemoryTotalBytes",
	}
	got := make([]string, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		got[index] = typ.Field(index).Name
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Host fields = %v, want %v", got, want)
	}
}
```

测试辅助函数 `collectHostForTest` 必须创建真实 `provider.Set` 并调用 `New().Collect`，不能直接测试私有 map 构造逻辑：

```go
func collectHostForTest(t *testing.T, host model.Host, status model.Status) coremodule.Result {
	t.Helper()
	providers, err := provider.NewSet("linux", fakeHostProvider{
		host: host,
		status: model.CollectorStatus{
			Collector: "host", Status: status, Errors: []string{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return New().Collect(t.Context(), providers, coremodule.Request{})
}
```

- [ ] **Step 3: Update downstream fixtures so the intended model compiles everywhere**

把依赖测试中的旧 `ID` 字段替换为稳定身份事实：

```go
// internal/modules/network/module_test.go
"host": {Internal: model.Host{MachineID: "machine", DMIUUID: "dmi", Hostname: "server-1"}},

// internal/modules/process/module_test.go
"host": {Internal: model.Host{MachineID: "machine", DMIUUID: "dmi", BootID: "boot-1"}},
```

- [ ] **Step 4: Run the focused tests and verify RED**

Run:

```powershell
go test ./internal/collect/host ./internal/modules/host ./internal/modules/network ./internal/modules/process -count=1
```

Expected: FAIL because `model.Host` does not yet expose `DistributionName`、`DistributionVersion`、`KernelRelease`、`MemoryTotalBytes`, and the old module still outputs legacy attributes/confidence behavior.

- [ ] **Step 5: Replace `model.Host` with the minimal fact set**

在 `internal/model/model.go` 中使用：

```go
type Host struct {
	Hostname            string `json:"hostname,omitempty"`
	DistributionName    string `json:"distribution_name,omitempty"`
	DistributionID      string `json:"distribution_id,omitempty"`
	DistributionVersion string `json:"distribution_version,omitempty"`
	KernelRelease       string `json:"kernel_release,omitempty"`
	Architecture        string `json:"architecture,omitempty"`
	MachineID           string `json:"machine_id,omitempty"`
	BootID              string `json:"boot_id,omitempty"`
	DMIUUID              string `json:"dmi_uuid,omitempty"`
	MemoryTotalBytes     uint64 `json:"memory_total_bytes,omitempty"`
}
```

- [ ] **Step 6: Implement minimal Linux collection and completeness rules**

修改 `internal/collect/host/collector.go`。必要字段和替代身份字段使用两个不同的读取闭包：

```go
readRequired := func(path string) string {
	data, err := root.ReadFile(path, factLimit)
	if err != nil {
		status.Errors = append(status.Errors, path+": "+err.Error())
		return ""
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		status.Errors = append(status.Errors, path+": empty value")
	}
	return value
}

readIdentity := func(path string) string {
	data, err := root.ReadFile(path, factLimit)
	if err != nil {
		if !os.IsNotExist(err) {
			status.Errors = append(status.Errors, path+": "+err.Error())
		}
		return ""
	}
	return strings.TrimSpace(string(data))
}
```

字段赋值和最终状态使用确定规则：

```go
result := model.Host{Architecture: runtime.GOARCH}

if data, err := root.ReadFile("/etc/os-release", factLimit); err == nil {
	values := capability.ParseOSRelease(data)
	result.DistributionName = strings.TrimSpace(values["PRETTY_NAME"])
	result.DistributionID = strings.TrimSpace(values["ID"])
	result.DistributionVersion = strings.TrimSpace(values["VERSION_ID"])
	for name, value := range map[string]string{
		"distribution_name": result.DistributionName,
		"distribution_id": result.DistributionID,
		"distribution_version": result.DistributionVersion,
	} {
		if value == "" {
			status.Errors = append(status.Errors, "/etc/os-release: missing "+name)
		}
	}
} else {
	status.Errors = append(status.Errors, "/etc/os-release: "+err.Error())
}

result.KernelRelease = readRequired("/proc/sys/kernel/osrelease")
result.Hostname = readRequired("/proc/sys/kernel/hostname")
result.MachineID = readIdentity("/etc/machine-id")
result.BootID = readRequired("/proc/sys/kernel/random/boot_id")
result.DMIUUID = readIdentity("/sys/class/dmi/id/product_uuid")
if data, err := root.ReadFile("/proc/meminfo", factLimit); err == nil {
	result.MemoryTotalBytes = ParseMemoryBytes(data)
	if result.MemoryTotalBytes == 0 {
		status.Errors = append(status.Errors, "/proc/meminfo: missing or invalid MemTotal")
	}
} else {
	status.Errors = append(status.Errors, "/proc/meminfo: "+err.Error())
}

hasStableIdentity := result.MachineID != "" || result.DMIUUID != ""
if !hasStableIdentity {
	status.Errors = append(status.Errors, "stable host identity unavailable")
}
switch {
case !hasStableIdentity && result.Hostname == "":
	status.Status = model.StatusFailed
	status.Objects = 0
case len(status.Errors) > 0:
	status.Status = model.StatusPartial
	status.Objects = 1
default:
	status.Status = model.StatusOK
	status.Objects = 1
}
```

`context` 已取消时仍按现有逻辑直接返回 `failed`。状态的开始时间、结束时间和耗时继续由现有 defer 填充。

随后完成以下删除和约束：

- 初始化时只设置 `Architecture: runtime.GOARCH`；
- 解析 os-release 到 `DistributionName`、`DistributionID`、`DistributionVersion`；
- 读取 hostname、kernel release、Machine ID、Boot ID、DMI UUID、MemTotal；
- 删除三个废弃数据源和 `runtime.NumCPU()`；
- 对 hostname、发行版三字段、kernel、architecture、memory、boot ID 做必要字段检查；
- Machine ID 与 DMI UUID 作为替代身份来源，DMI 单独缺失不追加错误；
- 无稳定身份但有 hostname 时追加 `stable host identity unavailable` 并返回 `partial`；
- 三个身份字段都为空时返回 `failed` 且 `Objects = 0`；其他可发布事实为 `Objects = 1`。

保留 `ParseMemoryBytes`，从 `parse.go` 删除 `ParseCPUModel`，从 `parse_test.go` 删除对应测试。

- [ ] **Step 7: Implement minimal Host record and identity downgrade**

`RecordID` 必须保持已有节点的 ID 算法稳定：

```go
func RecordID(host model.Host) string {
	machineID := strings.TrimSpace(host.MachineID)
	dmiUUID := strings.TrimSpace(host.DMIUUID)
	if machineID != "" || dmiUUID != "" {
		return coremodule.StableRecordID("host", machineID+":"+dmiUUID)
	}
	if hostname := strings.TrimSpace(host.Hostname); hostname != "" {
		return coremodule.StableRecordID("host", "", "", hostname)
	}
	return ""
}

func identityConfidence(host model.Host) string {
	machine := strings.TrimSpace(host.MachineID) != ""
	dmi := strings.TrimSpace(host.DMIUUID) != ""
	switch {
	case machine && dmi:
		return "exact"
	case machine || dmi:
		return "strong"
	case strings.TrimSpace(host.Hostname) != "":
		return "inferred"
	default:
		return ""
	}
}
```

`Collect` 的记录分支使用以下确定结构：

```go
recordID := RecordID(host)
confidence := identityConfidence(host)
if recordID == "" {
	status.Status = model.StatusFailed
	status.Errors = append(status.Errors, "无法建立主机身份")
	data := coremodule.NewModuleResult(
		"host", status, []string{"host"},
		[]model.AssetRecord{}, []model.RelationshipRecord{},
	)
	return coremodule.Result{Data: data, Internal: host}
}
if confidence == "inferred" && status.Status == model.StatusOK {
	status.Status = model.StatusPartial
	status.Errors = append(status.Errors, "仅能使用 hostname 推断主机身份")
}
name := strings.TrimSpace(host.Hostname)
if name == "" {
	name = recordID
}
attributes := map[string]any{
	"hostname": host.Hostname,
	"distribution_name": host.DistributionName,
	"distribution_id": host.DistributionID,
	"distribution_version": host.DistributionVersion,
	"kernel_release": host.KernelRelease,
	"architecture": host.Architecture,
	"memory_total_bytes": host.MemoryTotalBytes,
	"machine_id": host.MachineID,
	"boot_id": host.BootID,
	"dmi_uuid": host.DMIUUID,
}
```

在此基础上构造现有 `AssetRecord` 外层字段、Evidence 和 `NewModuleResult`。同时满足：

- `RecordID == ""` 时把状态改为 `failed`，追加身份错误并返回空记录；
- hostname 回退时把 `OK` 状态降为 `partial`；
- 记录名称优先 hostname，缺失时使用稳定 `record_id`，不再生成 `unknown-host`；
- 不设置顶层 `Version`、`Vendor`；
- `Attributes` 只构造 Global Constraints 中列出的十个键；
- Evidence 和记录使用相同置信度；
- `Internal` 继续返回完整的最小 `model.Host`，保证 Process 取得 Boot ID。

- [ ] **Step 8: Run focused and full tests to verify GREEN**

Run:

```powershell
go fmt ./...
go test ./internal/collect/host ./internal/modules/host ./internal/modules/network ./internal/modules/process -count=1
go test ./... -count=1
```

Expected: all packages PASS; no build errors reference removed Host fields.

- [ ] **Step 9: Verify legacy host fields and data sources are absent**

Run:

```powershell
rg -n 'CPUModel|CPUCount|MemoryBytes|\.Vendor|\.Model|\.OSVersion|\.Kernel|\.Distribution\b|host\.ID|"cpu_model"|"cpu_count"|"memory_bytes"|"vendor"|"model"|"os_version"|"kernel"' internal/collect/host internal/modules/host internal/model/model.go internal/modules/network/module_test.go internal/modules/process/module_test.go
rg -n '/proc/cpuinfo|sys_vendor|product_name|runtime\.NumCPU' internal/collect/host
```

Expected: both commands return no production or test matches for the removed Host contract. A non-zero `rg` exit caused solely by no matches is success.

- [ ] **Step 10: Commit and push the working code change**

```powershell
git add internal/model/model.go internal/collect/host internal/modules/host internal/modules/network/module_test.go internal/modules/process/module_test.go
git diff --cached --check
git commit -m "refactor: minimize host asset fields"
git push -u origin codex/host-minimal-fields
```

Expected: one code commit on `codex/host-minimal-fields`; existing unrelated untracked files remain unstaged.

---

### Task 2: Update current documentation and verify the release build

**Files:**
- Modify: `README.md:7-17`
- Modify: `README.md:99-113`

**Interfaces:**
- Consumes: Task 1 `host.attributes` ten-field contract and identity confidence behavior.
- Produces: Chinese operator documentation that distinguishes static Host facts from future 10-minute `resource` metrics.

- [ ] **Step 1: Run the documentation acceptance check and verify RED**

Run:

```powershell
$readme = git show HEAD:README.md
if ($readme -match '主机身份、发行版、内核、CPU、内存') { throw 'README still advertises CPU host collection' }
```

Expected: FAIL with `README still advertises CPU host collection`.

- [ ] **Step 2: Update the current module table and Host contract text**

把 Host 模块表述改为：

```markdown
| `host` | 主机名、发行版、内核、架构、内存总量、Machine ID、Boot ID、DMI UUID | 24 小时 | 无 |
```

在当前模块说明后补充：

```markdown
`host` 只保存主机身份、操作系统基线和内存总容量，不采集 CPU 型号、CPU 数量、硬件厂商或机器型号。CPU、内存、Load 和 Swap 的当前使用情况由后续 `resource` 模块按 10 分钟周期提供。

Machine ID 与 DMI UUID 用于生成稳定主机 ID，Boot ID 只表示当前启动实例，不参与稳定身份。DMI UUID 缺失但 Machine ID 存在时仍可形成完整结果；只有 hostname 可以使用时结果为非权威。
```

不要修改历史设计文档；新规格 `docs/superpowers/specs/2026-08-13-host-minimal-fields-design.md` 已明确覆盖旧的 Host 字段描述。

- [ ] **Step 3: Verify documentation and source terminology**

Run:

```powershell
$readme = Get-Content -Raw -Encoding UTF8 README.md
if ($readme -match '主机身份、发行版、内核、CPU、内存') { throw 'README still advertises CPU host collection' }
foreach ($term in @('Machine ID', 'Boot ID', 'DMI UUID', 'resource')) {
	if ($readme -notmatch [regex]::Escape($term)) { throw "README missing $term" }
}
git diff --check
```

Expected: exit 0.

- [ ] **Step 4: Run fresh quality gates and Linux cross-build**

Run:

```powershell
go fmt ./...
git diff --check
go vet ./...
go test ./... -count=1
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go build -buildvcs=false -trimpath -o dist/asset-agent-linux-amd64 ./cmd/asset-agent
go version -m dist/asset-agent-linux-amd64
```

Expected:

- `go vet` exit 0；
- all Go packages PASS；
- build exit 0；
- build metadata reports `GOOS=linux`、`GOARCH=amd64`、`CGO_ENABLED=0`。

- [ ] **Step 5: Commit and push documentation**

```powershell
git add README.md
git diff --cached --check
git commit -m "docs: document minimal host scan fields"
git push origin codex/host-minimal-fields
```

Expected: documentation commit pushed; `dist/` remains ignored.

---

### Task 3: Merge the verified feature into main

**Files:**
- No source file changes expected.
- Preserve: root `asset-agent-linux-amd64` and existing untracked Chinese documents.

**Interfaces:**
- Consumes: verified `codex/host-minimal-fields` branch from Tasks 1-2.
- Produces: synchronized local and remote `main` containing the minimal Host contract.

- [ ] **Step 1: Verify the feature worktree and branch are clean**

```powershell
git status --short --branch
git log --oneline main..codex/host-minimal-fields
```

Expected: no tracked changes; exactly the planned code and README commits are ahead of main.

- [ ] **Step 2: Re-run tests at the feature tip**

```powershell
go vet ./...
go test ./... -count=1
```

Expected: exit 0 and all packages PASS.

- [ ] **Step 3: Merge from the main worktree**

```powershell
git checkout main
git pull --ff-only origin main
git merge --no-ff codex/host-minimal-fields -m "merge: minimize host asset fields"
```

Expected: merge succeeds without overwriting untracked user files.

- [ ] **Step 4: Verify and push main**

```powershell
go vet ./...
go test ./... -count=1
git push origin main
git rev-parse main
git rev-parse origin/main
```

Expected: tests pass and both revisions are identical.

- [ ] **Step 5: Clean up only the merged local worktree and branch**

Use `superpowers:finishing-a-development-branch`. Confirm the feature worktree is clean and inside the repository before removing it, then delete only local `codex/host-minimal-fields`; do not delete unrelated untracked files or the remote feature branch unless separately requested.
