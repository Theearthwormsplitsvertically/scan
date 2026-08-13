# Nmap 式扫描器命令设计

日期：2026-08-13

状态：已确认，作为下一版 CLI 的有效规格。

## 目标

扫描器的日常使用方式应接近 nmap：每个扫描模块对应一个直接参数，用户不需要理解模块动作、依赖编排或批次目录协议。一次命令完成模块选择、依赖计算、扫描、简洁终端展示和完整 CMDB 批次发布。

本次只重构 CLI 入口和终端展示，不改变现有 Provider、模块采集内容、JSON 字段、批次协议或原子发布安全边界。

## 公共命令

单模块扫描：

```bash
sudo asset-agent -host
sudo asset-agent -network
sudo asset-agent -process
sudo asset-agent -port
sudo asset-agent -connection
```

组合扫描：

```bash
sudo asset-agent -host -network
sudo asset-agent -process -port
sudo asset-agent -host -network -process -port -connection
```

全量扫描：

```bash
sudo asset-agent scan
```

基础信息命令：

```bash
asset-agent modules
asset-agent doctor
asset-agent version
asset-agent help
```

可选输出目录：

```bash
sudo asset-agent -host -output /data/cmdb
sudo asset-agent -host -network -output /data/cmdb
sudo asset-agent scan -output /data/cmdb
```

`-output` 是保留参数，不能注册为模块名。参数顺序不影响模块选择；重复模块参数自动去重。

## 删除的旧命令

下列语法全部删除，不再兼容，也不再输出迁移期 `deprecated` 提示：

```text
asset-agent host scan
asset-agent host describe
asset-agent host status
asset-agent host schedule
asset-agent all scan
asset-agent scan host
asset-agent scan all
asset-agent scan socket
```

这些输入统一作为参数错误处理，返回退出码 `2`，并显示当前有效用法。

## 动态模块参数

模块短参数不能由 CLI 使用固定模块名 `switch` 实现。CLI 从模块注册表读取名称，并按 `-<module>` 规则动态识别：

```text
注册 host       -> -host
注册 network    -> -network
未来注册 service -> -service
```

新模块注册完成后自动获得直接扫描参数，并自动进入 `scan` 全量扫描，不需要修改 CLI 的模块名列表。

模块名必须继续遵守现有安全命名约束，并且不得使用 `output`、`help`、`version`、`doctor`、`modules` 或 `scan` 等保留名称。

## 组合扫描与依赖

CLI 把用户选择的模块集合传给编排层。编排层合并各模块依赖图，进行拓扑排序，并保证同一个模块和底层共享数据源在一次命令中最多执行一次。

例如：

```bash
asset-agent -network -port
```

内部可形成：

```text
host -> network
host -> process -> port
```

`host` 只执行一次。正式批次发布用户明确选择的 `network` 和 `port` 资产记录；依赖模块的状态、覆盖范围和错误仍写入 `manifest.json`，确保结果可解释。

`asset-agent scan` 动态选择全部已注册模块并发布 `snapshot` 批次。`scan` 不能与任何 `-<module>` 参数组合；例如 `asset-agent scan -host` 返回退出码 `2`。

## 终端摘要

扫描命令默认不向终端打印完整 JSON。扫描完成后打印稳定、简洁、适合人工查看的文本摘要，同时把完整机器数据写入批次目录。

示例：

```text
Asset Agent Scan
Modules: host, network
Status: complete

MODULE       STATUS       RECORDS    DURATION
host         complete     1          8ms
network      complete     6          23ms

Output: /var/lib/asset-agent/output/inbox/module-multi-...
```

摘要必须包含：

- 用户选择的模块；
- 整体状态；
- 每个执行模块的状态、记录数和耗时；
- `partial`、`degraded`、`failed`、`timeout`、`unsupported` 模块的简短错误信息；
- 正式批次目录的绝对路径。

整体状态按最严重模块状态汇总。终端摘要只用于人工操作；CMDB 仍以 `manifest.json` 和 JSONL 为唯一机器接口。

## `modules` 输出

原来的模块级 `describe`、`status`、`schedule` 命令删除。`asset-agent modules` 一次展示所有已注册模块的名称、当前平台支持状态、默认周期、资源等级、超时和依赖。

默认使用简洁表格，例如：

```text
MODULE       STATUS       INTERVAL    RESOURCE
host         supported    24h         light
network      supported    6h          light
process      supported    12h         medium
port         supported    1h          medium
connection   supported    1h          medium
```

## 输出目录与批次协议

Linux 默认输出根目录固定为：

```text
/var/lib/asset-agent/output
```

默认目录由平台层提供，不写入任何具体扫描模块。未来 Windows 和 macOS Provider 可以提供各自平台默认目录。

`-output <path>` 覆盖本次命令的默认目录。输出层继续使用现有批次协议：

- 输出到 `<root>/inbox/<formal-batch>/`；
- 先写 `.partial-<scan-id>`，同步完成后原子改名；
- `manifest.json` 最后写入；
- 目录权限 `0700`，文件权限 `0600`；
- JSON 字段保持英文；
- 不改变 schema version `2.0`、JSONL 分片限制或 SHA-256 清单语义；
- 扫描器仍不自动删除未消费批次。

组合扫描使用能够明确表示多模块选择的 module 批次目录名；全量扫描继续使用 `snapshot-...`。目录名由报告层根据批次类型生成，CLI 不拼接路径。

## 错误与退出码

```text
0  扫描已执行且正式批次成功发布
1  扫描无法执行或正式批次发布失败
2  命令或参数错误
```

只要正式批次成功发布，即使某些模块为 `partial` 或其他非完整状态，进程仍返回 `0`；终端摘要必须清晰标出警告，完整可信度由 manifest 表达。

以下情况返回 `2`：

- 未知模块参数，例如 `-docker`；
- `scan` 与模块参数混用；
- `-output` 缺少路径或重复给出冲突路径；
- 使用已删除的旧命令；
- 传入额外位置参数。

未知模块错误必须列出当前有效模块参数，方便用户立即修正。

## 测试要求

自动化测试必须覆盖：

1. 每个已注册模块的直接参数；
2. 多模块组合、顺序无关和重复参数去重；
3. 依赖图合并以及模块/共享采集不重复执行；
4. `scan` 动态执行全部注册模块；
5. 新注册模块自动获得 `-<module>` 参数；
6. 未知模块、保留名称、缺失输出路径、冲突参数和全部旧命令；
7. 简洁扫描摘要和非完整模块的错误摘要；
8. `modules` 表格包含状态、周期、资源等级、超时和依赖；
9. Linux 默认输出目录和 `-output` 覆盖；
10. 现有批次 schema、JSONL、manifest、权限、SHA-256 和原子发布测试继续通过；
11. 全量测试、`go vet` 和 Linux amd64、CGO-disabled 发布构建。

## 发布

删除旧 CLI 是有意的破坏性变更。实现、审查和真实 Linux 验证完成后发布 `v0.4.0`，README 只把新短参数和 `scan` 作为当前用法，不再展示旧命令。
