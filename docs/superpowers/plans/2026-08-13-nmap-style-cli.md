# Nmap Style Scanner CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将扫描器改造成由动态模块注册表驱动的 nmap 式短参数 CLI，支持单模块、组合模块和 `scan` 全量扫描，并把完整批次落盘、简洁摘要输出到终端。

**Architecture:** CLI 只负责解析 `-<module>`、`scan` 与 `-output`，模块名称来自运行时注册表；编排层接收显式选择集合，合并硬依赖图并保证每个模块只执行一次。报告层继续原子发布协议 2.0 批次，平台层提供默认输出根目录，终端展示使用不进入 JSON 的扫描统计。

**Tech Stack:** Go 1.22、标准库 `flag` 风格参数语义（自定义动态解析）、`text/tabwriter`、现有 Provider/Module/Report 协议、Go testing、POSIX shell、GitHub CLI。

## Global Constraints

- JSON 字段继续使用英文，schema version 保持 `2.0`，JSONL 分片、SHA-256、`manifest.json` 最后写入和原子改名语义不变。
- Linux 默认输出根目录必须是 `/var/lib/asset-agent/output`；`-output <path>` 只覆盖本次扫描。
- 单模块使用 `asset-agent -<module>`，组合扫描允许多个动态模块参数，全量扫描只使用 `asset-agent scan`。
- `scan` 不得与模块参数组合；参数错误返回 `2`，扫描或发布的致命错误返回 `1`，正式批次发布成功返回 `0`。
- 所有旧长命令和旧输出参数均不兼容：`host scan`、模块 `describe/status/schedule`、`all scan`、`scan host/all/socket`、`-o`、`--output`、`--output-dir` 必须返回 `2`。
- 用户明确选择的模块才发布记录；依赖模块的状态、覆盖范围和错误继续进入 manifest。
- 新注册模块必须自动获得 `-<module>` 参数并自动进入 `scan`，CLI 不得包含内置模块名 switch。
- 每个任务遵循 RED→GREEN→重构→定向测试→全量测试→代码审查→提交→推送；不得修改三个现有未跟踪用户文件。
- 完成后合并回 `main`，更新中文 README，并发布破坏性版本 `v0.4.0`。

---

## File Structure

- `internal/module/registry.go`：验证模块名，计算多目标或全量硬依赖拓扑计划。
- `internal/module/registry_test.go`：覆盖共享依赖去重、顺序稳定、全量动态发现、未知目标和保留名称。
- `internal/modules/register_test.go`：确认默认注册表仍包含五个模块且 `PlanAll` 动态覆盖全部模块。
- `internal/agent/runtime.go`：定义显式扫描选择和带终端计数的运行时返回值。
- `internal/agent/scanner.go`：执行合并计划、标记发布范围、生成 `module-multi`/单模块/snapshot 批次。
- `internal/agent/scanner_test.go`：覆盖多模块共享依赖只执行一次、选择发布、全量快照和非法选择。
- `internal/cli/arguments.go`：新增动态短参数解析器，只认识注册表模块名、`scan` 和 `-output`。
- `internal/cli/arguments_test.go`：覆盖参数顺序、去重、未知模块、冲突、旧命令和旧参数。
- `internal/cli/run.go`：只保留 `version`、`doctor`、`modules`、`help` 和扫描入口，并调用新运行时接口。
- `internal/cli/run_test.go`：覆盖公共命令、退出码、动态模块发现、模块表和批次发布。
- `internal/cli/output.go`：发布批次并打印扫描摘要。
- `internal/cli/output_test.go`：覆盖整体状态、模块明细、错误缩写和输出绝对路径。
- `internal/cli/scan_options.go`：删除；其旧文件输出模式由 `arguments.go` 的单一 `-output` 根目录语义取代。
- `internal/platform/output_linux.go`：提供 Linux 固定默认输出目录。
- `internal/platform/output_other.go`：保留非 Linux 平台可替换的默认目录实现，不把路径写入模块。
- `internal/platform/output_linux_test.go`、`internal/platform/output_other_test.go`：验证平台默认目录行为。
- `scripts/verify-linux.sh`：按新命令执行真实 root 扫描并从摘要中提取批次路径验证协议。
- `README.md`：只展示新短参数、`scan`、模块表、输出位置和退出码。

### Task 1: 多目标动态模块计划

**Files:**
- Modify: `internal/module/registry.go`
- Modify: `internal/module/registry_test.go`
- Modify: `internal/modules/register_test.go`

