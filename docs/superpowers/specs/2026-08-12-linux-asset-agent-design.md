# Linux 资产采集 Agent 设计规格

日期：2026-08-12  
状态：设计已确认，待书面规格审阅

## 1. 背景与目标

本项目首先实现一个运行在 Linux 服务器上的只读资产采集 Agent，用真实生产服务器验证资产信息采集能力、准确性、增量更新能力和资源开销。

Agent 需要回答以下技术问题：

- 服务器是什么发行版、内核、架构和硬件；
- 有哪些网卡、IP、路由和网络命名空间；
- 哪些 TCP/UDP 端口处于监听状态；
- 每个 Socket 属于哪个 PID、进程、systemd 服务、软件包或容器；
- 运行文件的版本、文件身份和 SHA-256 是什么；
- 当前进程正在连接哪些目标 IP 和端口；
- 识别出的 Nginx 实例启用了哪些模块、监听项和上游；
- 发生变化后，能否仅重新采集变化部分；
- 整轮采集和每个采集器消耗了多少 CPU、内存、IO 和时间。

首版以单机 PoC 为目标，结果写入本地 JSON/JSONL，不实现远程通信。Agent 的职责止于 Linux 主机内部的资产发现、直接关系关联、脱敏、变化检测和本地报告；采集结果后续由什么系统消费，不属于本规格，也不得反向影响采集逻辑。

## 2. 已确认的运行边界

- 目标平台为 Linux，Agent 必须自动识别发行版、内核和架构；
- PoC 允许以 `root` 身份只读运行；
- Agent 可以读取已识别服务的白名单配置文件；
- 配置只在本机解析，不输出配置原文、密码、Token、私钥或环境变量；
- 采用混合采集：首次基线、持续变化感知、定向重采、周期校准；
- 关键资产变化立即轻量采集；
- 计划增量扫描每 72 小时执行一次；
- 普通全量扫描每 168 小时执行一次；
- 首版使用 Go 编写并交付为单个可执行文件。

## 3. 首版范围

### 3.1 包含

- Linux 环境和能力自动识别；
- 主机、OS、硬件、网络信息；
- 进程、Socket、监听端口和当前连接；
- 端口到 PID、服务、软件包和容器的关联；
- systemd 服务采集及非 systemd 环境的能力降级；
- dpkg、rpm、apk、pacman 软件包采集；
- Docker 和 containerd 运行时采集；
- 运行文件身份、按需 SHA-256 和监听进程动态库；
- Nginx 识别及白名单深度采集；
- Netlink、inotify、systemd、容器事件和轻量摘要变化感知；
- 本地状态、全量 JSON、增量 JSONL、资源用量和错误输出；
- 数据脱敏、资源限制、任务超时、熔断和恢复。

### 3.2 不包含

- 远程控制面、集中接收端和网络上传；
- 主动 Nmap 或本机端口扫描；
- 任意远程命令或 Shell 执行；
- eBPF 程序；
- 全盘文件遍历；
- Java、数据库、Jenkins 等应用的深度采集器；
- 容器镜像或文件系统的深度 SBOM；
- 每次请求、函数调用或系统调用级别的行为分析。

Java、数据库等应用深度采集、SBOM 和 eBPF 可以作为 Agent 后续能力；远程控制、集中传输及任何下游系统适配不属于本规格。

## 4. 命令行接口

Agent 提供以下运行模式：

```bash
asset-agent doctor
asset-agent scan
asset-agent watch
asset-agent version
```

### 4.1 `doctor`

只检查运行环境和采集能力，不执行深度资产采集。输出包括：

- OS、内核、架构；
- root 和 Linux capabilities；
- `/proc`、`/sys`、SockDiag、Process Connector、Netlink、inotify 可用性；
- init 系统；
- 包管理器；
- Docker/containerd；
- SELinux/AppArmor；
- cgroup v1/v2；
- 可用采集器和降级路径。

### 4.2 `scan`

执行一次普通全量采集并输出完整 JSON。用户可以使用 `--output` 指定输出路径。用户明确指定的文件不由 Agent 自动删除。

### 4.3 `watch`

启动时恢复或建立基线，随后持续监听变化、执行关键轻采、按计划执行增量和全量扫描，并输出增量 JSONL。

## 5. 总体架构

