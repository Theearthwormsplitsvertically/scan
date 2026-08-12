# System Profile Preflight Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 每次完整或单模块扫描前只读识别 Linux 系统画像，依据画像记录所选采集后端，并让并发默认扫描安全生成不同报告文件。

**Architecture:** capability 层从 `/etc/os-release`、`/proc`、`/sys` 构建 `SystemProfile`；agent 编排层统一执行一次 preflight，并用策略解析器为采集状态标注 backend。report 层增加默认报告的原子独占发布，CLI 不再把“选文件名”和“写文件”拆成有竞争窗口的两个动作。

**Tech Stack:** Go 1.26、Linux procfs/sysfs、标准库文件 API、Go testing。

## Global Constraints

- 不调用 Shell 或外部系统命令。
- 系统选择采用“能力优先、发行版辅助”，不能仅按发行版名称硬编码。
- 每次扫描 preflight 只执行一次。
- 已有 `host/network/process/socket/all` 数据内容保持不变。
- 默认输出目录继续使用可执行文件同级 `output`，目录 `0700`、文件 `0600`。
- 并发默认扫描不得覆盖报告；同秒扫描必须获得不同文件名。
- 中文代码注释。

---

### Task 1: 系统画像检测

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/capability/detect.go`
- Modify: `internal/capability/detect_test.go`

**Interfaces:**
- Produces: `model.SystemProfile`，包含 OS、发行版 ID/版本/名称、内核、架构、init、cgroup、安全模块、容器运行时和可用数据源。
- Produces: `DoctorReport.SystemProfile`。

- [ ] 先写 CentOS 7 和现代 Linux fixture 的失败测试，断言画像字段来自只读事实文件。
- [ ] 运行 `go test ./internal/capability -run SystemProfile -count=1`，确认因画像缺失失败。
- [ ] 实现 `Detect` 填充画像，并保留既有 DoctorReport 字段。
- [ ] 运行 `go test ./internal/capability -count=1`。

### Task 2: 全扫描和模块扫描统一 preflight

**Files:**
- Create: `internal/agent/strategy.go`
- Create: `internal/agent/strategy_test.go`
- Modify: `internal/agent/scan.go`
- Modify: `internal/agent/module.go`
- Modify: `internal/agent/module_test.go`
- Modify: `internal/model/model.go`
- Modify: `internal/model/module_report.go`

**Interfaces:**
- Produces: `Snapshot.SystemProfile`、`ModuleReport.SystemProfile`。
- Produces: `CollectorStatus.Backend`。
- Produces: `collectorBackend(module, profile) string`。

- [ ] 写失败测试，断言每种模块只调用一次 Doctor，报告包含相同画像，并为当前后端标注 `procfs_sysfs`、`standard_library_procfs`、`procfs`。
- [ ] 运行 `go test ./internal/agent -run 'Preflight|Backend' -count=1`，确认失败原因是 preflight 未统一。
- [ ] 实现一次性 preflight 和能力驱动策略解析；当前 socket 实现固定选择真实存在的 `procfs` 后端。
- [ ] 运行 `go test ./internal/agent ./internal/model -count=1`。

### Task 3: 并发安全的默认报告发布

**Files:**
- Modify: `internal/report/path.go`
- Modify: `internal/report/path_test.go`
- Modify: `internal/report/json.go`
- Modify: `internal/report/json_test.go`
- Modify: `internal/cli/output.go`
- Modify: `internal/cli/run_test.go`

**Interfaces:**
- Produces: `WriteDefaultJSONFile(executablePath, module string, now time.Time, value any) (string, error)`。
- Preserves: 显式 `-o` 使用 `WriteJSONFile`；默认输出使用独占发布。

- [ ] 写并发失败测试：两个 goroutine 使用同一模块和时间，必须成功得到两个不同文件，内容都完整。
- [ ] 运行 `go test ./internal/report -run Concurrent -count=1`，确认当前检查后写入存在竞争。
- [ ] 使用同目录临时文件与原子独占链接发布；文件已存在时尝试 `-1`、`-2`，不覆盖。
- [ ] 修改 CLI 默认输出走新接口；`-o -` 和显式路径保持现状。
- [ ] 运行 `go test ./internal/report ./internal/cli -count=1`。

### Task 4: 文档与完整验证

**Files:**
- Modify: `README.md`
- Modify: `docs/代码阅读地图.md`
- Modify: `docs/superpowers/specs/2026-08-12-modular-scan-cli-design.md`

- [ ] 说明 preflight、能力优先策略、backend 字段和 output 存在性检查。
- [ ] 运行 `gofmt`、`git diff --check`、`go vet ./...`、`go test ./... -count=1`。
- [ ] 交叉构建 Linux amd64，并检查工作区状态。
