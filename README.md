# 可扩展资产扫描器

这是一个以 root 只读运行的服务器资产扫描器。它负责采集主机资产、规范化数据并将正式批次原子写入本地目录；扫描器不连接 CMDB、不上传数据，也不执行来自 CMDB 的命令。公共 JSON 字段使用英文，文档和终端说明使用中文。

## 当前扫描模块

| 模块参数 | 采集内容 | 默认周期 | 硬依赖 |
|---|---|---:|---|
| `-host` | 主机名、发行版、内核、架构、内存总量、Boot ID、DMI UUID | 24 小时 | 无 |
| `-network` | 网卡、MAC、IP 地址、IPv4 路由、DNS 配置摘要 | 6 小时 | `host` |
| `-process` | PID、父进程、启动时钟、可执行文件、脱敏命令行、Cgroup、Namespace | 12 小时 | `host` |
| `-port` | TCP 监听端口、UDP 本地端口及进程归属 | 1 小时 | `process` |
| `-connection` | 已建立连接（ESTABLISHED 的 TCP/UDP）及进程归属 | 1 小时 | `port` |

模块参数由注册表动态生成。以后注册 `service` 模块后会自动获得 `-service` 参数，并自动进入全量扫描，不需要修改 CLI 的模块名称列表。

## 下载和安装

本仓库为私有仓库，需先登录 GitHub CLI（`gh auth login`），再按版本下载：

```bash
gh release download v0.4.1 -R Theearthwormsplitsvertically/scan -p asset-agent-linux-amd64
chmod +x asset-agent-linux-amd64
sudo install -m 0755 asset-agent-linux-amd64 /usr/local/bin/asset-agent
asset-agent version
```

扫描器在 Linux 上按 root 运行。`doctor` 可用于确认当前 Provider、权限和数据源能力：

```bash
sudo asset-agent doctor
```

## 扫描命令

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
sudo asset-agent -network -port
```

全量扫描：

```bash
sudo asset-agent scan
```

覆盖本次输出目录：

```bash
sudo asset-agent -host -output /data/cmdb
sudo asset-agent -network -port -output /data/cmdb
sudo asset-agent scan -output /data/cmdb
```

参数顺序不影响模块选择，重复模块参数自动去重。`scan` 不能与 `-<module>` 混用。

查看模块的支持状态、周期、资源等级、超时和依赖：

```bash
asset-agent modules
```

## 终端结果

扫描成功后终端打印简洁摘要，不打印完整 JSON：

```text
Asset Agent Scan
Modules: host, network
Status: complete

MODULE   STATUS    RECORDS  DURATION  ERROR
host     complete  1        8ms       -
network  complete  6        23ms      -

Output: /var/lib/asset-agent/output/inbox/module-multi-...
```

摘要用于人工查看。CMDB 的唯一机器接口是 `Output` 所指正式批次内的 `manifest.json` 和 JSONL 分片。

### CMDB 跨批次引用契约

`snapshot` 全量批次发布全部已注册模块，是新主机建立资产基线的自包含入口。`module` 单模块或组合批次只发布用户明确选择模块的记录，因此其中的关系允许引用本批次未重复发布的依赖资产。例如 `-port` 的 `listens_on.from_id` 可以引用此前完整批次中的稳定进程 `record_id`。

CMDB 必须按以下顺序消费：

1. 校验 `manifest.json`、文件记录数、字节数和 SHA-256；
2. 按 `host_id` 与稳定 `record_id` 幂等写入本批次资产；
3. 写入关系；端点暂时不存在时保存为待解析关系，不得仅因同批次缺少端点而拒绝整个批次；
4. 后续端点到达后重新解析待处理关系。

新主机首次接入、主机稳定身份变化或 CMDB 丢失该主机基线后，必须先执行一次 `sudo asset-agent scan`，成功入库后再启动周期单模块扫描。`module` 批次不对未发布模块产生删除语义；只有完整且权威的对应模块结果才能参与消失判断。

## 文件输出

Linux 默认输出根目录：

```text
/var/lib/asset-agent/output
└── inbox
    ├── module-host-<UTC>-<scan-id>
    ├── module-multi-<UTC>-<scan-id>
    └── snapshot-<UTC>-<scan-id>
```

发布过程先写入 `.partial-<scan-id>`，完成同步后原子改名为正式批次；`manifest.json` 最后写入。目录权限为 `0700`，文件权限为 `0600`。manifest 保存模块状态、覆盖范围、错误、记录数、字节数和 SHA-256。

单模块和组合扫描只发布用户明确选择模块的记录；为满足依赖而执行的模块不会重复采集，其状态仍写入 manifest。`scan` 动态发布全部已注册模块的 snapshot。

扫描器不会自动删除尚未被 CMDB 消费的正式批次。CMDB 应在完成校验和入库后负责归档或清理。

## 退出码

| 退出码 | 含义 |
|---:|---|
| `0` | 扫描执行且正式批次发布成功；部分模块不完整时仍以 manifest 和摘要表达 |
| `1` | 扫描无法执行或正式批次发布失败 |
| `2` | 命令或参数错误 |

## 本地开发验证

```bash
go test ./...
go vet ./...
VERSION=$(git describe --tags --always 2>/dev/null || echo dev)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags "-X github.com/Theearthwormsplitsvertically/scan/internal/buildinfo.Version=${VERSION} \
            -X github.com/Theearthwormsplitsvertically/scan/internal/buildinfo.Commit=$(git rev-parse --short HEAD) \
            -X github.com/Theearthwormsplitsvertically/scan/internal/buildinfo.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o asset-agent-linux-amd64 ./cmd/asset-agent
sudo ./scripts/verify-linux.sh ./asset-agent-linux-amd64
```

真实 Linux 验证脚本会运行五个单模块、组合扫描和全量扫描，并校验 schema `2.0`、目录和文件权限、JSONL 记录数、字节数、SHA-256、manifest 以及原子发布残留。