```text
Agent Core
├─ CapabilityDetector
├─ Scheduler
├─ ResourceGovernor
├─ Collectors
│  ├─ HostCollector
│  ├─ NetworkCollector
│  ├─ ProcessCollector
│  ├─ SocketCollector
│  ├─ ServiceCollector
│  ├─ PackageCollector
│  └─ ContainerCollector
├─ Correlator
├─ Recognizers
├─ DeepCollectors
│  └─ NginxCollector
├─ Watchers
│  ├─ NetlinkWatcher
│  ├─ ProcessWatcher
│  ├─ InotifyWatcher
│  ├─ SystemdWatcher
│  └─ ContainerWatcher
├─ EventCoalescer
├─ DiffEngine
├─ Redactor
├─ StateStore
└─ Reporter
```

主要数据流：

```text
系统能力识别
→ 基础采集
→ 对象关联
→ 应用识别
→ 专用深度采集
→ 规范化和脱敏
→ 差异计算
→ 本地 JSON/JSONL
```

变化处理流：

```text
内核或应用事件
→ 变化提示
→ 去重和去抖
→ 定向任务队列
→ 重新读取操作系统事实
→ 差异计算
→ 增量 JSONL
```

事件只表示“可能发生变化”，不能直接替代事实采集。

## 6. 系统能力自动识别

Agent 启动时读取或检测：

```text
/etc/os-release
uname
/etc/machine-id
/proc/sys/kernel/random/boot_id
/sys/class/dmi/id/*
/proc/1/cgroup
/proc/filesystems
/proc/self/mountinfo
init进程和systemd D-Bus
cgroup版本
SELinux/AppArmor状态
Docker/containerd socket
SockDiag、Process Connector、Netlink、inotify能力
```

不支持的能力必须显式报告为 `unsupported` 或 `degraded`，并在存在安全降级路径时自动切换。单项能力缺失不得导致整个 Agent 退出。

## 7. 首次和普通全量采集

普通全量表示执行首版全部采集器，不表示递归扫描整个磁盘。

### 7.1 主机与系统

主要来源：

- `/etc/os-release`；
- `uname` 系统调用；
- machine-id、boot-id、DMI；
- `/proc/cpuinfo`、`/proc/meminfo`；
- `/sys/block` 和挂载信息。

采集发行版、内核、架构、启动标识、主机标识、厂商型号、CPU、内存、磁盘和挂载。

### 7.2 网络

优先通过 Netlink 获取：

- 网卡、MAC 和状态；
- IPv4/IPv6 地址；
- 路由；
- 网络命名空间；
- DNS 配置摘要。

### 7.3 进程

从 `/proc/<pid>` 读取：

- PID、PPID 和启动时间；
- UID/GID 和运行用户；
- 进程名称、状态；
- 可执行路径；
- 命令行，写出前脱敏；
- 工作目录、根目录；
- cgroup 和 namespace；
- systemd、容器归属线索。

进程稳定身份使用：

```text
boot-id + PID + process starttime
```

### 7.4 Socket 与端口

优先使用 `NETLINK_SOCK_DIAG` 获取 TCP/UDP、IPv4/IPv6 Socket，必要时以 `/proc/net/tcp*` 和 `/proc/net/udp*` 降级。

通过 `/proc/<pid>/fd` 的 Socket inode 建立：

```text
IP:协议:端口
→ Socket inode
→ 一个或多个PID
→ 进程
```

采集监听 Socket 和扫描时存在的连接。当前连接只能证明观测时刻存在，不能直接推定长期业务依赖。

### 7.5 systemd 服务

优先通过 systemd D-Bus 和 unit 信息采集：

- unit 名称；
- active/sub 状态；
- MainPID；
- ExecStart；
- unit 文件；
- 启用状态。

非 systemd 系统仅采集能够可靠确定的 init 信息，并报告降级状态。

### 7.6 软件包

支持：

- dpkg；
- rpm；
- apk；
- pacman。

采集包管理器、名称、版本、架构、厂商和安装状态。优先读取稳定数据库格式；必须调用系统程序时，直接执行固定绝对路径和固定参数，不经过 Shell。

### 7.7 容器

通过 Docker/containerd API、cgroup 和 namespace 采集：

- runtime 和容器 ID；
- 名称、状态；
- 镜像名称和 digest；
- 标签，敏感键值脱敏；
- 端口映射；
- 挂载；
- 容器内进程和宿主 PID；
- 容器网络命名空间。

### 7.8 文件与动态组件