**Interfaces:**
- Consumes: `Registry.modules map[string]Module`、`Module.Descriptor()`、`Descriptor.HardDependencies`。
- Produces: `func (registry *Registry) PlanSelected(names []string) ([]Module, error)`、`func (registry *Registry) PlanAll() ([]Module, error)`；模块注册拒绝 `all`、`output`、`help`、`version`、`doctor`、`modules`、`scan`。

- [ ] **Step 1: 写多目标依赖合并的失败测试**

```go
func TestRegistryPlansMultipleTargetsWithSharedDependenciesOnce(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, fakeModule{name: "host"})
	mustRegister(t, registry, fakeModule{name: "network", hard: []string{"host"}})
	mustRegister(t, registry, fakeModule{name: "process", hard: []string{"host"}})
	mustRegister(t, registry, fakeModule{name: "port", hard: []string{"process"}})

	plan, err := registry.PlanSelected([]string{"port", "network", "port"})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(plan); !reflect.DeepEqual(got, []string{"host", "network", "process", "port"}) {
		t.Fatalf("plan = %v", got)
	}
}
```

同时新增表驱动断言：空选择、未知模块、`PlanAll` 空注册表、硬依赖环返回错误；注册 `output/help/version/doctor/modules/scan/all` 任一名称均失败。

- [ ] **Step 2: 运行定向测试并确认 RED**

Run: `go test ./internal/module ./internal/modules -run 'TestRegistryPlansMultiple|TestRegistryRejectsReserved|TestDefaultRegistryPlanAll' -count=1`

Expected: FAIL，提示 `PlanSelected`、`PlanAll` 尚未定义或保留名仍被接受。

- [ ] **Step 3: 用一个内部拓扑函数实现两个公共计划入口**

```go
var reservedModuleNames = map[string]struct{}{
	"all": {}, "output": {}, "help": {}, "version": {},
	"doctor": {}, "modules": {}, "scan": {},
}

func (registry *Registry) PlanSelected(names []string) ([]Module, error) {
	if registry == nil {
		return nil, fmt.Errorf("module registry is nil")
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no modules selected")
	}
	selected := make(map[string]bool)
	for _, name := range names {
		if _, ok := registry.modules[name]; !ok {
			return nil, fmt.Errorf("module %q is not registered", name)
		}
		if err := registry.selectHardDependencies(name, selected); err != nil {
			return nil, err
		}
	}
	return registry.plan(selected)
}

func (registry *Registry) PlanAll() ([]Module, error) {
	if registry == nil {
		return nil, fmt.Errorf("module registry is nil")
	}
	selected := make(map[string]bool, len(registry.modules))
	for name := range registry.modules {
		selected[name] = true
	}
	return registry.plan(selected)
}
```

把当前 `Plan(target string)` 的拓扑排序主体移动为 `plan(selected map[string]bool)`，删除单 target/`all` 分支；`Register` 使用 `reservedModuleNames`。同层仍按名称排序，重复选择通过 map 自动去重。

- [ ] **Step 4: 更新现有计划测试和默认注册表测试**

把单模块测试从 `registry.Plan("port")` 攄为 `registry.PlanSelected([]string{"port"})`，全量测试改为 `registry.PlanAll()`；默认注册表测试断言：

```go
plan, err := registry.PlanAll()
if err != nil {
	t.Fatal(err)
}
if got := moduleNames(plan); !reflect.DeepEqual(got, []string{"host", "network", "process", "connection", "port"}) {
	t.Fatalf("plan = %v", got)
}
```

断言期望顺序必须按真实依赖层和字母序调整，而不是按注册顺序。

- [ ] **Step 5: 运行定向和全量测试并自审**

Run: `gofmt -w internal/module/registry.go internal/module/registry_test.go internal/modules/register_test.go`

Run: `go test ./internal/module ./internal/modules -count=1`

Run: `go test ./... -count=1`

Expected: 全部 PASS；自审确认一个模块只会在 `plan` 结果出现一次，未知依赖和环不会返回部分计划。

- [ ] **Step 6: 提交并推送当前实现分支**

```bash
git add internal/module/registry.go internal/module/registry_test.go internal/modules/register_test.go
git commit -m "refactor: plan dynamic module selections"
git push -u origin codex/nmap-style-cli
```

### Task 2: 多模块扫描编排与发布边界

**Files:**
- Modify: `internal/agent/runtime.go`
- Modify: `internal/agent/scanner.go`
- Modify: `internal/agent/scanner_test.go`

