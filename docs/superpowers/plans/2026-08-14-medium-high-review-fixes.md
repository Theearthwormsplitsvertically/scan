# 中高级审查问题修正实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修正命令行敏感信息泄露、软依赖无法自动执行和 CMDB 跨批次引用契约缺失，并统一中优先级设计文档。

**Architecture:** 保留当前动态注册表、模块发布边界和协议 2.0。注册表把硬依赖与软依赖都纳入执行闭包和拓扑顺序，但只有硬依赖失败才阻断目标模块；软依赖异常只使目标模块降级。模块批次继续只发布用户显式选择的模块，CMDB通过全局稳定 `record_id` 延迟解析跨批次关系。

**Tech Stack:** Go 1.24、Go 标准库、现有模块注册表与批次协议、HTML/Markdown 文档。

## Global Constraints

- 保留 Nmap 风格命令：`-<module>`、组合模块参数与全量 `scan`。
- 不恢复旧长命令、`-o`、`--output` 或 `--output-dir`。
- 不改变“依赖执行但只发布用户选择模块记录”的既有边界。
- 软依赖自动执行、执行顺序早于消费者、失败不阻断消费者。
- 不把通用短参数 `-p` 固定解释为密码。
- 每个阶段独立测试、提交并推送 `main`。
- 不修改或提交用户已有的未跟踪二进制和其它文档。

---

### Task 1: 扩展命令行敏感信息脱敏

**Files:**
- Modify: `internal/security/redact.go`
- Test: `internal/security/redact_test.go`

**Interfaces:**
- Consumes: `RedactArgs(args []string) []string`
- Produces: 支持组合键、Java `-D` 属性、凭据文件键和 URL 查询参数的脱敏行为。

- [ ] **Step 1: 写失败测试**

在 `redact_test.go` 增加表驱动测试，输入至少包含：

```go
[]string{
    "--db-password=db-secret",
    "--client_secret", "client-secret",
    "-Dfoo.password=java-secret",
    "--password-file=/run/secrets/db",
    "DB_ACCESS_TOKEN=env-secret",
    "https://example.test/api?access_token=url-secret&port=443",
    "-p", "5432",
}
```

断言六个敏感值均消失，`-p 5432` 和普通 URL 参数仍保留，参数数量不变。

- [ ] **Step 2: 运行测试确认 RED**

Run: `go test ./internal/security -run 'TestRedactArgs' -count=1`

Expected: FAIL，输出仍含组合键或 URL 中的秘密。

- [ ] **Step 3: 实现最小安全修正**

在 `redact.go` 中：

```go
func isSensitiveKey(key string) bool
func sensitiveKeyTokens(key string) []string
func redactURL(argument string) (string, bool)
```

键名按 `-`、`_`、`.` 分词，识别 `password/passwd/token/secret/authorization/cookie/credential/credentials`，并识别 `api+key`、`private+key` 组合；仅对 `-D` 属性移除 `D` 前缀。URL只处理具有 scheme/host 的地址，并对敏感查询键替换值。

- [ ] **Step 4: 运行 focused 与完整测试**

Run: `go test ./internal/security -count=1`

Expected: PASS。

Run: `go test ./... -count=1`

Expected: PASS。

- [ ] **Step 5: 提交并推送**

```bash
git add internal/security/redact.go internal/security/redact_test.go
git commit -m "fix: redact composite credential arguments"
git push origin main
```

---

### Task 2: 实现软依赖自动规划和非阻断降级

**Files:**
- Modify: `internal/module/registry.go`
- Test: `internal/module/registry_test.go`
- Modify: `internal/agent/scanner.go`
- Test: `internal/agent/scanner_test.go`

**Interfaces:**
- Consumes: `Descriptor.HardDependencies`、`Descriptor.SoftDependencies`
- Produces: `PlanSelected`/`PlanAll` 的完整依赖拓扑，以及软依赖异常后的 `partial` 非权威结果。

- [ ] **Step 1: 写注册表失败测试**

增加测试覆盖：

```go
target := fakeModule{name: "component", hard: []string{"host"}, soft: []string{"package"}}
```

断言选择 `component` 得到 `host, package, component`，`package` 只执行一次；未知软依赖返回明确错误；`alpha` 软依赖 `beta` 且 `beta` 硬依赖 `alpha` 时返回依赖环错误。

- [ ] **Step 2: 运行注册表测试确认 RED**

Run: `go test ./internal/module -run 'Soft|MixedDependencyCycle' -count=1`

Expected: FAIL，当前计划缺少软依赖或未检测混合环。

- [ ] **Step 3: 修改注册表规划**

