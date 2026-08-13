# 可扩展资产扫描器与 CMDB 证据协议设计

日期：2026-08-13  
状态：已完成交互设计，等待书面复核

## 1. 背景

本项目的核心不是简单输出主机信息，而是在每台服务器上运行一个低影响、只读、可追溯的资产扫描器。扫描器需要持续回答：

- 当前服务器及文件系统、磁盘、网络的资源使用情况；
- 服务器和容器中安装、运行、加载及暴露了哪些系统、服务、组件、框架和插件；
- 每个监听端口属于哪个进程、容器、服务和应用；
- 应用、服务、组件、容器、进程、端口和主机之间有什么直接证据关系；
- 某个节点失联、链路节点故障，或某个服务、组件、框架、插件出现漏洞时，哪些服务器和业务系统可能受影响；
- 每项结论来自什么数据源，扫描是否完整，哪些范围因权限、超时或数据源缺失而没有覆盖。

扫描器只负责资产发现、资源观测、关系构建、变化感知、证据记录和本地文件交付。CMDB 负责消费文件、保存历史、资产治理、漏洞匹配和影响范围计算。扫描器不内置 CMDB 业务规则，也不上传数据。

## 2. 设计目标

### 2.1 核心目标

1. 每个扫描模块彼此独立，拥有自己的命令、协议、依赖、调度周期、资源预算和测试。
2. CLI 从模块注册表动态生成命令，不维护固定模块名单。
3. 业务模块不直接依赖 Linux `/proc`、`/sys` 等实现细节，而通过平台 Provider 获取能力。
4. 首版实现 Linux Provider，同时为 Windows、macOS 和其他系统保留清晰的实现边界。
5. 通过事件触发定向更新和低频完整校准兼顾实时性与服务器资源安全。
6. 输出协议可处理未知规模的数据，支持分片、校验、原子发布、消费确认和故障恢复。
7. 每个资产、状态、版本和关系均可追溯到证据。
8. 数据完整性必须显式表达，失败和未覆盖范围不能伪装成空结果或资产删除。

### 2.2 非目标

- 不在扫描器中实现漏洞库、CVE 匹配或影响分析界面；
- 不实现 CMDB、中心接收端或网络上传；
- 不执行主动端口探测、攻击验证或远程命令；
- 不递归遍历整个磁盘；
- 不读取进程环境变量、私钥、Token、密码或配置原文；
- 首版不加载第三方代码到 root 主进程；
- 首版不实现 Windows 和 macOS 的完整采集，只提供接口、平台探测和明确的 `unsupported` 状态。

## 3. 总体架构

```text
CLI / systemd service
        ↓
动态模块注册表与命令注册表
        ↓
依赖解析、事件合并、调度与资源保护
        ↓
独立业务模块
        ↓
通用 Provider 能力接口
        ↓
Linux / Windows / macOS / 外部插件 Provider
        ↓
统一资产、指标、关系和证据模型
        ↓
分片批次、manifest、原子发布和消费确认
        ↓
指定输出目录，等待 CMDB 消费
```

调用方向必须保持单向：CLI 和调度器依赖模块注册表，模块依赖 Provider 接口和模型，平台实现只实现 Provider，输出层只处理统一模型。业务模块不得直接导入具体平台包。

## 4. 模块注册表

### 4.1 模块接口

每个内置模块实现统一边界：

```go
type Module interface {
	Descriptor() ModuleDescriptor
	Probe(context.Context, Runtime) SupportResult
	Collect(context.Context, Runtime, Request) ModuleResult
}
```

`ModuleDescriptor` 至少声明：

```text
name
schema_version
record_types
commands
command_options
required_capabilities
optional_capabilities
hard_dependencies
soft_dependencies
default_schedule
resource_class
timeout
supports_delta
```

- 硬依赖失败时，模块不能声明完整成功；
- 软依赖失败时，模块保留可验证事实并标记降级；
- `Probe` 根据当前 Runtime 中的 Provider 能力返回 `supported`、`degraded` 或 `unsupported`；
- 新增模块只需实现接口并注册，CLI、帮助、`all`、调度和输出编排不增加模块名称分支。