**Interfaces:**
- Consumes: Task 1 的 `Registry.PlanSelected([]string)` 和 `Registry.PlanAll()`。
- Produces: `type ScanSelection struct { All bool; Modules []string }`、`type ScanOutcome struct { Batch model.Batch; RecordCounts map[string]int }`、`Runtime.Scan(context.Context, ScanSelection) (ScanOutcome, error)`。

- [ ] **Step 1: 写共享依赖只执行一次且只发布选择模块的失败测试**

```go
func TestScannerMultipleModulesRunSharedDependencyOnce(t *testing.T) {
	registry := coremodule.NewRegistry()
	calls := map[string]int{}
	host := countingModule("host", nil, calls)
	network := countingModule("network", []string{"host"}, calls)
	process := countingModule("process", []string{"host"}, calls)
	port := countingModule("port", []string{"process"}, calls)
	mustRegisterScannerModule(t, registry, host)
	mustRegisterScannerModule(t, registry, network)
	mustRegisterScannerModule(t, registry, process)
	mustRegisterScannerModule(t, registry, port)

	outcome, err := scannerForRegistry(registry).Scan(context.Background(), ScanSelection{Modules: []string{"network", "port"}})
	if err != nil {
		t.Fatal(err)
	}
	if calls["host"] != 1 || calls["network"] != 1 || calls["process"] != 1 || calls["port"] != 1 {
		t.Fatalf("calls = %v", calls)
	}
	if outcome.Batch.Type != model.BatchTypeModule || outcome.Batch.RequestedModule != "multi" {
		t.Fatalf("batch = %+v", outcome.Batch)
	}
	assertPublishedModules(t, outcome.Batch.Results, []string{"network", "port"})
}
```

再写测试确认 `RecordCounts["host"] == 1`，即依赖记录从可发布 Batch 中清空后，终端仍能展示真实采集条数。

- [ ] **Step 2: 写选择合法性和全量动态快照失败测试**

覆盖：`All:true` 与 `Modules` 同时出现时报错；空选择报错；单模块产生 `RequestedModule:"host"`；全量产生 `BatchTypeSnapshot`、`RequestedModule:"all"` 且全部 `Published:true`；取消 context 在计划前返回 `context.Canceled`。

- [ ] **Step 3: 运行测试确认 RED**

Run: `go test ./internal/agent -run 'TestScannerMultiple|TestScannerSelection|TestScannerAll' -count=1`

Expected: FAIL，旧 `ScanTarget` 无法表达选择集合和 `ScanOutcome`。

- [ ] **Step 4: 替换运行时边界并实现选择归一化**

```go
type ScanSelection struct {
	All     bool
	Modules []string
}

type ScanOutcome struct {
	Batch        model.Batch
	RecordCounts map[string]int
}

type Runtime interface {
	Doctor(context.Context) (model.DoctorReport, error)
	Modules(context.Context) ([]coremodule.Info, error)
	Scan(context.Context, ScanSelection) (ScanOutcome, error)
}
```

`Scanner.Scan` 必须先验证 `All`/`Modules`，复制并去重模块名，选择 `PlanAll` 或 `PlanSelected`。批次元数据规则固定为：全量=`snapshot/all`，一个显式模块=`module/<name>`，两个及以上显式模块=`module/multi`。

- [ ] **Step 5: 实现执行、计数和发布范围**

```go
selected := make(map[string]bool, len(selection.Modules))
for _, name := range selection.Modules {
	selected[name] = true
}
recordCounts := make(map[string]int, len(plan))
for _, item := range plan {
	descriptor := item.Descriptor()
	result := collectPlannedModule(ctx, item, descriptor, executed)
	recordCounts[descriptor.Name] = len(result.Data.Records)
	executed[descriptor.Name] = result

	output := result.Data
	output.Published = selection.All || selected[descriptor.Name]
	if !output.Published {
		output.Records = []model.AssetRecord{}
		output.Relationships = []model.RelationshipRecord{}
	}
	batch.Results = append(batch.Results, output)
}
return ScanOutcome{Batch: batch, RecordCounts: recordCounts}, nil
```

保留现有 panic、timeout、硬依赖阻断、partial 传播和强类型内部依赖复用逻辑；只改变选择入口和统计返回值。

- [ ] **Step 6: 迁移现有 scanner 测试并验证**

把所有 `ScanTarget(ctx, "all")` 改成 `Scan(ctx, ScanSelection{All:true})`，单模块改成 `Scan(ctx, ScanSelection{Modules:[]string{"host"}})`，断言从 `outcome.Batch` 读取。

