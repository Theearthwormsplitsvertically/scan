# Modular Scan CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Linux 资产 Agent 改造成按 `host/network/process/socket/all` 独立执行的模块化扫描器，并默认把安全 JSON 报告写入可执行文件同级的 `output` 目录。

**Architecture:** CLI 层只解析模块和输出参数；Agent 层通过模块注册语义编排采集器及内部依赖；Model 层分别定义完整报告和单模块报告；Report 层解析安装目录、生成防覆盖文件名并执行原子写入。`all` 是编排入口，不实现任何专用采集逻辑。

**Tech Stack:** Go 1.26、Linux procfs/sysfs、标准库 `encoding/json`、`os`、`path/filepath`、Go testing。

## Global Constraints

- 产品行为基线为 `v0.2.0`，不保留 v0.1.0 默认向 stdout 写扫描 JSON 的行为。
- `asset-agent scan` 与 `asset-agent scan all` 默认写入 `<可执行文件目录>/output/all-<UTC时间>.json`。
- 只有显式 `-o -` 才向 stdout 写 JSON。
- 默认目录权限 `0700`，报告权限 `0600`，报告通过同目录临时文件原子发布。
- 当前实现模块仅为 `host`、`network`、`process`、`socket`、`all`。
- 每个独立资产功能必须对应独立模块；`all` 只编排模块。
- 不调用 Shell、不读取进程环境变量、不修改服务器状态。
- 代码注释使用中文；Go build tag 保留规定语法。

---

### Task 1: 单模块报告协议与模块编排

**Files:**
- Create: `internal/model/module_report.go`
- Create: `internal/agent/module.go`
- Create: `internal/agent/module_test.go`
- Modify: `internal/model/model.go`
- Modify: `internal/agent/runtime.go`
- Modify: `internal/agent/local_runtime.go`

**Interfaces:**
- Produces: `type Module string` 及 `ModuleAll/Host/Network/Process/Socket`。
- Produces: `Runtime.ScanModule(context.Context, Module) (model.ModuleReport, error)`。
- Produces: `model.ModuleReport`、`model.ModuleData`。
- Preserves: `Runtime.Scan(context.Context) (model.Snapshot, error)` 作为完整扫描编排。

- [ ] **Step 1: 写失败测试，定义模块可见数据和内部依赖**

在 `internal/agent/module_test.go` 使用可注入 `Dependencies`，分别验证：host 只输出 host；network 只输出网络集合；process 自动执行 host 依赖但只输出 processes；socket 自动执行 host、process、socket 并只输出 sockets 与 relationships；未知模块返回错误。

- [ ] **Step 2: 运行测试并确认因 `ScanModule` 和报告类型不存在而失败**

Run: `go test ./internal/agent -run TestScanModule -count=1`

- [ ] **Step 3: 实现最小模块协议和编排**

模块报告接口固定为：

```go
type ModuleReport struct {
    SchemaName      string            `json:"schema_name"`
    SchemaVersion   string            `json:"schema_version"`
    Module          string            `json:"module"`
    Scan            ScanMetadata      `json:"scan"`
    Agent           AgentInfo         `json:"agent"`
    Data            ModuleData        `json:"data"`
    CollectorStatus []CollectorStatus `json:"collector_status"`
    ResourceUsage   ResourceUsage     `json:"resource_usage"`
}
```

`ModuleData` 使用指针和非 nil 切片，仅让被选模块的字段出现。复用现有 `invokeHost/invokeNetwork/invokeProcesses/invokeSockets` 的超时、panic 隔离和状态规范化。

- [ ] **Step 4: 给完整 Snapshot 增加 `schema_name: asset-agent.snapshot`，单模块使用 `asset-agent.module-report`**

更新现有序列化测试，验证两种报告拥有独立协议标识且集合不输出 `null`。

- [ ] **Step 5: 运行 Agent 和 Model 相关测试**

Run: `go test ./internal/agent ./internal/model -count=1`

- [ ] **Step 6: 提交**

```bash
git add internal/model internal/agent
git commit -m "feat: add modular scan orchestration"
```

### Task 2: 默认输出路径与安全报告目录

**Files:**
- Create: `internal/report/path.go`
- Create: `internal/report/path_test.go`
- Modify: `internal/report/json.go`
- Modify: `internal/report/json_test.go`

**Interfaces:**
- Produces: `DefaultOutputPath(executablePath, module string, now time.Time) (string, error)`。
- Preserves: `WriteJSONFile(path string, value any) error`，要求父目录已存在。

- [ ] **Step 1: 写失败测试，定义安装目录和文件名行为**

覆盖：解析绝对可执行文件父目录；创建 `output`；使用 UTC 文件名；相同时间已有文件时追加 `-1`；符号链接解析；空可执行文件路径报错。

- [ ] **Step 2: 运行测试并确认函数缺失导致失败**

Run: `go test ./internal/report -run 'TestDefaultOutputPath|TestWriteJSONFile' -count=1`

