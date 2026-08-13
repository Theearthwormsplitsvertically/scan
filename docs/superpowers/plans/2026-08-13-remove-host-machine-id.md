# Host Machine ID Removal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Host 模块从十字段收敛为九字段，彻底删除 Machine ID 的采集、模型、输出和身份计算。

**Architecture:** Linux Host Provider 只采集 DMI UUID 作为强主机身份，Host 模块在 DMI 缺失时使用 hostname 生成低置信度回退记录。Boot ID 仅供启动实例和 Process 关联使用，不参与稳定 Host ID。

**Tech Stack:** Go、标准库、现有 Provider/Module 抽象、Go testing、GitHub Release。

## Global Constraints

- `host.attributes` 只允许 `hostname`、`distribution_name`、`distribution_id`、`distribution_version`、`kernel_release`、`architecture`、`memory_total_bytes`、`boot_id`、`dmi_uuid`。
- 不读取 `/etc/machine-id`，`model.Host` 不含 `MachineID`，任何运行时 JSON 不含 `machine_id`。
- DMI UUID 身份为 `strong`；仅 hostname 时为 `partial`、`inferred`、非权威；DMI UUID 与 hostname 都没有时为 `failed` 且不发布记录。
- Boot ID 改变不得改变 Host `record_id`。
- 各模块边界和命令保持不变，不增加依赖或平台专用命令。
- JSON 字段使用英文，README 和说明使用中文。
- 保留仓库中的 `*_test.go`；仅删除本次测试生成的临时输出、报告、覆盖率文件和编译产物，不删除用户原有未跟踪文件。
- 每个完成任务必须提交并推送到 `origin/codex/host-minimal-fields`。

---

### Task 1: 实现 Host 九字段契约

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/collect/host/collector.go`
- Modify: `internal/collect/host/collector_test.go`
- Modify: `internal/collect/host/parse.go`
- Modify: `internal/modules/host/module.go`
- Modify: `internal/modules/host/module_test.go`
- Modify: `internal/modules/network/module_test.go`
- Modify: `internal/modules/process/module_test.go`

**Interfaces:**
- Produces: `model.Host` 九字段结构；`host.RecordID(model.Host) string` 只使用 DMI UUID 或 hostname。
- Preserves: `provider.HostProvider`、`host.Collect(context.Context, platform.Root)` 和模块命令接口。

- [ ] **Step 1: 把测试改为九字段期望**

调整真实 Collector/Module 测试，使它们明确验证：属性键恰好为九个；反射得到的 `model.Host` 字段不含 `MachineID`；DMI UUID 生成 `strong` 身份；hostname 回退降级；无身份失败；Boot ID 改变不影响 `record_id`；读取错误转换为状态错误。Network 和 Process 夹具只构造实际消费的 Host 字段。

- [ ] **Step 2: 运行 focused tests 验证 RED**

Run:

```powershell
go test ./internal/collect/host ./internal/modules/host ./internal/modules/network ./internal/modules/process -count=1
```

Expected: FAIL，原因是生产模型和模块仍含 `MachineID`/`machine_id` 或身份置信度仍按旧规则计算。

- [ ] **Step 3: 实现最小生产变更**

删除 `model.Host.MachineID`；Collector 删除 `/etc/machine-id` 读取，只以 DMI UUID 判断强身份；Module 属性表删除 `machine_id`，`RecordID` 仅使用规范化后的 DMI UUID，否则回退 hostname；DMI 的置信度固定为 `strong`。保留上次审查已经补充的命名返回值计时修复、必需事实测试和 `StatusComplete` 回退降级修复。

- [ ] **Step 4: 格式化并验证 GREEN**

Run:

```powershell
gofmt -w internal/model/model.go internal/collect/host/collector.go internal/collect/host/collector_test.go internal/collect/host/parse.go internal/modules/host/module.go internal/modules/host/module_test.go internal/modules/network/module_test.go internal/modules/process/module_test.go
go test ./internal/collect/host ./internal/modules/host ./internal/modules/network ./internal/modules/process -count=1
go test ./... -count=1
go vet ./...
```

Expected: 全部 exit 0。

- [ ] **Step 5: 检查 Machine ID 已从运行时代码移除**

Run:

```powershell
rg -n 'MachineID|machine_id|/etc/machine-id' internal cmd
```

Expected: 无匹配，`rg` 因无匹配返回 1。

- [ ] **Step 6: 提交并推送**

```powershell
git add internal/model/model.go internal/collect/host internal/modules/host internal/modules/network/module_test.go internal/modules/process/module_test.go
git diff --cached --check
git commit -m "refactor: remove host machine id"
git push origin codex/host-minimal-fields
```

### Task 2: 更新文档、验证发布构建并清理临时文件

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-08-13-host-minimal-fields-design.md`
- Modify: `docs/superpowers/plans/2026-08-13-host-minimal-fields.md`