Run: `gofmt -w internal/agent/runtime.go internal/agent/scanner.go internal/agent/scanner_test.go`

Run: `go test ./internal/agent -count=1`

Run: `go test ./... -count=1`

Expected: 全部 PASS；自审确认依赖记录未发布、依赖状态仍存在于 `Batch.Results`、共享模块未重复收集。

- [ ] **Step 7: 提交并推送**

```bash
git add internal/agent/runtime.go internal/agent/scanner.go internal/agent/scanner_test.go
git commit -m "feat: scan multiple dynamic modules"
git push origin codex/nmap-style-cli
```

### Task 3: 动态短参数解析与公共命令收敛

**Files:**
- Create: `internal/cli/arguments.go`
- Create: `internal/cli/arguments_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Delete: `internal/cli/scan_options.go`

**Interfaces:**
- Consumes: Task 2 的 `Runtime.Scan(context.Context, agent.ScanSelection)`，以及 `Runtime.Modules` 返回的动态模块描述。
- Produces: `func parseScanInvocation(args []string, infos []module.Info) (scanInvocation, error)`；`scanInvocation` 包含 `selection agent.ScanSelection`、排序去重后的 `selected []string` 和 `outputRoot string`。

- [ ] **Step 1: 写动态短参数成功路径的失败测试**

```go
func TestParseScanInvocationUsesRegisteredModuleFlags(t *testing.T) {
	infos := []coremodule.Info{moduleInfo("host"), moduleInfo("network"), moduleInfo("custom")}
	got, err := parseScanInvocation([]string{"-network", "-host", "-network", "-output", "/data/cmdb"}, infos)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.selected, []string{"host", "network"}) {
		t.Fatalf("selected = %v", got.selected)
	}
	if got.outputRoot != "/data/cmdb" || got.selection.All {
		t.Fatalf("invocation = %+v", got)
	}
}

func TestParseScanInvocationDiscoversFutureModule(t *testing.T) {
	got, err := parseScanInvocation([]string{"-service"}, []coremodule.Info{moduleInfo("service")})
	if err != nil || !reflect.DeepEqual(got.selection.Modules, []string{"service"}) {
		t.Fatalf("invocation=%+v err=%v", got, err)
	}
}
```

- [ ] **Step 2: 写参数错误和旧语法失败测试**

表驱动覆盖以下输入均返回参数错误：

```go
[][]string{
	{}, {"-docker"}, {"-output"}, {"-host", "-output", "/a", "-output", "/b"},
	{"scan", "-host"}, {"scan", "host"}, {"scan", "all"}, {"scan", "socket"},
	{"host", "scan"}, {"host", "describe"}, {"host", "status"}, {"host", "schedule"},
	{"all", "scan"}, {"-host", "-o", "x"}, {"-host", "--output", "x"},
	{"-host", "--output-dir", "x"}, {"-host", "extra"},
}
```

未知 `-docker` 的错误文本必须包含所有当前有效动态参数，例如 `-custom, -host, -network`。

- [ ] **Step 3: 运行解析器测试确认 RED**

Run: `go test ./internal/cli -run 'TestParseScanInvocation' -count=1`

Expected: FAIL，`parseScanInvocation` 尚未定义。

- [ ] **Step 4: 实现无模块名 switch 的解析器**

```go
type scanInvocation struct {
	selection  agent.ScanSelection
	selected   []string
	outputRoot string
}

