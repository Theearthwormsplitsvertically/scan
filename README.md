# Linux Asset Agent

## 扫描前置画像与策略选择

每次执行 `scan`（包括单模块和 `all`）都会先执行一次只读系统画像。Agent 识别发行版、版本、内核、架构、init、cgroup、安全组件、容器运行时和可用数据源，然后再为所选模块选择采集后端。

报告中的 `system_profile` 保存本次判断依据，`strategies` 保存每个模块的实际 `backend`、所需数据源、缺失数据源、策略状态和降级原因。缺少不可替代的数据源时，模块不会被盲目执行，而是在 `collector_status` 中标记为 `unsupported`。

当前策略：

- `host`：优先使用 `procfs_sysfs`，单一来源可降级采集；
- `network`：使用 Go 标准库获取网卡/IP，存在 procfs 时补充路由；
- `process`：要求 procfs；
- `socket`：要求 procfs 和 proc_net，当前使用 `/proc/net` 后端，并记录相对 sock_diag 的降级原因。

一个只读的 Linux 服务器资产采集 Agent。v0.2.0 采用“具体功能、具体扫描模块”的设计：主机、网络、进程和端口可独立扫描，`all` 只负责统一编排。

## 扫描模块

```bash
sudo ./asset-agent scan host
sudo ./asset-agent scan network
sudo ./asset-agent scan process
sudo ./asset-agent scan socket
sudo ./asset-agent scan all
```

`scan` 是 `scan all` 的简写：

```bash
sudo ./asset-agent scan
```

模块职责：

- `host`：主机身份、发行版、内核、CPU、内存、Machine ID、Boot ID 和 DMI；
- `network`：网卡、MAC、IP 地址和 IPv4 路由；
- `process`：PID、可执行文件、脱敏命令行、Cgroup 和 Namespace；
- `socket`：TCP/UDP、监听端口、当前连接以及 Socket inode 到 PID 的精确关系；
- `all`：编排全部已实现模块，生成完整 Snapshot。

`service`、`package`、`container`、`application`、`file` 和 `security` 已进入设计，但尚未实现。

## 输出位置

不指定输出参数时，报告自动写入 Agent 可执行文件同级的 `output`：

```text
/opt/asset-agent/asset-agent
/opt/asset-agent/output/socket-20260812T090000Z.json
```

成功后命令会打印最终报告绝对路径。目录权限为 `0700`，报告权限为 `0600`。

指定输出文件：

```bash
sudo ./asset-agent scan socket -o /tmp/socket.json
sudo ./asset-agent scan socket --output /tmp/socket.json
```

显式输出到标准输出：

```bash
sudo ./asset-agent scan network -o - | jq .
```

## 版本和环境诊断

```bash
./asset-agent version
sudo ./asset-agent doctor | jq .
./asset-agent scan --help
```

## Linux amd64 构建

在 Windows PowerShell 的项目目录运行：

```powershell
$goExe = 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe'
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
& $goExe build -trimpath -ldflags '-s -w' -o dist/asset-agent-linux-amd64 ./cmd/asset-agent
```

复制到 Linux 后：

```bash
chmod 0755 asset-agent-linux-amd64
sudo ./asset-agent-linux-amd64 doctor
sudo ./asset-agent-linux-amd64 scan socket
```

## 自动验证

```bash
sudo sh scripts/verify-linux.sh ./asset-agent-linux-amd64
```

脚本会复制二进制到一个权限为 `0700` 的受控测试目录，执行五种扫描命令，并保留 JSON 结果。脚本最后会打印报告目录。

Agent 不监听端口、不上传数据、不调用 Shell、不读取进程环境变量，也不修改服务、进程、网络或防火墙。命令行中的密码、Token、API Key、Authorization 和 URL 凭据会脱敏。