### 4.2 外部插件扩展

首版只运行编译进单二进制的模块，但预留外部插件协议。未来插件采用受限独立进程和版本化 IPC，不采用 Go 原生动态插件。插件描述文件必须声明：

- 插件和 API 版本；
- 提供的模块和 Provider 能力；
- 支持的平台；
- 所需权限和允许读取的证据类型；
- 输出记录类型及 Schema 版本；
- CPU、内存、IO、超时和最大输出预算；
- 签名、所有权和允许列表信息。

root 主进程不把第三方插件加载到自身地址空间。插件不可获得未声明的数据源。

## 5. 独立扫描模块

| 模块 | 正式扫描命令 | 主要职责 | 默认周期 |
|---|---|---|---:|
| `host` | `asset-agent host scan` | 主机身份、OS、内核、架构和启动信息 | 24 小时 |
| `resource` | `asset-agent resource scan` | CPU、内存、Load、Swap及扫描器自身指标 | 1 分钟 |
| `filesystem` | `asset-agent filesystem scan` | 挂载点、容量、inode和使用率 | 30 分钟 |
| `disk` | `asset-agent disk scan` | 块设备、吞吐、IOPS和繁忙时间 | 30 分钟 |
| `network` | `asset-agent network scan` | 网卡、IP、路由、DNS和网络作用域 | 30 分钟 |
| `process` | `asset-agent process scan` | 宿主机及容器进程、身份和运行上下文 | 12 小时 |
| `port` | `asset-agent port scan` | 监听端口、协议、地址及归属 | 5 分钟 |
| `connection` | `asset-agent connection scan` | 聚合后的服务调用和外部依赖 | 5 分钟 |
| `package` | `asset-agent package scan` | 宿主机及容器内已安装软件包 | 24 小时 |
| `container` | `asset-agent container scan` | 运行时、镜像、容器、挂载、网络和状态 | 12 小时 |
| `service` | `asset-agent service scan` | 操作系统服务和通用运行服务实例 | 12 小时 |
| `file` | `asset-agent file scan` | 运行相关文件、清单、权限和按需摘要 | 24 小时 |
| `component` | `asset-agent component scan` | 软件、运行时、框架、库和插件 | 24 小时 |
| `application` | `asset-agent application scan` | 可部署应用和业务系统实例聚合 | 24 小时 |
| `all` | `asset-agent all scan` | 动态执行注册表中全部已启用模块 | 按配置 |

`all` 是由注册表提供的虚拟编排模块，不实现任何专用采集逻辑。它在运行时解析全部已启用模块的依赖图，因此新增模块后不需要修改 `all`。

容器内进程仍由 `process` 统一采集，以 `runs_in` 关系关联容器，不复制第二套进程模块。

`application` 不针对某个具体产品。它通过 systemd Unit、容器标签、Compose/Kubernetes 元数据、启动入口、部署目录、制品名称、组件清单和用户资产标签，把进程、服务、容器、组件与端口聚合为可部署应用。证据不足时输出 `unknown_application`，不得静默遗漏或仅凭进程名强行归类。

## 6. 模块优先命令设计

### 6.1 全局命令

```bash
asset-agent run
asset-agent status
asset-agent doctor
asset-agent modules
```

- `run` 启动常驻调度、事件监听和文件交付；
- `status` 展示队列、模块延迟、最近成功扫描、输出积压、磁盘水位及扫描器资源使用；
- `doctor` 检查 root 权限、平台 Provider、模块能力、输出目录、磁盘水位、事件源和资源限制；
- `modules` 从注册表列出模块、支持状态、Schema、计划和资源等级。

### 6.2 模块命令空间

每个模块自动获得以下公共动作：

```bash
asset-agent <module> scan
asset-agent <module> describe
asset-agent <module> status
asset-agent <module> schedule
```

`schedule` 只展示模块当前生效的周期、事件触发器和下一次计划，不直接修改配置。