func parseScanInvocation(args []string, infos []coremodule.Info) (scanInvocation, error) {
	known := make(map[string]struct{}, len(infos))
	for _, info := range infos {
		known[info.Descriptor.Name] = struct{}{}
	}
	if len(args) > 0 && args[0] == "scan" {
		return parseFullScan(args[1:], known)
	}
	return parseModuleScan(args, known)
}
```

`parseModuleScan` 逐项识别 `-output <path>` 和 `-<registered-name>`，其他 token 全部报错；模块用 map 去重后排序。`parseFullScan` 只允许零个或一个 `-output <path>`。重复相同输出路径允许，重复不同路径报冲突；空白路径报错。

- [ ] **Step 5: 重写 `Run` 路由并彻底删除旧入口**

`Run` 的路由顺序固定为：

1. 精确的 `version`；
2. 精确的 `doctor`；
3. 读取 `runtime.Modules`；
4. 精确的 `help`/`-h`/`--help`；
5. 精确的 `modules`；
6. `scan` 或首项以 `-` 开头时进入 `parseScanInvocation`；
7. 其他位置参数打印新用法并返回 `2`。

删除 `runLegacyScan`、模块 action switch、`writeValue` 和所有 deprecated 输出。扫描路径调用：

```go
outcome, err := runtime.Scan(ctx, invocation.selection)
if err != nil {
	fmt.Fprintf(stderr, "扫描失败: %v\n", err)
	return 1
}
if err := writeScanResult(stdout, outcome, invocation.selected, invocation.outputRoot, env); err != nil {
	fmt.Fprintf(stderr, "写入扫描结果: %v\n", err)
	return 1
}
return 0
```

- [ ] **Step 6: 把 `modules` 改成稳定表格并更新帮助**

使用 `text/tabwriter` 输出以下列：

```text
MODULE  STATUS  INTERVAL  RESOURCE  TIMEOUT  DEPENDENCIES
```

`StatusOK` 显示为 `supported`，其他状态显示原始英文状态；无硬依赖显示 `-`，多个依赖排序后用逗号连接。帮助必须动态生成 `-<module>` 列表并只展示：单/多模块扫描、`scan`、`-output`、`modules`、`doctor`、`version`、`help`。

- [ ] **Step 7: 改造 fake runtime 和端到端 CLI 测试**

```go
type fakeRuntime struct {
	infos         []coremodule.Info
	outcome       agent.ScanOutcome
	scanErr       error
	selectionSeen *agent.ScanSelection
}

func (runtime fakeRuntime) Scan(_ context.Context, selection agent.ScanSelection) (agent.ScanOutcome, error) {
	if runtime.selectionSeen != nil {
		*runtime.selectionSeen = selection
	}
	return runtime.outcome, runtime.scanErr
}
```

断言 `Run(..., []string{"-custom"}, ...)` 会选择 custom；`Run(..., []string{"-host","-network"}, ...)` 只调用一次 `Scan`；`scan` 设置 `All:true`；全部旧语法退出 `2` 且没有 `deprecated`。

- [ ] **Step 8: 格式化、测试和自审**

Run: `gofmt -w internal/cli/arguments.go internal/cli/arguments_test.go internal/cli/run.go internal/cli/run_test.go`

Run: `go test ./internal/cli -count=1`

Run: `go test ./... -count=1`

Expected: 全部 PASS；`rg -n 'runLegacyScan|ScanTarget|describe|schedule|deprecated|--output-dir|"-o"' internal/cli` 只允许测试中的拒绝样例命中，不允许生产代码命中。

- [ ] **Step 9: 提交并推送**

```bash
git add internal/cli/arguments.go internal/cli/arguments_test.go internal/cli/run.go internal/cli/run_test.go internal/cli/scan_options.go
git commit -m "feat: add nmap style module flags"
git push origin codex/nmap-style-cli
```

### Task 4: 平台默认落盘与终端扫描摘要

**Files:**
- Modify: `internal/cli/output.go`
- Create: `internal/cli/output_test.go`
- Create: `internal/platform/output_linux.go`
- Create: `internal/platform/output_linux_test.go`
- Create: `internal/platform/output_other.go`
- Create: `internal/platform/output_other_test.go`

**Interfaces:**
- Consumes: Task 2 的 `agent.ScanOutcome` 和现有 `report.WriteBatch(root, batch)`。
- Produces: `func platform.DefaultOutputRoot() (string, error)`、`func writeScanSummary(io.Writer, agent.ScanOutcome, []string, string) error`。

- [ ] **Step 1: 写 Linux 固定默认目录的失败测试**

```go
//go:build linux

func TestDefaultOutputRootIsLinuxSystemDirectory(t *testing.T) {
	root, err := DefaultOutputRoot()
	if err != nil {
		t.Fatal(err)
	}
	if root != "/var/lib/asset-agent/output" {
		t.Fatalf("root = %q", root)
	}
}
```

非 Linux 构建测试只断言返回绝对、非卷根路径，使未来平台实现可以独立替换；不得从任何模块描述或模块 switch 推导路径。

- [ ] **Step 2: 实现按 build tag 隔离的平台默认目录**

Linux 文件：

```go
//go:build linux

package platform