- 为正在运行的可执行文件采集 device、inode、size、mtime 和 SHA-256；
- 相同文件身份在同一轮扫描中只哈希一次；
- 读取监听服务进程的 `/proc/<pid>/maps`，采集已加载动态库；
- 不递归扫描根目录；
- 每周全量重新核验运行文件和已识别应用配置，但仍受 IO 和时间限制。

## 8. 对象关联

Correlator 建立有直接系统证据的关系：

```text
Socket inode → PID
PID → 可执行文件
PID → systemd unit
可执行文件 → 软件包
PID → cgroup/namespace → 容器
应用进程 → 监听端点
应用进程 → 当前远端连接
```

每条关系记录来源、采集器、采集时间和可信度。名称相似、端口惯例或路径猜测不得标为精确关系。

## 9. Nginx 深度采集

满足以下任一条件后启用：

- 进程名或执行文件匹配 Nginx；
- 软件包匹配 Nginx；
- systemd unit 指向 Nginx。

采集：

- 版本；
- 二进制路径和哈希；
- 编译参数；
- 静态和动态模块；
- master/worker 关系；
- 监听地址和端口；
- 配置入口和 include 树；
- server_name；
- upstream；
- proxy_pass；
- 公钥证书路径、摘要、主体、颁发者和有效期。

不使用会输出全部配置原文的 `nginx -T`。配置解析器只读取规范化后的白名单配置树，限制单文件大小、include 深度和文件数量，检测循环 include，并拒绝越界路径。认证头、密码、Token、Cookie、私钥和凭据值不得输出。

## 10. 变化感知与定向重采

### 10.1 变化来源

| 域 | 首选 | 降级 |
|---|---|---|
| 进程 | Process Connector | 周期比较 PID + starttime |
| Socket | SockDiag 轻量摘要 | `/proc/net/*` 摘要 |
| IP/网卡/路由 | Netlink RTM | 周期 Netlink 摘要 |
| systemd | D-Bus 信号 | 状态摘要 |
| 配置/插件 | inotify | 路径元数据摘要 |
| 软件包 | 包数据库 inotify | 包数据库摘要 |
| Docker | Docker Events | 容器列表摘要 |
| containerd | containerd Events | 容器列表摘要 |

### 10.2 两阶段端口检测

```text
读取SockDiag监听Socket
→ 计算IP、协议、端口、inode摘要
→ 摘要未变化：结束
→ 摘要变化：遍历/proc/<pid>/fd并重建关系
```

### 10.3 事件合并

所有变化转换为包含域、对象、类型、来源和时间的内部提示。

- 按“采集域 + 对象”去重；
- 默认等待 3 至 10 秒稳定窗口；
- 相同对象同时最多运行一个任务；
- 执行期间再次变化，完成后最多补跑一次；
- 使用有界队列；
- 队列压力高时优先保留端口、进程、IP 和容器变化；
- 连续失败使用退避。

### 10.4 删除判定

只有定向重采成功后才能生成 `removed`。采集器失败、超时或权限不足时保留上次成功状态，避免误报大规模资产消失。

## 11. 调度策略

### 11.1 持续变化感知

Agent 常驻监听低成本事件。关键变化在去抖后立即执行轻量定向采集：

- 新监听端口；
- 新进程；
- 新 IP 或路由；
- 新容器；
- 服务状态变化；
- 已识别应用配置变化。

降级时的默认轻量检查：

- 进程 PID + starttime：10 秒；
- Socket 摘要：15 秒。

这些检查只产生变化提示，不等同于完整增量扫描。

### 11.2 计划增量扫描

每 72 小时执行一次：

```text
检查全部采集域摘要
→ 只深采变化或标记为dirty的域
→ 输出added/modified/removed
```

配置：

```text
interval: 72h
jitter: 30m
max_delay: 12h
```

### 11.3 普通全量扫描

每 168 小时执行一次，默认窗口为本地时间 02:00 至 05:00：

```text
interval: 168h
window: 02:00-05:00
jitter: 60m
max_delay: 24h
```

全量执行所有首版采集器并重新校准完整状态，但不进行全盘文件遍历。

若计划增量距离全量不足 12 小时，则跳过增量并等待全量。全量成功后重置下一次增量的计时。

示例：

```text
第0天：全量
第3天：增量
第6天：增量
第7天：全量并重置计时
```

### 11.4 非计划恢复扫描

以下情况不等待 72 小时：

- 首次运行且没有有效状态：普通全量；
- boot-id 变化：进程、端口、服务、容器和网络域校准；
- inotify 溢出：配置域校准；
- Process Connector 中断：进程和端口域校准；
- Netlink 中断：网络域校准；
- 容器事件流重连：容器域校准；
- 状态文件损坏或模式不兼容：普通全量；
- 管理员在本机手动执行 `scan`。

