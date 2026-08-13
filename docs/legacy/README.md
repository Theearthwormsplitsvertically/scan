# 历史协议样例说明

本目录用于本地隔离 2026-08-12 在真实 Linux 服务器生成的协议 1.0 单文件 JSON 样例。

- 样例只证明当时最小化扫描路径能够读取真实 Linux 事实；
- 约 6.2 MB 的旧样例不能作为新扫描器容量、内存、磁盘或网络规模基准；
- 协议 1.0 的 `asset-agent.snapshot`、顶层 `sockets` 和伪空未来字段已经停止使用；
- 真实资产明细可能包含主机名、IP、进程和路径，因此 `docs/legacy/*.json` 被 Git 忽略；
- 不允许在 Windows 上修改旧 JSON 伪造成协议 2.0，也不允许把 `unsupported` 诊断批次冒充真实 Linux 样例；
- 新协议样例必须由真实 Linux root 扫描生成，并用 `manifest.json` 的记录数、字节数和 SHA-256 校验。

当前协议与文件结构以项目根目录 `README.md` 和 `2026-08-13-extensible-asset-scanner-design.md` 为准。