func DefaultOutputRoot() (string, error) {
	return "/var/lib/asset-agent/output", nil
}
```

非 Linux 文件延续当前可执行文件同级 `output` 的安全路径解析：调用 `os.Executable`、`filepath.Abs`、`filepath.EvalSymlinks`，再返回 `filepath.Join(filepath.Dir(resolved), "output")`。该实现只属于平台层，未来 Windows/macOS 可分别拆分 build-tag 文件。

- [ ] **Step 3: 写完整、非完整和错误摘要的失败测试**

```go
func TestWriteScanSummaryShowsModulesStatusesCountsAndOutput(t *testing.T) {
	outcome := agent.ScanOutcome{
		Batch: model.Batch{Results: []model.ModuleResult{
			{Module: "host", Status: model.StatusComplete, DurationMS: 8, Errors: []model.ErrorDetail{}},
			{Module: "network", Status: model.StatusPartial, DurationMS: 23, Errors: []model.ErrorDetail{{Code: "collection_error", Message: "route source unavailable"}}},
		}},
		RecordCounts: map[string]int{"host": 1, "network": 6},
	}
	var output bytes.Buffer
	err := writeScanSummary(&output, outcome, []string{"host", "network"}, "/data/cmdb/inbox/module-multi-test")
	if err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"Asset Agent Scan", "Modules: host, network", "Status: partial", "host", "complete", "1", "8ms", "network", "route source unavailable", "Output: /data/cmdb/inbox/module-multi-test"} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q:\n%s", want, text)
		}
	}
}
```

另测错误消息中的换行被压成空格且长度上限为 120 个 rune，避免单个 Provider 错误淹没终端。

- [ ] **Step 4: 实现稳定摘要与严重程度汇总**

严重度固定为：`failed` > `timeout` > `unsupported` > `partial` > `degraded` > `complete/ok`；相同输入必须产生相同整体状态。表格列为 `MODULE STATUS RECORDS DURATION ERROR`，无错误显示 `-`。模块行按 `Batch.Results` 的拓扑执行顺序输出，`Modules:` 行按 CLI 已排序的显式选择输出；全量显示 `Modules: all`。

```go
func writeScanSummary(writer io.Writer, outcome agent.ScanOutcome, selected []string, published string) error {
	status := overallStatus(outcome.Batch.Results)
	modules := strings.Join(selected, ", ")
	if outcome.Batch.Type == model.BatchTypeSnapshot {
		modules = "all"
	}
	if _, err := fmt.Fprintln(writer, "Asset Agent Scan"); err != nil {
		return err
	}
	fmt.Fprintf(writer, "Modules: %s\nStatus: %s\n\n", modules, status)
	return writeModuleSummaryTable(writer, outcome, published)
}
```

- [ ] **Step 5: 让扫描结果始终发布批次后再打印摘要**

`environment` 改为：

```go
type environment struct {
	defaultOutputRoot func() (string, error)
}