模块可以通过描述符增加定向扫描参数，例如：

```bash
asset-agent process scan --pid 1234
asset-agent container scan --container-id abc123
asset-agent component scan --scope container:abc123
asset-agent application scan --changed-since 2026-08-13T10:00:00Z
asset-agent port scan --listen-only
```

这些参数由模块注册，不进入 CLI 的固定 `switch`。未知模块从注册表统一返回错误。

现有 `asset-agent scan host` 等旧语法只保留一个发布版本作为兼容别名，并输出迁移提示；正式命令统一为 `asset-agent host scan`。新部署和 CMDB 调用不得依赖旧语法。

### 6.3 常驻服务与人工扫描协调

当常驻服务正在运行时，人工扫描命令通过本地控制通道提交到服务内部队列，避免两个 root 扫描进程同时遍历服务器。Linux 首版使用权限为 `0600` 的 Unix Domain Socket。服务未运行时，命令可以使用相同模块和 Provider 独立执行一次性扫描。

## 7. 跨平台 Provider

业务模块通过能力接口访问平台数据：

```text
IdentityProvider
MetricsProvider
FilesystemProvider
DiskProvider
ProcessProvider
NetworkProvider
SocketProvider
PackageProvider
ContainerProvider
ServiceProvider
FileEvidenceProvider
EventProvider
ControlProvider
```

平台实现目录：

```text
providers/linux
providers/windows
providers/darwin
```

典型数据源：

| 平台 | 可选数据源 |
|---|---|
| Linux | procfs、sysfs、Netlink、cgroup、systemd D-Bus、包数据库、Docker/containerd/CRI |
| Windows | Win32 API、WMI/CIM、性能计数器、SCM、注册表、ETW、HCS |
| macOS | sysctl、libproc、IOKit、launchd、pkgutil、APFS、Docker |

模块声明需要 `ProcessProvider`，不关心实现来自 `/proc`、Win32 API 或 `libproc`。缺失必需能力必须返回 `unsupported`，不能以成功的空数组代替。

首版 Linux Provider 应支持宿主机与可访问容器作用域。容器 Provider 负责枚举容器、根文件系统、命名空间和运行时元数据，其他 Provider 使用统一 `scope_id` 在对应作用域采集。

## 8. 统一资产与证据模型

### 8.1 公共资产记录

JSON 字段、命令和配置键使用英文；设计、使用和运维文档使用中文。

机器可判断的错误码保持英文，面向运维人员的错误说明使用中文；CMDB 必须依据错误码和状态处理，不能解析自然语言错误说明。

```json
{
  "record_id": "stable-id",
  "record_type": "application",
  "host_id": "host-id",
  "scope_id": "host-or-container-id",
  "scope_type": "host",
  "name": "example",
  "version": "1.2.3",
  "vendor": "example-vendor",
  "platform": "linux",
  "states": {
    "installed": true,
    "running": true,
    "loaded": true,
    "exposed": true
  },
  "first_observed_at": "2026-08-13T10:00:00Z",
  "last_observed_at": "2026-08-13T10:01:00Z",
  "confidence": "exact",
  "attributes": {},
  "evidence": []
}
```

`installed`、`running`、`loaded`、`exposed` 必须分别表达。已安装但未运行的包、服务和组件仍是漏洞排查对象；`exposed` 表示存在可观测暴露证据，不等同于已经可被外网访问。

组件记录在公共字段之外优先输出：

```text
ecosystem
purl
cpe
package_source
artifact_digest
```

### 8.2 稳定标识

- 主机标识优先使用稳定硬件或云实例身份，结合 Machine ID 并记录回退策略；
- 容器使用运行时、集群或主机作用域及容器 ID；
- 进程使用 Boot ID、PID 和启动时钟值，不能只使用 PID；
- 其他资产使用主机、作用域、记录类型和平台稳定键生成确定性 ID；
- 显示名称、临时 PID、容器短名称和端口号不能单独作为稳定 ID。

### 8.3 证据