## 12. 资源保护

### 12.1 内部限制

- 轻量采集器最大并发 2；
- 深度采集器最大并发 1；
- 外部程序最大并发 1；
- 有界队列；
- 所有任务使用 context 取消和超时；
- 单个采集器 panic 在边界捕获；
- 流式处理文件和 JSON，避免大对象常驻内存。

### 12.2 systemd/cgroup 初始限制

```ini
CPUQuota=10%
MemoryMax=256M
IOWeight=10
Nice=10
IOSchedulingClass=idle
TasksMax=64
OOMScoreAdjust=500
```

### 12.3 默认超时

| 采集器 | 超时 |
|---|---:|
| 主机、网络 | 15 秒 |
| 进程、Socket | 30 秒 |
| systemd | 30 秒 |
| 软件包 | 120 秒 |
| Docker/containerd | 60 秒 |
| Nginx | 30 秒 |
| 单文件哈希 | 30 秒 |

### 12.4 负载感知

Agent 读取 load、可用内存、PSI 和自身资源数据。负载升高时依次：

1. 降低并发和轻量检查频率；
2. 暂停哈希、配置解析、软件包和深度采集；
3. 只保留变化信号和最低成本检查；
4. 压力恢复并稳定一段时间后逐步恢复。

增量最多因负载推迟 12 小时，全量最多推迟 24 小时。

## 13. 安全设计

- PoC 不监听任何网络端口；
- 不主动访问互联网；
- 不修改服务器文件、服务、进程、网络和防火墙；
- 不安装软件；
- 不加载内核模块或 eBPF；
- 不接受远程控制指令；
- 不经过 Shell 执行外部命令；
- 外部命令必须是固定绝对路径、固定参数和独立超时；
- 默认不读取或输出进程环境变量；
- 命令行中的密码、Token、Secret、Authorization 和 URL 凭据必须脱敏；
- Docker 标签按敏感键规则脱敏；
- 配置解析只输出允许的结构化字段；
- 私钥和凭据文件不得读取；
- 文件路径必须规范化并限制在采集器允许范围；
- 对软链接、超大文件、循环 include 和异常 `/proc` 数据安全失败。

## 14. 本地文件与保留策略

默认目录：

```text
/var/lib/asset-agent/
├─ state/
│  └─ state.db
└─ reports/
   ├─ snapshots/
   └─ events/

/var/log/asset-agent/
/run/asset-agent/
```

权限：目录 `0700`，文件 `0600`，所有者 `root`。

保留策略：

| 数据 | 策略 |
|---|---|
| 当前状态 | 持续保留，不参与历史轮转 |
| 最新成功全量 | 始终保留 |
| 历史全量 | 最近 4 份，最长约 28 天 |
| 增量 JSONL | 按天或 100 MiB 轮转，保留 30 天 |
| 运行日志 | 保留 14 天 |
| 临时未完成文件 | 启动时安全清理 |
| Agent 管理数据总量 | 默认上限 1 GiB |

达到上限时依次删除过期日志、过期增量和最旧历史全量；不得删除当前状态和最新成功全量。仍无法释放空间时停止生成新报告，但保留低成本变化感知并报告磁盘错误。

用户通过 `scan --output <path>` 指定的输出不参与自动删除。

## 15. 输出协议

### 15.1 全量 JSON

文件名：

```text
snapshot-<host-id>-<timestamp>.json
```

顶层结构：

```json
{
  "schema_version": "1.0",
  "scan": {},
  "agent": {},
  "capabilities": {},
  "host": {},
  "network_interfaces": [],
  "addresses": [],
  "routes": [],
  "processes": [],
  "sockets": [],
  "services": [],
  "packages": [],
  "containers": [],
  "files": [],
  "applications": [],
  "relationships": [],
  "collector_status": [],
  "resource_usage": {}
}
```

### 15.2 增量 JSONL

每行一个事件，动作类型：

```text
added
modified
removed
collector_recovered
collector_degraded
```

每个对象或关系必须包含观测时间、采集器、系统来源和可信度。

### 15.3 稳定身份

```text
主机：machine-id + DMI UUID
进程：boot-id + PID + starttime
Socket：network namespace + inode + protocol
监听端点：network namespace + IP + protocol + port
服务：init system + unit name
软件包：package manager + name + version + architecture
容器：runtime + container ID
文件：device + inode + size + mtime，附SHA-256
```