func productionEnvironment() environment {
	return environment{defaultOutputRoot: platform.DefaultOutputRoot}
}
```

`writeScanResult` 选择显式 `outputRoot` 或 `env.defaultOutputRoot()`，调用 `report.WriteBatch`，将返回路径 `filepath.Abs` 后传给 `writeScanSummary`。删除 JSON stdout、单 JSON 文件和可执行文件路径默认目录分支。

- [ ] **Step 6: 运行定向、全量和竞态测试**

Run: `gofmt -w internal/cli/output.go internal/cli/output_test.go internal/platform/output_linux.go internal/platform/output_linux_test.go internal/platform/output_other.go internal/platform/output_other_test.go`

Run: `go test ./internal/cli ./internal/platform ./internal/report -count=1`

Run: `go test -race ./internal/module ./internal/agent ./internal/cli ./internal/report -count=1`

Run: `go test ./... -count=1`

Expected: 全部 PASS；现有 report 协议、权限、SHA-256 和 partial 清理测试不变并继续通过。

- [ ] **Step 7: 提交并推送**

```bash
git add internal/cli/output.go internal/cli/output_test.go internal/platform/output_linux.go internal/platform/output_linux_test.go internal/platform/output_other.go internal/platform/output_other_test.go
git commit -m "feat: publish scans with concise summaries"
git push origin codex/nmap-style-cli
```

### Task 5: README 与真实 Linux 验证脚本

**Files:**
- Modify: `README.md`
- Modify: `scripts/verify-linux.sh`

**Interfaces:**
- Consumes: Tasks 3–4 的公共 CLI、摘要中的 `Output: <absolute-path>` 和协议 2.0 批次目录。
- Produces: 可复制执行的中文使用说明，以及覆盖五个模块、组合扫描和全量扫描的 root Linux 验证脚本。

- [ ] **Step 1: 先改验证脚本为新命令并让旧二进制失败**

新增摘要路径提取函数：

```sh
scan_and_get_batch() {
  summary_file=$1
  shift
  "$installed_agent" "$@" -output "$output_root" | tee "$summary_file" >&2
  batch_dir=$(sed -n 's/^Output: //p' "$summary_file" | tail -n 1)
  [ -n "$batch_dir" ] || { printf '%s\n' "摘要缺少 Output: $summary_file" >&2; exit 1; }
  printf '%s\n' "$batch_dir"
}
```

调用固定为：

```sh
host_batch=$(scan_and_get_batch "$work_dir/host.txt" -host)
network_batch=$(scan_and_get_batch "$work_dir/network.txt" -network)
process_batch=$(scan_and_get_batch "$work_dir/process.txt" -process)
port_batch=$(scan_and_get_batch "$work_dir/port.txt" -port)
connection_batch=$(scan_and_get_batch "$work_dir/connection.txt" -connection)
multi_batch=$(scan_and_get_batch "$work_dir/multi.txt" -network -port)
snapshot_batch=$(scan_and_get_batch "$work_dir/snapshot.txt" scan)
```

Run on existing v0.3.1 Linux binary: `sudo ./scripts/verify-linux.sh ./asset-agent-linux-amd64`

Expected: FAIL 在第一个 `-host`，证明脚本确实检验新 CLI。

- [ ] **Step 2: 完成协议和旧命令拒绝断言**

保留目录权限、文件权限、records/bytes/SHA-256、schema、无 `.partial-*` 的全部检查；新增：multi manifest 的 `requested_module == "multi"`，snapshot manifest 的 `requested_module == "all"`。使用以下 helper 确认旧语法退出 `2`：

```sh
expect_usage_error() {
  set +e
  "$installed_agent" "$@" >/dev/null 2>&1
  code=$?
  set -e
  [ "$code" -eq 2 ] || { printf '%s\n' "旧语法未返回 2: $* ($code)" >&2; exit 1; }
}
```

至少验证 `host scan`、`all scan`、`scan host`、`scan socket`、`-host -o x`。

- [ ] **Step 3: 用中文重写 README 当前用法**

README 必须给出以下完整流程且不出现任何旧命令：

```bash
curl -fL -o asset-agent-linux-amd64 \
  https://github.com/Theearthwormsplitsvertically/scan/releases/download/v0.4.0/asset-agent-linux-amd64
chmod +x asset-agent-linux-amd64
sudo install -m 0755 asset-agent-linux-amd64 /usr/local/bin/asset-agent