每项结论记录：

```text
provider
source_type
locator 或 locator_hash
observed_at
digest
confidence
```

证据置信度使用 `exact`、`strong`、`inferred`、`unknown`。直接 API、包数据库、文件清单、Socket inode 与文件描述符匹配可以产生直接证据；名称相似只能作为推断，不能覆盖直接证据。

### 8.4 关系

首版公共关系类型包括：

```text
runs_on
runs_in
contains
provided_by
consists_of
listens_on
exposes
depends_on
connects_to
loaded_by
installed_from
```

关系本身具有独立 `record_id`、来源、时间和置信度。漏洞影响反查的基本路径为：

```text
component → application → service/container → process → port → host
```

### 8.5 资源指标记录

资源时间序列不复用资产记录，而使用独立指标模型：

```json
{
  "metric_id": "stable-metric-id",
  "subject_id": "host-container-or-process-id",
  "subject_type": "host",
  "metric_name": "cpu.utilization",
  "unit": "percent",
  "interval_start": "2026-08-13T10:00:00Z",
  "interval_end": "2026-08-13T10:01:00Z",
  "sample_count": 6,
  "current": 12.4,
  "minimum": 8.1,
  "maximum": 27.6,
  "average": 14.3
}
```

主机级 CPU、Load、内存、Swap、磁盘 IO 和网络吞吐使用轻量计数器按分钟聚合。文件系统容量与 inode 按 30 分钟采集。已知容器和已跟踪服务进程可以使用独立子周期采样，但资源采样不得触发完整进程、容器、服务或组件枚举。资产周期和指标周期必须分别配置。

## 9. 完整性语义

模块状态统一为：

```text
complete
partial
degraded
failed
timeout
unsupported
```

完整性相对于模块声明的覆盖范围，而不是无法证明的“世界上所有资产”。模块必须在清单中报告：

- 预期和已访问的数据源；
- 预期和已完成的 host/container scope；
- 失败、超时、权限不足和不支持的 scope；
- 记录数、解析错误数和未知记录数；
- 本批次能否作为该模块的权威完整快照。

只有 `complete` 且 `authoritative: true` 的完整快照中缺失的旧记录，CMDB 才能判定为消失。`partial`、`failed`、`timeout` 和 `unsupported` 不能产生资产删除语义。

事件批次使用明确的 `upsert` 和 `delete` 操作。事件序列出现缺口时标记 `reconcile_required`，并安排对应模块完整校准。

## 10. 批次输出协议

### 10.1 协议版本

现有单文件 `asset-agent.snapshot` `1.0` 作为旧协议保留在 `docs/legacy`。新批次协议使用：

```text
schema_name: asset-agent.batch-manifest
schema_version: 2.0
```

资产、关系和指标记录分别使用版本化 Schema。模块描述符同时声明自己的数据 Schema 版本，允许模块独立演进。

### 10.2 目录结构

完整扫描：

```text
<output-dir>/inbox/
└── snapshot-20260813T120000Z-<scan-id>/
    ├── manifest.json
    ├── host-00001.jsonl
    ├── resource-00001.jsonl
    ├── process-00001.jsonl
    ├── container-00001.jsonl
    ├── component-00001.jsonl
    └── relationships-00001.jsonl
```

单模块扫描：

```text
<output-dir>/inbox/
└── module-component-20260813T120000Z-<scan-id>/
    ├── manifest.json
    ├── component-00001.jsonl
    └── relationships-00001.jsonl
```

批次类型：

- `snapshot`：完整校准，可在模块权威成功时用于消失判断；
- `delta`：事件触发的 `upsert/delete` 变化；
- `metrics`：资源时间序列，不参与资产删除判断；
- `module`：人工或定向单模块结果。

### 10.3 分片

- JSONL 默认在未压缩大小达到 64 MiB 或 100,000 条记录时分片，以先达到者为准；
- 单条记录默认最大 1 MiB，超过上限必须报告错误，不把配置原文等大对象塞入资产记录；
- 首版默认不压缩，避免在最低配置服务器上增加 CPU；
- 每个分片在 manifest 中记录名称、记录类型、记录数、字节数和 SHA-256；
- 输出层流式编码，不在内存中构建完整扫描文档。

