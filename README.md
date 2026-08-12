# Linux Asset Agent

一个只读的 Linux 主机资产采集 Agent。当前版本是单机 PoC：不会监听端口、不会上传数据、不会调用 Shell 或修改被采集服务器的服务、进程、网络和防火墙。

## 当前交付范围

- `asset-agent version`：输出 Agent 版本 JSON；
- `asset-agent doctor`：识别 Linux 发行版、内核、架构、root、`/proc`、`/sys`、init、cgroup、SELinux、AppArmor、Docker/containerd socket 及可用/降级能力；
- `asset-agent scan`：生成一次完整 JSON，采集主机、网络接口/IP、IPv4 路由、进程、TCP/UDP Socket、监听端口、当前连接，以及 Socket inode 到 PID/进程的直接证据关系；
- 命令行中的密码、Token、API key、Authorization 和 URL 凭据会脱敏；进程环境变量不会读取；
- 单个采集域失败只影响自身状态，不删除或掩盖其他成功采集到的数据。

当前尚未实现：`watch`、72 小时增量/168 小时全量调度、systemd/软件包/容器归属、运行文件哈希/动态库、Nginx 深度采集、状态保留与轮转。这些会在后续里程碑中加入。

## 构建 Linux 可执行文件

在 Windows PowerShell 的项目目录运行：

```powershell
$goExe = 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
& $goExe build -trimpath -ldflags '-s -w' -o dist/asset-agent-linux-amd64 ./cmd/asset-agent
```

把 `dist/asset-agent-linux-amd64` 通过受控发布渠道放到目标 Linux 服务器后，赋予可执行权限：

```bash
chmod 0755 asset-agent-linux-amd64
```

## 在 Linux 上验证

推荐在变更窗口中、使用 root 或具备读取 `/proc/<pid>/fd` 权限的账户执行。Agent 读取系统事实，不创建测试进程，也不改动业务配置。

```bash
sudo ./asset-agent-linux-amd64 version
sudo ./asset-agent-linux-amd64 doctor | jq .
sudo ./asset-agent-linux-amd64 scan --output ./snapshot.json
jq '.host, .addresses, [.sockets[] | select(.state == "LISTEN")]' ./snapshot.json
ss -lntup
```

`scan --output <path>` 写入用户指定文件；这个文件不受 Agent 的自动清理策略影响。未指定 `--output` 或指定 `--output -` 时，JSON 写到标准输出。

可以使用以下只读验证脚本对照 Agent 与原生系统事实：

```bash
sudo bash scripts/verify-linux.sh ./asset-agent-linux-amd64
```

脚本只在 `mktemp` 创建的临时目录写入 Agent 报告，在结束时清理该目录；不会修改业务文件或服务。

## JSON 中的关键关系

每个 Socket 都以 Linux socket inode 作为证据锚点。Agent 读取 `/proc/net/tcp*` 与 `/proc/net/udp*` 得到端点和 inode，再读取 `/proc/<pid>/fd` 中严格匹配 `socket:[inode]` 的链接，输出：

```text
network namespace + socket inode + protocol
→ PID
→ process identity (boot-id + PID + starttime)
```

`relationships` 中只有通过这条直接证据链确认的关系才会标为 `confidence: "exact"`；不会依据端口名称或进程名猜测归属。