asset-agent version
sudo asset-agent -host
sudo asset-agent -network -port
sudo asset-agent scan
sudo asset-agent -host -output /data/cmdb
asset-agent modules
```

说明默认目录 `/var/lib/asset-agent/output/inbox`、摘要不是 CMDB 接口、CMDB 读取正式批次内 `manifest.json`/JSONL、扫描器不自动删除未消费批次、退出码 0/1/2、当前模块和周期：host 24h、network 6h、process 12h、port 1h、connection 1h。

- [ ] **Step 4: 静态检查文档与脚本**

Run: `sh -n scripts/verify-linux.sh`

Run: `rg -n 'host scan|all scan|scan host|scan all|scan socket|describe|status|schedule|--output-dir|asset-agent .* -o' README.md scripts/verify-linux.sh`

Expected: 除“旧语法返回 2”的验证参数外无命中；README 无旧语法命中。

Run: `go test ./... -count=1`

Expected: PASS。

- [ ] **Step 5: 提交并推送**

```bash
git add README.md scripts/verify-linux.sh
git commit -m "docs: document nmap style scanner workflow"
git push origin codex/nmap-style-cli
```

### Task 6: 全量审查、合并 main 与 v0.4.0 发布

**Files:**
- Verify: all tracked Go, shell, Markdown files
- Create locally then delete after release: `dist/asset-agent-linux-amd64`、`dist/asset-agent-linux-amd64.sha256`

**Interfaces:**
- Consumes: Tasks 1–5 的完整实现和测试。
- Produces: 已推送的 `main`、Git tag `v0.4.0`、包含 Linux amd64 二进制与 SHA-256 文件的 GitHub Release。

- [ ] **Step 1: 执行规格覆盖和遗留入口审查**

Run: `rg -n 'ScanTarget|runLegacyScan|deprecated:|all scan|scan socket|--output-dir|StandardCommands' cmd internal scripts README.md`

Expected: `cmd`、`internal`、README 生产路径中没有旧 CLI 实现；shell 中只允许旧语法退出码测试出现。若 `StandardCommands` 只作为未展示的模块元数据保留，确认 CLI/help/modules 不读取它，不扩大本次协议改动。

Run: `git diff --check origin/main...HEAD`

Expected: 无输出。

- [ ] **Step 2: 执行完整自动化验证**

Run: `go test ./... -count=1`

Run: `go test -race ./internal/module ./internal/agent ./internal/cli ./internal/report -count=1`

Run: `go vet ./...`

Run: `sh -n scripts/verify-linux.sh`

Expected: 全部退出 `0`。

- [ ] **Step 3: 请求代码审查并修复所有高可信问题**

审查清单逐项核对：动态模块无 switch、共享依赖一次执行、只发布选择记录、manifest 保留依赖状态、摘要计数真实、默认目录正确、旧语法退出 `2`、发布成功即退出 `0`、报告协议测试未变化。每个修复都先增加能复现问题的测试，再修改实现并重跑 Step 2。

如产生修复提交：

```bash
git add cmd internal scripts README.md
git commit -m "fix: address nmap cli review findings"
git push origin codex/nmap-style-cli
```

- [ ] **Step 4: 合并回 main 并推送**

```bash
git switch main
git pull --ff-only origin main
git merge --no-ff codex/nmap-style-cli -m "merge: add nmap style scanner cli"
git push origin main
```

Run: `git status --short --branch`

Expected: `main...origin/main` 无 tracked 修改；三个既有 untracked 用户文件仍在且内容未改变。

- [ ] **Step 5: 从 main 构建可复现 Linux amd64 发行物**

PowerShell：

```powershell
New-Item -ItemType Directory -Force dist | Out-Null
$releaseCommit = (git rev-parse HEAD).Trim()
$releaseTime = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
$env:CGO_ENABLED = '0'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
go build -trimpath -ldflags "-s -w -X github.com/Theearthwormsplitsvertically/scan/internal/buildinfo.Version=v0.4.0 -X github.com/Theearthwormsplitsvertically/scan/internal/buildinfo.Commit=$releaseCommit -X github.com/Theearthwormsplitsvertically/scan/internal/buildinfo.BuildTime=$releaseTime" -o dist/asset-agent-linux-amd64 ./cmd/asset-agent
$hash = (Get-FileHash -Algorithm SHA256 dist/asset-agent-linux-amd64).Hash.ToLowerInvariant()
Set-Content -Encoding ascii -NoNewline dist/asset-agent-linux-amd64.sha256 "$hash  asset-agent-linux-amd64`n"
```

Run: `go version -m dist/asset-agent-linux-amd64`

Expected: `GOOS=linux`、`GOARCH=amd64`、`CGO_ENABLED=0`。

- [ ] **Step 6: 在真实 Linux root 环境验证发行物**

```bash
chmod +x ./asset-agent-linux-amd64
sudo ./scripts/verify-linux.sh ./asset-agent-linux-amd64
```

Expected: 输出“真实 Linux 协议 2.0 验证通过”，五个单模块、一个组合模块和一个 snapshot 均通过 manifest、权限、记录数、字节数和 SHA-256 校验。

- [ ] **Step 7: 创建 tag 和 GitHub Release**

```bash
git tag -a v0.4.0 -m "v0.4.0"
git push origin v0.4.0
gh release create v0.4.0 dist/asset-agent-linux-amd64 dist/asset-agent-linux-amd64.sha256 --title "v0.4.0" --notes "破坏性 CLI 更新：新增动态 -<module> 单模块与组合扫描，保留 scan 全量扫描，统一批次落盘与终端摘要；旧长命令不再兼容。"
gh release view v0.4.0 --json tagName,targetCommitish,url,assets
```

Expected: tag 为 `v0.4.0`，目标提交属于最新 `main`，两个资产名称和 SHA 文件均存在。

- [ ] **Step 8: 删除本地测试/发布产物并做最终核验**

只删除本任务新建的 `dist/asset-agent-linux-amd64` 和 `dist/asset-agent-linux-amd64.sha256`；不得删除根目录现有用户二进制或两个未跟踪中文文档。

Run: `go test ./... -count=1`

Run: `git status --short --branch`

Run: `git log -1 --oneline`

Expected: 测试 PASS；main 与 origin/main 一致；仅原有三个 untracked 用户文件可见；发布 URL 可访问。