### 10.4 原子发布

1. 在 `inbox` 同一文件系统创建 `.partial-<scan-id>`；
2. 流式写入分片并设置 `0600`；
3. 完成每个分片的 flush、sync、close 和 SHA-256；
4. 最后写入并同步 `manifest.json`；
5. 同步目录后，将临时目录原子重命名为正式批次目录；
6. CMDB 只读取正式目录且必须校验 manifest、记录数和摘要。

模块失败时可以发布诊断批次，但 manifest 必须明确标记状态和非权威性。CMDB 不得把缺失分片理解为零记录。

### 10.5 输出参数

生产方式：

```bash
asset-agent component scan --output-dir /var/lib/asset-agent/output
asset-agent all scan --output-dir /var/lib/asset-agent/output
```

- `--output-dir` 指定批次根目录；不存在时创建为 `0700`；
- `-o/--output` 仅保留给人工单模块单文件导出；
- `--output-dir` 与 `-o/--output` 互斥；
- `all scan -o` 不作为生产协议，返回清晰的用法错误；
- 文件发布成功后，标准输出仅打印正式批次目录绝对路径。

## 11. CMDB 消费确认与本地保留

输出根目录：

```text
<output-dir>/
├── inbox/
├── ack/
└── diagnostics/
```

CMDB 成功校验并入库后写入：

```text
<output-dir>/ack/<scan-id>.ack
```

确认文件：

```json
{
  "scan_id": "scan-id",
  "manifest_sha256": "sha256",
  "consumed_at": "2026-08-13T12:01:00Z"
}
```

扫描器只有在扫描 ID 和 manifest SHA-256 均匹配时才接受确认。

保留策略：

| 文件类型 | 未确认消费 | 确认消费后 |
|---|---|---|
| 完整快照 | 不删除 | 每个模块保留最新 2 份 |
| 增量事件 | 不删除 | 保留 24 小时后删除 |
| 资源指标 | 不删除 | 保留 24 小时后删除 |
| 失败诊断 | 保留 | 7 天且最多 100 批 |
| `.partial-*` | CMDB 不可读 | 重启识别或超过 24 小时后精确清理 |

完整扫描批次可能同时覆盖多个模块。删除一个已确认完整批次前，批次中每个 `authoritative` 模块都必须存在至少两份更新且已确认的完整结果；否则继续保留该批次。这样“每个模块保留最新 2 份”不会因为全量批次共享目录而破坏其他模块的回退点。

扫描器不自动删除未确认批次。磁盘压力达到硬限制时暂停普通输出和重型扫描，标记 `output_backlog` 与 `reconcile_required`，不得用静默删除换取继续运行。

## 12. 事件与独立调度

### 12.1 调度原则

- 每个模块描述符提供默认周期，配置文件可以覆盖；
- 周期属于模块，不写死在 CLI 或中心调度器；
- 同类事件在默认 10 秒窗口中合并；
- 事件只触发受影响模块和必要依赖；
- 周期任务加入随机抖动，避免公司服务器同时扫描；
- 事件是实时加速路径，周期完整扫描是防止丢事件的最终校准路径。

### 12.2 默认周期

默认周期以第 5 节模块表为准。资源模块每分钟输出主机级 CPU、内存、Load 和 Swap 聚合；已知容器与已跟踪服务进程的资源指标可以使用独立子周期采样，但不能触发完整进程资产遍历。文件系统和磁盘资产按 30 分钟采集；进程、容器和服务每 12 小时完整校准；组件和应用每 24 小时完整校准。

### 12.3 事件来源

Linux Provider 优先使用 Netlink、systemd、容器运行时事件和受限文件变化事件；能力不可用时使用轻量摘要轮询并在 manifest 中记录降级来源。事件不得替代完整扫描。

## 13. 资源保护