**Interfaces:**
- Consumes: Task 1 九字段契约。
- Produces: 与代码一致的中文文档和可发布 Linux amd64 二进制验证结果。

- [ ] **Step 1: 更新当前文档**

README 将 Host 表述改为九字段，说明 DMI UUID 是强身份、hostname 是低置信度回退、Boot ID 不参与稳定身份。旧的十字段规格和计划增加醒目的“已被 Machine ID 移除规格取代”说明，并移除当前有效契约中的 Machine ID 要求，避免两份当前文档互相矛盾。

- [ ] **Step 2: 验证源码、文档和 Linux 构建**

```powershell
git diff --check
go test ./... -count=1
go vet ./...
$env:GOOS='linux'
$env:GOARCH='amd64'
$env:CGO_ENABLED='0'
go build -buildvcs=false -trimpath -o dist/asset-agent-linux-amd64 ./cmd/asset-agent
go version -m dist/asset-agent-linux-amd64
```

Expected: tests、vet、build 全部 exit 0，构建元数据为 Linux/amd64/CGO disabled。

- [ ] **Step 3: 删除本次测试产物并确认工作树范围**

只删除 `dist/asset-agent-linux-amd64`、本计划 SDD 工作区内报告以及本次新产生的覆盖率/临时 JSON（如存在）。不得删除根目录既有 `asset-agent-linux-amd64` 和两份未跟踪中文文档。确认 `git status --short` 仅含本任务预期文档修改。

- [ ] **Step 4: 提交并推送**

```powershell
git add README.md docs/superpowers/specs/2026-08-13-host-minimal-fields-design.md docs/superpowers/plans/2026-08-13-host-minimal-fields.md
git diff --cached --check
git commit -m "docs: document host identity fields"
git push origin codex/host-minimal-fields
```

### Task 3: 审查、合并 main 并发布新版

**Files:**
- No source changes expected.

**Interfaces:**
- Consumes: 通过完整测试和审查的功能分支。
- Produces: 与 `origin/main` 同步的 main 和包含九字段 Host 模块的新 GitHub Release。

- [ ] **Step 1: 完成任务级和全分支代码审查**

所有 Critical/Important 问题必须修复并重新验证；Minor 问题记录裁决。最终审查覆盖从 `main` 合并基点到功能分支 HEAD 的完整差异。

- [ ] **Step 2: 在功能分支执行最终质量门**

```powershell
go test ./... -count=1
go vet ./...
git diff --check main...HEAD
```

- [ ] **Step 3: 合并并验证 main**

在主工作树先 `git pull --ff-only origin main`，再使用 `--no-ff` 合并功能分支。重新运行完整 tests、vet 和 Linux amd64 build，随后推送 `main`。

- [ ] **Step 4: 发布下一补丁版本**

根据当前最新标签递增补丁版本，生成 Linux amd64 二进制和 SHA-256 校验文件，创建 GitHub Release；发布说明用中文列出 Host 九字段契约和 Machine ID 已删除。验证 Release 标签、附件和 `main` 提交一致。