## 16. 错误处理与恢复

- 每个采集器独立返回 `ok`、`partial`、`degraded`、`failed` 或 `timeout`；
- 所有错误包含原因、采集时间、耗时和降级路径；
- 扫描期间退出的进程短暂重试一次，仍失败则标记竞态；
- 采集失败不产生删除事件；
- 外部命令超时终止对应进程；
- 状态损坏后隔离损坏文件并重建全量；
- 事件队列溢出后执行对应域校准；
- SIGTERM 停止接收新任务，在安全点退出；
- systemd 对整体崩溃限速重启，避免崩溃循环。

## 17. 自身资源观测

每轮和每个采集器记录：

- 开始、结束和墙钟耗时；
- 用户态、内核态 CPU；
- RSS 和峰值；
- 读写字节；
- 采集对象数量；
- 错误和超时；
- 降级、限速和推迟原因。

## 18. 测试策略

### 18.1 单元测试

- os-release、DMI、`/proc` 解析；
- SockDiag 和 `/proc/net` 规范化；
- PID/starttime 身份；
- dpkg、rpm、apk、pacman 输出；
- systemd 属性；
- Docker/containerd 数据；
- Nginx 配置、include、模块和脱敏；
- DiffEngine、事件合并、轮转和保留。

### 18.2 集成测试

- 真实 Linux 进程、TCP/UDP、IPv4/IPv6；
- 多进程共享 Socket；
- 进程重启和 PID 复用；
- systemd 服务；
- Docker/containerd；
- Agent 重启和状态恢复；
- 只读 root 运行。

### 18.3 安全与韧性测试

- 命令行和配置敏感字段脱敏；
- 软链接和路径越界；
- 循环 include；
- 超大文件；
- 恶意或截断的 `/proc` 数据；
- 事件风暴和队列上限；
- 状态损坏；
- 采集器超时和 panic；
- 磁盘不足和报告轮转。

## 19. PoC 验证方法

Agent 输出与相邻时间执行的系统原生命令对照：

| Agent 域 | 对照 |
|---|---|
| OS/内核 | `uname`、`/etc/os-release` |
| IP/路由 | `ip addr`、`ip route` |
| 监听端口/PID | `ss -lntup` |
| 进程 | `ps`、`/proc/<pid>` |
| systemd | `systemctl show` |
| 软件包 | `rpm -qa`、`dpkg-query` 或 `apk info` |
| 容器 | `docker inspect`、`ctr` 或 `crictl` |
| Nginx | `nginx -V` 和允许字段的人工检查 |

经变更窗口确认后，可人工启动一个只监听 `127.0.0.1` 的临时进程，验证新增端口、PID、停止和删除事件。Agent 自己不得创建测试进程或修改业务配置。

## 20. 首版验收标准

功能：

- 自动识别 Linux 环境和采集能力；
- 持续监听端口与相邻时间的 `ss` 对照一致，竞态项有明确解释；
- 可映射端口关联到正确 PID、进程、服务、包或容器；
- Nginx 存在时完成约定的深度采集；
- 关键变化立即轻采；
- 计划增量按 72 小时策略执行；
- 普通全量按 168 小时策略执行；
- 变化后只重采相关域；
- 事件中断、状态损坏和重启后能够恢复；
- 单个采集器失败不影响其他采集器。

性能初始目标：

```text
空闲平均CPU           ≤ 1%
常驻内存RSS           ≤ 100 MiB
硬内存上限            256 MiB
CPU硬限制             10%
持续端口/进程变化发现 ≤ 30秒
IP/路由事件发现       ≤ 5秒
配置变化发现          ≤ 10秒
容器事件发现          ≤ 10秒
```

安全：

- 不产生监听端口和外部网络请求；
- 不修改服务器状态；
- 不通过 Shell 执行命令；
- 输出中不存在密码、Token、私钥和环境变量；
- 异常输入不能导致越界读取、无限资源消耗或 Agent 崩溃。

## 21. 实施原则

- 先实现 `doctor` 和一次性 `scan`，验证原始采集准确性；
- 再实现关联和 Nginx 深度采集；
- 在静态采集稳定后实现状态、DiffEngine 和 `watch`；
- 最后启用计划调度、轮转和 systemd/cgroup 生产保护；
- 每个阶段都必须在模拟环境通过测试后，才能在真实服务服务器上验证。
