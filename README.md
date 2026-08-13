# 可扩展资产扫描器

这是一个以 root 只读运行的服务器资产扫描器。它只负责采集、规范化和原子写入本地文件，不连接 CMDB、不上传数据，也不执行 Shell。CMDB 后续只需读取指定输出目录中的正式批次并校验 `manifest.json`。

当前交付是扫描器内核第一阶段，公共 JSON 字段全部使用英文；本文档和命令说明使用中文。

## 当前已实现模块

| 模块 | 当前采集内容 | 默认完整校准周期 | 依赖 |
|---|---|---:|---|
| `host` | 主机身份、发行版、内核、CPU、内存、Machine ID、Boot ID、DMI | 24 小时 | 无 |
| `network` | 网卡、MAC、IP 地址、IPv4 路由、DNS 配置摘要 | 6 小时 | `host` |
| `process` | PID、父进程、启动时钟、可执行文件、脱敏命令行、Cgroup、Namespace | 12 小时 | `host` |
| `port` | TCP 监听端口、UDP 本地端口、进程归属 | 1 小时 | `process` |
| `connection` | 已建立 TCP 连接及进程归属 | 1 小时 | `port` |
| `all` | 动态编排全部已注册模块；它是虚拟目标，不是扫描模块 | 由外部调度决定 | 全部已注册模块 |

每个真实模块都有独立命令、描述、支持状态和周期信息：

```bash
./asset-agent modules
./asset-agent host describe
./asset-agent host status
./asset-agent host schedule
```

`all` 和单模块扫描都由注册表动态规划依赖。单独运行 `connection` 时会依次执行 `host → process → port → connection`，但只发布 `connection` 记录；同一次计划不会重复采集 Socket。

## 扫描命令

生产扫描使用模块优先语法和批次目录输出：

```bash
sudo ./asset-agent host scan --output-dir /var/lib/asset-agent/output
sudo ./asset-agent port scan --output-dir /var/lib/asset-agent/output
sudo ./asset-agent connection scan --output-dir /var/lib/asset-agent/output
sudo ./asset-agent all scan --output-dir /var/lib/asset-agent/output
```

未指定输出参数时，默认使用可执行文件同级的 `output` 目录。命令成功后，标准输出只打印正式批次目录的绝对路径。

人工调试单个模块时，可导出一个完整 `Batch` JSON 文件或标准输出：

```bash
sudo ./asset-agent host scan -o /tmp/host-batch.json
sudo ./asset-agent host scan -o - | jq .
```

`all scan` 不支持 `-o`，必须使用批次目录；`--output-dir` 与 `-o/--output` 互斥。

## 当前文件输出

完整扫描示例：

```text
/var/lib/asset-agent/output/
└── inbox/
    └── snapshot-20260813T120000Z-<scan-id>/
        ├── manifest.json
        ├── host-00001.jsonl
        ├── network-00001.jsonl
        ├── process-00001.jsonl
        ├── port-00001.jsonl
        ├── connection-00001.jsonl
        └── relationships-00001.jsonl
```

单模块示例：

```text
/var/lib/asset-agent/output/
└── inbox/
    └── module-port-20260813T120000Z-<scan-id>/
        ├── manifest.json
        ├── port-00001.jsonl
        └── relationships-00001.jsonl
```

只有存在记录时才生成对应 JSONL 分片；模块状态和覆盖范围始终写入 `manifest.json`，所以缺失分片不会被解释为“权威的零记录”。依赖模块会出现在 manifest 中，但单模块批次不会重复发布依赖记录。

输出协议：

- manifest：`schema_name = asset-agent.batch-manifest`，`schema_version = 2.0`；
- 分片：每行一个 JSON 对象，默认每片最多 100,000 条或 64 MiB，单行最多 1 MiB；
- manifest 为每个分片记录 `name`、`module`、`record_type`、`records`、`bytes` 和 `sha256`；
- 资产记录包含 `record_id`、`record_type`、`host_id`、`scope_id`、`states`、`attributes` 和 `evidence`；
- 关系记录包含 `relationship_type`、`from_id`、`to_id`、`observed_at`、`confidence` 和 `evidence`；
- 输出根目录、`inbox` 和批次目录权限为 `0700`，文件权限为 `0600`；
- 扫描器先写 `inbox/.partial-<scan-id>`，全部分片与 manifest 同步后才原子改名为正式目录；CMDB 不得读取 `.partial-*`。

当前阶段只负责可靠发布，不实现 CMDB 确认和自动删除。扫描器不会自动删除未消费批次；基于 ack 的“每模块保留最新两份完整快照”和其他保留策略属于后续交付。

## 完整性语义

模块状态为 `complete`、`partial`、`degraded`、`failed`、`timeout` 或 `unsupported`。只有 `complete` 且 `authoritative: true` 的模块结果才允许未来 CMDB 推断旧资产消失；其他状态只能用于增加或更新已确认事实，不能产生删除语义。

扫描异常按模块隔离。模块 panic、超时、缺 Provider 和硬依赖失败都会写入结构化错误，不会伪装为成功空数组，也不会清除同批次已经成功的模块。

## 平台和扩展方式

当前 Linux Provider 使用 procfs、sysfs 和 Go 标准库。Windows 与 macOS 构建加载相同模块和命令，但在对应 Provider 实现前会明确报告 `unsupported`。

新增模块时实现统一 `Module` 接口并注册；新增系统支持时实现相同的强类型 Provider 契约。CLI、`all` 和输出层不需要增加模块名称 switch。

以下已批准模块属于后续阶段，当前二进制不会把它们伪造成空字段：

- `resource`：CPU、内存、Load、Swap 等当前资源指标，目标周期 10 分钟；
- `filesystem`、`disk`：目标周期 30 分钟；
- `package`：目标周期 24 小时；
- `container`、`service`：目标周期 12 小时；
- `file`、`component`、`application`：目标周期 24 小时；
- 容器内进程、服务依赖、组件证据、事件增量、常驻调度、消费确认和保留清理。

## 旧命令迁移

以下旧语法仅保留一个迁移版本，并向标准错误输出 `deprecated`：

```bash
asset-agent scan       # 等价于 asset-agent all scan
asset-agent scan all  # 等价于 asset-agent all scan
asset-agent scan host # 等价于 asset-agent host scan
```

旧 `asset-agent scan socket` 不会静默映射，因为它原来同时包含端口和连接。该命令返回退出码 2；请分别使用 `asset-agent port scan` 和 `asset-agent connection scan`。

## 安全边界

- 生产环境以 root 只读运行；
- 不调用 Shell，不执行任意外部命令；
- 不读取 `/proc/<pid>/environ`；
- 命令行密码、Token、Authorization、API Key 和 URL 凭据会脱敏；
- DNS 配置只输出摘要，不输出原文；
- 不修改服务、进程、网络、防火墙或容器；
- 不连接 CMDB、HTTP 服务或消息队列。

## 构建和真实 Linux 验证

```powershell
$goExe = 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
& $goExe build -trimpath -o dist/asset-agent-linux-amd64 ./cmd/asset-agent
```

复制到 Linux 后必须用 root 执行验证：

```bash
chmod 0755 asset-agent-linux-amd64
sudo sh scripts/verify-linux.sh ./asset-agent-linux-amd64
```

验证脚本检查协议 2.0、正式目录、权限、分片记录数、字节数和 SHA-256。真实 Linux 样例只能在真实 Linux 上生成，不能用 Windows 的 `unsupported` 诊断批次冒充生产数据。