最低部署基线为 4 核 CPU、100 GB 磁盘，内存未知。实现必须以流式、限速和可暂停为基础，不能按当前 6.2 MB 旧样例估算未来容量。

默认保护：

- 重型模块并发数为 1；
- systemd `CPUQuota=10%`；
- `MemoryHigh=96M`、`MemoryMax=128M`；
- 低 CPU 和 IO 调度优先级；
- 每个模块独立超时、最大读取字节、最大记录和资源等级；
- 高负载时推迟重型模块，轻量资源采样和关键事件记录继续；
- 临界负载或内存压力下不强制运行重型扫描，只标记 `overdue` 和 `reconcile_required`；
- 高负载恢复后以低速执行积压任务；
- 禁止多个模块并发完整遍历进程或容器；
- 文件摘要按路径、大小、修改时间和已有摘要缓存，未变化不重复计算；
- 只沿进程、服务、容器、包和应用形成的证据链读取文件，不遍历整个磁盘。

磁盘保护同时检查：

- `minimum_free_percent`；
- `maximum_backlog_bytes`；
- 每个分片写入前的可用空间；
- 临时批次的实际累计大小。

达到高水位时暂停深度模块；达到硬限制时停止普通文件输出并通过 systemd journal 报告 `output_blocked`。

## 14. 常驻服务、心跳与故障恢复

- `asset-agent run` 作为受 systemd 约束的常驻服务运行；
- `resource` 每分钟指标批次兼作扫描器和主机心跳；
- `status` 输出最近完整快照、模块延迟、队列、事件序列、输出积压、磁盘水位和扫描器自身 CPU、内存、IO；
- 服务器宕机后扫描器无法继续输出，由 CMDB 根据最后心跳超时判定失联；
- CMDB 必须保留最后一个完整快照，主机失联不等于资产删除；
- 扫描器崩溃由 systemd 拉起，崩溃前 `.partial-*` 永不标记成功；
- 重启后记录 `aborted_scan`，校验本地 generation、sequence 和临时目录，再安排必要校准；
- 队列过载时合并为 `reconcile_required`，不能静默丢弃后继续声明数据完整。

## 15. Root 只读安全边界

- 扫描器以 root 只读运行；
- 核心进程不执行 Shell，也不执行任意外部命令；
- 禁止读取 `/proc/<pid>/environ`；
- 禁止读取或输出私钥、密码、Token、Cookie、连接凭据和配置原文；
- 只读取 Provider 与识别规则声明的白名单事实、包数据库和组件清单；
- 容器内读取遵循同样边界，不因为可访问容器根文件系统而扩大扫描范围；
- 允许的组件证据包括 `package.json`、锁文件、Java Manifest、Python dist-info、Go build info、包数据库、容器镜像元数据和服务 Unit；
- 敏感路径可只输出 `locator_hash`；
- 配置、控制 Socket、插件描述和输出目录必须校验所有权及权限；
- 脱敏在进入统一模型前完成，输出层不得承担首次脱敏责任。

## 16. 配置

默认配置文件：

```text
/etc/asset-agent/config.yaml
```

示例：

```yaml
output:
  directory: /var/lib/asset-agent/output
  minimum_free_percent: 10
  maximum_backlog_bytes: 10737418240

scheduler:
  concurrency: 1
  event_debounce: 10s
  load_shedding: true

modules:
  resource:
    enabled: true
    interval: 1m
  process:
    enabled: true
    interval: 12h
  component:
    enabled: true
    interval: 24h
```

`modules` 使用动态映射。新增模块后由模块描述符提供默认值，配置解析器不增加结构字段。未知模块配置必须报错，避免拼写错误被静默忽略。

## 17. 分阶段交付

### 阶段 1：扫描器内核

- 将现有 `host`、`network`、`process`、`socket` 迁移到动态注册表和 Provider；
- 将 `socket` 的监听语义拆为 `port`，连接语义拆为 `connection`；
- 实现模块优先命令和一个版本的旧命令兼容；
- 实现批次协议 2.0、JSONL 分片、manifest、原子目录发布和 `--output-dir`；
- 完整输出不再伪造未实现模块的空字段；
- Windows、macOS 构建返回能力级 `unsupported`，而不是散布 Linux 判断。