将硬、软依赖统一用于选择闭包和拓扑排序，并对同名依赖去重。未知硬依赖和未知软依赖分别返回可识别错误；任何硬/软混合环均返回 `module dependency cycle detected`。

- [ ] **Step 4: 运行注册表测试确认 GREEN**

Run: `go test ./internal/module -count=1`

Expected: PASS。

- [ ] **Step 5: 写扫描器失败测试**

建立 `host` 成功、`package` 失败、`component` 软依赖 package 的测试，断言：

```go
targetCalled == true
component.Status == model.StatusPartial
component.Authoritative == false
component.Errors[0].Code == "soft_dependency_unavailable"
package.Published == false
len(package.Records) == 0
```

另增加软依赖 `partial/degraded` 的降级测试和成功软依赖不降级测试。

- [ ] **Step 6: 运行扫描器测试确认 RED**

Run: `go test ./internal/agent -run 'SoftDependency' -count=1`

Expected: FAIL，当前软依赖不会进入计划，也不会使目标结果降级。

- [ ] **Step 7: 实现非阻断降级**

保持 `blockedResult` 只检查硬依赖；扩展 `constrainByDependencies`：软依赖状态为 `failed/timeout/unsupported` 时追加 `soft_dependency_unavailable`，状态为 `partial/degraded` 时追加 `soft_dependency_partial`，目标继续运行并变为 `partial`、`authoritative=false`。

- [ ] **Step 8: 运行 focused 与完整测试**

Run: `go test ./internal/module ./internal/agent -count=1`

Expected: PASS。

Run: `go test ./... -count=1`

Expected: PASS。

- [ ] **Step 9: 提交并推送**

```bash
git add internal/module/registry.go internal/module/registry_test.go internal/agent/scanner.go internal/agent/scanner_test.go
git commit -m "feat: schedule soft module dependencies"
git push origin main
```

---

### Task 3: 固化跨批次协议并统一设计文档

**Files:**
- Modify: `README.md`（若工作区透明过滤导致无效 UTF-8，则使用经 `apply_patch` 生成的精确 index patch）
- Modify: `docs/superpowers/specs/2026-08-13-extensible-asset-scanner-design.md`（同上）
- Modify: `docs/组件与容器采集模块设计.html`

**Interfaces:**
- Consumes: 协议 2.0 的 `batch_type`、稳定 `record_id` 和现有模块发布边界。
- Produces: CMDB 可实现的跨批次引用规范、历史规格状态说明和唯一规范源声明。

- [ ] **Step 1: 更新 CMDB机器接口契约**

在 README 明确：

```text
snapshot 批次自包含；module 批次允许关系端点不在同批次。
CMDB按稳定 record_id 跨批次解析，暂存未解析关系，不能仅因端点暂缺拒绝批次。
新主机或身份变化后必须先执行 asset-agent scan，再启动周期单模块扫描。
```

- [ ] **Step 2: 标记历史规格边界**

在旧可扩展扫描器规格顶部标记：CLI 已由 `2026-08-13-nmap-style-cli-design.md` 和 v0.4.0 替代；ACK、保留清理和常驻调度仍是未实现规划，不得作为当前功能验收依据。

- [ ] **Step 3: 确定组件设计唯一规范源并补 service 字段**

在 HTML 顶部声明当前已跟踪 HTML 为规范源、同名未跟踪 Markdown 不作为实现依据；在 §7.1 attributes 增加：

```text
service_manager、native_service_key、display_name、state
```

保留 Linux Provider 的 `unit_name/unit_state/load_state/fragment_path/enabled_state` 扩展字段。

- [ ] **Step 4: 文档验收**

Run: `git diff --check`

Expected: 无输出，退出 0。

Run: `rg -n '跨批次|暂存未解析关系|首次.*scan|已被.*v0.4.0|规范源|service_manager|native_service_key' README.md docs`

Expected: 每条新契约均可检索；旧规格顶部存在替代说明。

- [ ] **Step 5: 完整测试、提交并推送**

Run: `go test ./... -count=1`

Expected: PASS。

```bash
git add README.md docs/superpowers/specs/2026-08-13-extensible-asset-scanner-design.md docs/组件与容器采集模块设计.html
git commit -m "docs: define module batch reference contract"
git push origin main
```

---

## Self-Review

- 覆盖中高级问题 ①、②、⑥、⑦、⑧、⑨。
- 明确排除低优先级 ③、④、⑤。
- 软依赖规划、排序、失败降级和发布边界均有独立测试。
- 脱敏测试覆盖组合键、Java 属性、URL查询和 `-p` 误报边界。
- 文档任务区分当前实现、已替代 CLI 和未来 ACK/保留规划。
- 所有步骤均给出具体文件、接口、命令和预期结果。