- [ ] **Step 3: 实现路径生成**

`DefaultOutputPath` 必须：绝对化路径、解析符号链接、创建并收紧 `output` 为 `0700`、生成 `<module>-YYYYMMDDTHHMMSSZ.json`、发现冲突时依次尝试 `-1`、`-2`。

- [ ] **Step 4: 验证 JSON 文件权限和原子写入**

保留同目录临时文件、`Sync`、`Close`、`Rename` 流程；测试报告权限 `0600`，写入错误不留下最终半文件。

- [ ] **Step 5: 运行 Report 测试并提交**

Run: `go test ./internal/report -count=1`

```bash
git add internal/report
git commit -m "feat: add secure default report paths"
```

### Task 3: 模块化 CLI 和输出规则

**Files:**
- Create: `internal/cli/scan_options.go`
- Create: `internal/cli/output.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Modify: `cmd/asset-agent/main.go`
- Modify: `cmd/asset-agent/runtime_linux.go`
- Modify: `cmd/asset-agent/runtime_unsupported.go`

**Interfaces:**
- Produces: 包内 `scanOptions{module agent.Module, output string, explicitOutput bool}`。
- Produces: 包内 `environment{executablePath func() (string, error), now func() time.Time}` 以便无全局状态测试默认路径。
- Preserves: `Run(...) int` 作为生产入口。

- [ ] **Step 1: 写失败的表驱动参数测试**

覆盖：`scan`、`scan all`、四个模块、`-o`、`--output`、`-o -`、未知模块、规划模块、未知选项、缺少值、重复输出参数、多余参数、`scan --help`。

- [ ] **Step 2: 运行 CLI 测试并确认新命令行为失败**

Run: `go test ./internal/cli -count=1`

- [ ] **Step 3: 实现参数解析和帮助输出**

`scan` 无模块时选择 `all`；`service/package/container/application/file/security` 返回明确的未实现提示和退出码 `2`；其他未知值返回未知模块错误。

- [ ] **Step 4: 写失败测试，定义默认文件输出和成功路径提示**

测试使用临时“可执行文件”路径和固定 UTC 时间，断言默认 JSON 位于同级 `output`，stdout 只包含最终绝对路径；`-o -` 的 stdout 只包含 JSON。

- [ ] **Step 5: 实现输出决策**

未显式 `-o` 时调用 `report.DefaultOutputPath`；显式路径直接写入；显式 `-o -` 调用 `report.WriteJSON`。文件写入成功后打印绝对路径。

- [ ] **Step 6: 更新非 Linux Runtime**

`UnavailableRuntime.ScanModule` 返回 `asset collection requires Linux`，确保 Windows 开发构建满足接口。

- [ ] **Step 7: 运行 CLI 测试和全部 Go 测试并提交**

Run: `go test ./internal/cli ./... -count=1`

```bash
git add internal/cli cmd/asset-agent internal/agent
git commit -m "feat: add modular scan CLI"
```

### Task 4: 使用文档和 Linux 验证脚本

**Files:**
- Modify: `README.md`
- Modify: `scripts/verify-linux.sh`
- Modify: `docs/代码阅读地图.md`

**Interfaces:**
- Documents: v0.2.0 命令、默认输出位置、`-o`、`-o -`、模块职责。
- Verifies: 脚本在受控临时安装目录复制 Agent，执行每个已实现模块并检查 JSON。

- [ ] **Step 1: 更新 README 命令示例和输出位置说明**

- [ ] **Step 2: 修改验证脚本，实际执行五种模块命令**

脚本必须打印保留报告的位置；不再在退出时立即删除用户需要检查的测试结果。临时目录由脚本创建并设置 `0700`。

- [ ] **Step 3: 更新代码阅读地图的 CLI、模型和调用关系**

- [ ] **Step 4: 运行 shell 语法检查（可用时）和文档命令人工核对**

Run: `sh -n scripts/verify-linux.sh`

- [ ] **Step 5: 提交**

```bash
git add README.md scripts/verify-linux.sh docs/代码阅读地图.md
git commit -m "docs: document modular scanner usage"
```

### Task 5: 完整验证

**Files:**
- Verify only: `cmd/asset-agent/*.go`、`internal/**/*.go`、`README.md`、`scripts/verify-linux.sh`、`docs/代码阅读地图.md`

- [ ] **Step 1: 格式化和检查差异**

Run: `gofmt -w cmd/asset-agent/*.go internal/agent/*.go internal/cli/*.go internal/model/*.go internal/report/*.go`
Run: `git diff --check`

- [ ] **Step 2: 运行静态检查和全量测试**

Run: `go vet ./...`
Run: `go test ./... -count=1`

- [ ] **Step 3: 交叉构建 Linux amd64**

Run: `$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'; go build -trimpath -o dist/asset-agent-linux-amd64 ./cmd/asset-agent`

- [ ] **Step 4: 检查分支状态和提交记录**

Run: `git status --short --branch`
Run: `git log --oneline -8`