### 阶段 2：资源与常驻运行

- 实现 `resource`、`filesystem`、`disk`；
- 实现模块独立调度、常驻服务、控制通道、状态和 systemd 单元；
- 实现资源保护、磁盘水位和流式输出。

### 阶段 3：基础资产发现

- 实现 `package`、`container`、`service`、`port`、`connection`；
- 支持宿主机与容器 scope；
- 建立主机、容器、进程、服务和端口关系。

### 阶段 4：组件与业务系统

- 实现 `file`、`component`、`application`；
- 实现白名单证据读取、版本、PURL、CPE、框架和插件识别；
- 输出 `installed/running/loaded/exposed` 状态与完整证据链。

### 阶段 5：实时变化与交付可靠性

- 实现 EventProvider、定向增量扫描、generation 和 sequence；
- 实现消费确认、保留策略、积压保护和崩溃恢复；
- 用周期完整扫描校准事件缺口。

### 阶段 6：跨平台和外部插件

- 保持公共模块、命令和 Schema 不变，逐步实现 Windows 与 macOS Provider；
- 定义并验证受限独立进程插件协议。

## 18. 测试与验收

每一阶段必须包含：

1. 模块注册契约测试：测试模块注册后自动获得 `scan/describe/status/schedule`，CLI 无模块名称分支；
2. Provider 契约测试：相同 fixture 在不同 Provider 实现中生成统一模型；
3. 模块依赖测试：硬依赖、软依赖、降级、超时和 panic 隔离；
4. Schema 测试：记录字段、状态、证据、关系和版本稳定；
5. 输出故障注入：分片写入失败、磁盘满、崩溃和重启不能发布伪完整批次；
6. manifest 校验：记录数、字节数和 SHA-256 与分片一致；
7. 消费确认测试：错误扫描 ID 或摘要不能触发删除；
8. 保留测试：确认后保留两份完整快照，增量和指标等待 24 小时；
9. 调度虚拟时钟测试：周期、抖动、事件合并、积压和校准行为可重复；
10. 安全测试：不读取 environ、凭据和未声明路径，不执行 Shell；
11. 容量测试：使用大规模合成 Provider 验证流式分片和内存上限，不用旧样例大小推断容量；
12. Linux 集成测试：root 只读扫描、systemd 约束、容器 scope 和文件权限；
13. 跨平台构建测试：Linux、Windows、macOS 均可构建，缺失 Provider 明确报告 `unsupported`；
14. 最终执行 `go fmt`、`go vet`、`go test ./... -count=1` 和 Linux 静态构建验证。

## 19. 现有产物与文档迁移

本设计实施时同时纠正现有交付问题：

- 根目录旧 Linux 二进制在阶段 1 完成后使用当前源码、明确版本和提交信息重新构建；
- 现有 `docs/all-20260812T093156Z.json` 移入 `docs/legacy` 并标记为协议 1.0 旧样例，不伪造新字段；
- 新输出不再包含尚未执行或尚未实现模块的伪空字段；
- `--output-dir` 成为 CMDB 文件交付的正式入口；
- 旧设计中把 `watch`、固定调度、轮转或 CMDB 对接混入首版扫描命令的内容标记为历史设想；
- README 只描述已实现能力，规划模块必须明确标记为未实现；
- 在真实 Linux 环境生成协议 2.0 样例并以 manifest 校验，不能在 Windows 上伪造生产扫描数据。

## 20. 设计结论

扫描器采用“动态模块注册表 + 跨平台 Provider + 模块独立命令与调度 + 事件定向更新 + 周期完整校准 + 分片证据批次”的架构。它以最低服务器资源影响为优先，同时通过完整性、证据、稳定标识、状态和关系为未来 CMDB 提供可靠的漏洞与故障影响分析基础。
