# Host 模块移除 Machine ID 设计

日期：2026-08-13

状态：已批准，取代 `2026-08-13-host-minimal-fields-design.md` 中所有 Machine ID 相关约定。

## 目标

Host 模块只采集服务器身份、操作系统基线、内存总容量和当前启动实例所需的最小事实。Machine ID 不再读取、不进入内部模型、不输出到 JSON，也不参与稳定主机标识计算。

## 已选方案

采用“完全删除 Machine ID”。曾考虑但不采用的方案包括：仅从 JSON 隐藏但继续内部使用，以及只保留 hostname 身份。前者仍保留无必要的数据源和隐式依赖；后者在重命名或名称复用时稳定性不足。

## 九字段契约

`host.attributes` 只允许以下英文键：

1. `hostname`
2. `distribution_name`
3. `distribution_id`
4. `distribution_version`
5. `kernel_release`
6. `architecture`
7. `memory_total_bytes`
8. `boot_id`
9. `dmi_uuid`

Linux Provider 不再访问 `/etc/machine-id`。`boot_id` 只表示当前启动实例，不能参与稳定 Host `record_id`。

## 身份与状态规则

- `dmi_uuid` 存在：使用规范化后的 DMI UUID 生成稳定 `record_id`，`confidence = strong`。
- `dmi_uuid` 缺失但 `hostname` 存在：使用 hostname 生成回退 `record_id`，结果必须为 `partial`、`confidence = inferred`、`authoritative = false`，并记录稳定身份不可用。
- `dmi_uuid` 和 `hostname` 均缺失：模块返回 `failed`，`objects = 0`，不发布 Host 记录。
- DMI UUID 读取失败只在无法取得 DMI UUID 时作为结构化采集错误保留；有 hostname 时仍可发布低置信度记录。
- 发行版名称/ID/版本、内核版本、架构、内存总容量或 Boot ID 缺失时，现有记录可发布但状态为 `partial`。

## 代码边界

- `internal/model/model.go`：`model.Host` 删除 `MachineID`。
- `internal/collect/host/collector.go`：删除 `/etc/machine-id` 数据源及相关错误处理。
- `internal/modules/host/module.go`：删除 `machine_id` 属性，DMI UUID 成为唯一强身份，hostname 仅为回退。
- Network、Process 等下游模块继续通过 `model.Host` 使用所需事实，测试夹具不得再构造 Machine ID。
- README 的 Host 字段说明改为九字段语义。

## 测试与清理

测试必须覆盖九字段输出、DMI 强身份、hostname 回退、无身份失败、Boot ID 不改变稳定 `record_id`、必需事实缺失及读取错误。按 TDD 先验证新断言在旧实现上失败，再实现通过。

版本控制中的 `*_test.go` 是长期回归保护，随实现保留。测试生成的临时目录由 Go 测试框架清理；额外生成的测试报告、临时 JSON、覆盖率文件和编译产物在验证完成后删除。用户原有未跟踪文件不属于清理范围。

## 兼容性

删除 Machine ID 会改变尚未发布的新 Host 身份算法。该变更在合入 `main` 和创建下一版 Release 前完成，因此不提供旧十字段格式的兼容输出；文档与 Release 只描述九字段契约。
