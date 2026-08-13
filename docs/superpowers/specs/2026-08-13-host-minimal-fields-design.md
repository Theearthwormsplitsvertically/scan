# Host 模块最小字段设计

> [!WARNING]
> **本文已被取代。** 正文保留的是包含 `machine_id` 的历史十字段方案，仅供追溯；当前九字段契约与实施要求以[新设计](./2026-08-13-host-machine-id-removal-design.md)和[新计划](../plans/2026-08-13-remove-host-machine-id.md)为准。

## 目标

将 `host` 模块收敛为主机身份、操作系统基线和内存容量采集模块，删除当前业务不需要的硬件描述字段，降低数据契约复杂度。

本次只调整 `host` 模块，不实现 `resource`、`disk`、`filesystem` 或其他规划模块。

## 公共输出字段

`host` 资产仍使用统一 `AssetRecord` 外层结构。`attributes` 只保留以下字段：

| 字段 | 含义 | 数据源 |
|---|---|---|
| `hostname` | 当前主机名，仅用于展示和辅助定位 | `/proc/sys/kernel/hostname` |
| `distribution_name` | 发行版可读名称 | `/etc/os-release` 的 `PRETTY_NAME` |
| `distribution_id` | 发行版机器标识 | `/etc/os-release` 的 `ID` |
| `distribution_version` | 发行版版本 | `/etc/os-release` 的 `VERSION_ID` |
| `kernel_release` | 当前 Linux 内核版本 | `/proc/sys/kernel/osrelease` |
| `architecture` | 当前程序可见的系统架构 | Go Runtime |
| `memory_total_bytes` | 主机内存总容量，单位为字节 | `/proc/meminfo` 的 `MemTotal` |
| `machine_id` | 系统 Machine ID | `/etc/machine-id` |
| `boot_id` | 当前启动实例 ID | `/proc/sys/kernel/random/boot_id` |
| `dmi_uuid` | 物理机或虚拟机 DMI UUID，允许缺失 | `/sys/class/dmi/id/product_uuid` |

统一协议要求的 `record_id`、`record_type`、`host_id`、`scope_id`、`scope_type`、`name`、`platform`、`states`、时间、置信度和证据字段继续保留，不属于可删除的主机扩展字段。

`host` 记录不再在顶层 `version` 或 `vendor` 重复主机属性。

## 删除内容

停止采集和输出：

- `id`：与顶层稳定 `record_id` 重复；
- `vendor`：当前不做硬件厂商和保修管理；
- `model`：当前不做硬件型号管理；
- `cpu_model`：当前漏洞和资产范围不依赖 CPU 型号；
- `cpu_count`：后续由 `resource` 或容量类模块提供。

因此 `host` 模块不再读取 `/proc/cpuinfo`、`/sys/class/dmi/id/sys_vendor` 和 `/sys/class/dmi/id/product_name`，也不再调用 `runtime.NumCPU()`。

## 身份和置信度

稳定 `record_id` 不包含 `boot_id`，避免服务器重启后被识别成新资产。

身份规则为：

1. `machine_id` 和 `dmi_uuid` 都存在：使用两者生成稳定 ID，`confidence = exact`；
2. 只有其中一个存在：使用现有稳定字段生成 ID，`confidence = strong`；
3. 两者都不存在但 `hostname` 存在：使用 hostname 回退，`confidence = inferred`，模块结果必须为非权威；
4. 三者都不存在：模块返回 `failed`，不发布通用的 `unknown-host` 资产记录。

`dmi_uuid` 在云主机、ARM、容器或受限环境中可以正常缺失；只要 `machine_id` 可用，不能仅因 DMI 缺失把模块标记为 `partial`。

## 完整性规则

- `complete` 的最低条件是：`hostname`、发行版名称/ID/版本、`kernel_release`、`architecture`、有效的 `memory_total_bytes`、`boot_id` 均存在，并且 `machine_id` 与 `dmi_uuid` 至少存在一个；
- `machine_id` 和 `dmi_uuid` 是可替代的稳定身份来源，任意一个单独缺失都不构成 `partial`；
- 读取或解析必要字段失败时保留已成功采集的数据，并将模块标记为 `partial`；
- 无稳定身份但存在 hostname 回退时为 `partial`、`authoritative = false`；
- 完全无法建立身份时为 `failed`，不输出 host 资产；
- 只有完整结果可以设置 `authoritative = true`；
- 错误继续写入模块的结构化 `errors`，不能用空字符串掩盖读取失败。

## 兼容性

本次会删除和重命名 `host.attributes` 中的字段。CMDB 尚未设计，没有现有消费者需要兼容；README 和测试以新的最小字段契约为准。

批次外层协议、JSONL 分片方式、manifest 格式和其他模块记录不改变。

## 测试要求

实现采用测试驱动方式，至少覆盖：

1. 正常主机只输出十个批准字段；
2. 已删除字段不会出现在采集模型和 JSON 输出中；
3. Machine ID 与 DMI UUID 同时存在时身份稳定且置信度为 `exact`；
4. 只有一个稳定身份字段时置信度为 `strong`；
5. DMI UUID 缺失但 Machine ID 存在时不会仅因此降级；
6. 仅有 hostname 时结果非权威；
7. 三个身份字段都不存在时不发布 host 记录；
8. 现有依赖模块仍可通过 host 结果取得 `BootID` 和稳定 `host_id`；
9. 全部单元测试、`go vet ./...` 和 Linux/amd64 交叉编译通过。
