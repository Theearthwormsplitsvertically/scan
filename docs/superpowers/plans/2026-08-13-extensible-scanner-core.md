# 可扩展扫描器内核实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** 交付设计规格第一阶段：把现有固定 host/network/process/socket 扫描器迁移为动态模块注册表、跨平台 Provider、模块优先命令和协议 2.0 分片批次输出。

**Architecture:** model 定义平台无关的资产、关系、模块结果和批次协议；provider 通过名称注册强类型能力；module 注册表负责描述符和依赖计划；独立内置模块把现有 Linux 采集器转换为统一记录；agent 只负责编排；CLI 从注册表动态生成命令；report 流式写 JSONL 分片并以 manifest 原子发布。

**Tech Stack:** Go 1.26.5、Go 标准库、Linux procfs/sysfs、fixture 驱动 testing、JSON/JSONL、SHA-256、PowerShell 交叉构建。

## Global Constraints

- 正式二进制名保持 asset-agent。
- 正式模块命令采用 asset-agent <module> scan。
- 旧 asset-agent scan <module> 只保留一个版本，并向 stderr 输出迁移说明。
- CLI 不得包含 host/network/process/port/connection 等模块名称 switch。
- 首批公开模块为 host、network、process、port、connection；all 是注册表虚拟编排模块。
- resource、filesystem、disk、package、container、service、file、component、application 不在本计划实现，帮助信息不得把它们伪装成已实现。
- 新批次协议固定为 asset-agent.batch-manifest 2.0。
- 生产文件交付使用 --output-dir；-o/--output 只允许单模块人工导出；两者互斥。
- 批次根目录和子目录权限为 0700，文件权限为 0600。
- 批次通过同文件系统 .partial-<scan-id> 目录写入，manifest 最后写入，完成后原子重命名。
- 输出流式编码，不在内存中构建完整 JSON 文档。
- 不执行 Shell、不读取进程 environ、不修改服务器状态。
- 代码注释、README、错误说明使用中文；JSON 字段、错误码、命令和配置键使用英文。
- 保留现有采集器的脱敏、超时、panic 隔离和只读边界。
- 所有行为变化必须遵循测试先行：先运行失败测试，再写最小实现。

---

### Task 1: 协议 2.0 平台无关模型

**Files:**
- Create: internal/model/batch.go
- Create: internal/model/batch_test.go
- Modify: internal/model/model.go

**Interfaces:**
- Produces: BatchSchemaName、BatchManifestSchemaName、BatchSchemaVersion、BatchTypeSnapshot、BatchTypeModule。
- Produces: AssetRecord、AssetStates、Evidence、RelationshipRecord、ErrorDetail、Coverage。
- Produces: ModuleResult、Batch、BatchManifest、ModuleManifest、BatchFile。
- Preserves: 旧 Snapshot 和 ModuleReport 类型，仅用于一个版本的人工兼容输出。

- [ ] **Step 1: 写失败测试固定 Schema、状态、空集合和敏感字段边界**

在 internal/model/batch_test.go 创建：

~~~go
func TestBatchProtocolUsesVersionTwoAndNoReservedFutureDomains(t *testing.T) {
    batch := Batch{
        SchemaName: BatchSchemaName,
        SchemaVersion: BatchSchemaVersion,
        ID: "scan-1",
        Type: BatchTypeModule,
        RequestedModule: "host",
        Results: []ModuleResult{},
    }
    encoded, err := json.Marshal(batch)
    if err != nil { t.Fatal(err) }
    text := string(encoded)
    if !strings.Contains(text, "\"schema_name\":\"asset-agent.batch\"") {
        t.Fatalf("batch = %s", text)
    }
    for _, forbidden := range []string{"services", "packages", "containers", "files", "applications"} {
        if strings.Contains(text, "\""+forbidden+"\"") {
            t.Fatalf("batch contains future domain %q: %s", forbidden, text)
        }
    }
}

func TestAssetRecordKeepsEmptyEvidenceAsArray(t *testing.T) {
    record := AssetRecord{
        RecordID: "host:1", RecordType: "host", HostID: "host:1",
        ScopeID: "host:1", ScopeType: "host", Name: "server-1",
        Platform: "linux", Evidence: []Evidence{},
    }
    encoded, err := json.Marshal(record)
    if err != nil { t.Fatal(err) }
    if !bytes.Contains(encoded, []byte(`\"evidence\":[]`)) {
        t.Fatalf("record = %s", encoded)
    }
}
~~~

测试还要断言 StatusComplete 的字面值为 complete，ModuleResult 的 Errors、Records、Relationships 和 Coverage 集合编码为 [] 而不是 null。

- [ ] **Step 2: 运行模型测试并确认因新类型缺失而失败**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/model -run 'TestBatchProtocol|TestAssetRecord' -count=1
~~~

Expected: FAIL，错误包含 undefined: Batch 或 undefined: AssetRecord。

- [ ] **Step 3: 实现最小协议模型**

在 internal/model/model.go 增加：

~~~go
const StatusComplete Status = "complete"
~~~

在 internal/model/batch.go 定义以下字段及 JSON 名称：

~~~text
AssetRecord:
  record_id, record_type, host_id, scope_id, scope_type, name,
  version, vendor, platform, states, first_observed_at,
  last_observed_at, confidence, attributes, evidence

Evidence:
  provider, source_type, locator, locator_hash, observed_at,
  digest, confidence

RelationshipRecord:
  record_id, relationship_type, from_id, to_id, observed_at,
  confidence, evidence

ModuleResult:
  module, schema_version, status, authoritative, started_at,
  finished_at, duration_ms, coverage, errors, records,
  relationships

Batch:
  schema_name, schema_version, id, type, requested_module,
  platform, agent, started_at, finished_at, results

BatchManifest:
  schema_name, schema_version, scan_id, batch_type,
  requested_module, platform, agent, started_at, finished_at,
  modules, files
~~~

ModuleResult 额外包含不序列化的 Published bool，供编排器控制单模块依赖数据是否写出。模块内部依赖对象由 module.Result 包装，不进入 model 和 JSON。自定义 MarshalJSON 或构造函数必须把所有集合规范化为非 nil。

- [ ] **Step 4: 运行模型测试到绿色**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/model -count=1
~~~

Expected: PASS。

- [ ] **Step 5: 提交协议模型**

~~~powershell
git add -- internal/model/model.go internal/model/batch.go internal/model/batch_test.go
git commit -m "feat: add asset batch protocol model"
~~~

### Task 2: 可扩展 Provider 注册表与 Linux 适配器

**Files:**
- Create: internal/provider/provider.go
- Create: internal/provider/provider_test.go
- Create: internal/provider/contracts.go
- Create: internal/provider/linux/providers.go
- Create: internal/provider/linux/providers_test.go

**Interfaces:**
- Produces: provider.Provider、provider.Set、provider.NewSet、Set.Register、Set.Lookup、Set.Platform、provider.As。
- Produces capability constants: system_profile、host、network、process、socket。
- Produces typed contracts: ProfileProvider、HostProvider、NetworkProvider、ProcessProvider、SocketProvider。
- Produces: linux.New(root platform.Root) (*provider.Set, error)。

- [ ] **Step 1: 写失败测试固定动态能力注册和类型查询**

在 internal/provider/provider_test.go 创建 fakeProvider，并断言：

~~~go
func TestSetRegistersCapabilitiesWithoutCentralSwitch(t *testing.T) {
    set, err := NewSet("linux", fakeProvider{name: "custom-capability"})
    if err != nil { t.Fatal(err) }
    got, ok := set.Lookup("custom-capability")
    if !ok || got.Capability() != "custom-capability" {
        t.Fatalf("provider = %#v, ok = %v", got, ok)
    }
    if set.Platform() != "linux" { t.Fatalf("platform = %q", set.Platform()) }
}

func TestSetRejectsDuplicateCapability(t *testing.T) {
    _, err := NewSet("linux",
        fakeProvider{name: "process"},
        fakeProvider{name: "process"},
    )
    if err == nil { t.Fatal("duplicate capability accepted") }
}
~~~

- [ ] **Step 2: 运行 Provider 测试并确认包缺失**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/provider -count=1
~~~

Expected: FAIL，因为 provider 包接口尚未定义。

- [ ] **Step 3: 实现动态 Provider Set 和强类型契约**

internal/provider/provider.go 的公共接口固定为：

~~~go
type Provider interface {
    Capability() string
}

type Lookup interface {
    Platform() string
    Lookup(string) (Provider, bool)
}

func As[T Provider](lookup Lookup, capability string) (T, bool)
~~~

Set 内部使用 map[string]Provider；Register 拒绝空能力名和重复能力。contracts.go 中的采集方法必须保持现有采集器签名，使适配器不重写采集逻辑。

- [ ] **Step 4: 运行 Provider 核心测试到绿色**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/provider -count=1
~~~

Expected: PASS。

- [ ] **Step 5: 写 Linux Provider 注册失败测试**

providers_test.go 使用临时 platform.Root 调用 linux.New，断言 system_profile、host、network、process、socket 五项能力均可 Lookup，且 Platform 为 linux。测试不执行真实扫描。

- [ ] **Step 6: 运行 Linux Provider 测试并确认 New 缺失**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/provider/linux -count=1
~~~

Expected: FAIL，错误包含 undefined: New。

- [ ] **Step 7: 实现 Linux 适配器**

providers.go 创建五个小结构体，分别调用：

~~~text
capability.Detect(ctx, root, runtime.GOARCH)
host.Collect(ctx, root)
network.Collect(ctx, root, network.SystemInterfaceSource{})
process.Collect(ctx, root, bootID)
socket.Collect(ctx, root, processes)
~~~

适配器不得包含业务判断，只负责把 platform.Root 绑定到强类型 Provider。

- [ ] **Step 8: 运行 Provider 全部测试并提交**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/provider/... -count=1
~~~

~~~powershell
git add -- internal/provider
git commit -m "feat: add platform provider registry"
~~~

### Task 3: 动态模块注册表和依赖计划

**Files:**
- Create: internal/module/module.go
- Create: internal/module/registry.go
- Create: internal/module/registry_test.go

**Interfaces:**
- Produces: module.Descriptor、CommandDescriptor、SupportResult、Request、Result、Module。
- Produces: Registry.Register、Registry.Lookup、Registry.List、Registry.Plan。
- Consumes: provider.Lookup 和 model.ModuleResult。

- [ ] **Step 1: 写失败测试证明新增模块自动进入命令和 all 计划**

registry_test.go 定义只返回描述符的 fakeModule，覆盖：

~~~go
func TestRegistryListsNewModuleWithoutKnownNameTable(t *testing.T) {
    registry := NewRegistry()
    if err := registry.Register(fakeModule{name: "custom"}); err != nil { t.Fatal(err) }
    listed := registry.List()
    if len(listed) != 1 || listed[0].Name != "custom" {
        t.Fatalf("listed = %+v", listed)
    }
    plan, err := registry.Plan("all")
    if err != nil { t.Fatal(err) }
    if len(plan) != 1 || plan[0].Descriptor().Name != "custom" {
        t.Fatalf("plan = %+v", plan)
    }
}

func TestRegistryPlansHardDependenciesOnce(t *testing.T) {
    registry := NewRegistry()
    mustRegister(t, registry, fakeModule{name: "host"})
    mustRegister(t, registry, fakeModule{name: "process", hard: []string{"host"}})
    mustRegister(t, registry, fakeModule{name: "port", hard: []string{"process"}})
    plan, err := registry.Plan("port")
    if err != nil { t.Fatal(err) }
    if got := names(plan); !reflect.DeepEqual(got, []string{"host", "process", "port"}) {
        t.Fatalf("plan = %v", got)
    }
}
~~~

另加重复名称、保留名称 all、未知依赖和依赖环测试。

- [ ] **Step 2: 运行测试并确认 Registry 缺失**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/module -count=1
~~~

Expected: FAIL，错误包含 undefined: NewRegistry。

- [ ] **Step 3: 实现接口和描述符**

Descriptor 精确包含：

~~~go
type Descriptor struct {
    Name                 string
    SchemaVersion        string
    RecordTypes          []string
    Commands             []CommandDescriptor
    RequiredCapabilities []string
    OptionalCapabilities []string
    HardDependencies     []string
    SoftDependencies     []string
    DefaultInterval      string
    ResourceClass        string
    Timeout              string
    SupportsDelta        bool
}
~~~

Module 接口与批准规格一致：Descriptor、Probe(context.Context, provider.Lookup)、Collect(context.Context, provider.Lookup, Request)。

Result 和 Request 固定为：

~~~go
type Result struct {
    Data     model.ModuleResult
    Internal any
}

type Request struct {
    Dependencies map[string]Result
}
~~~

- [ ] **Step 4: 实现确定性拓扑计划**

Registry 使用名称 map 保存模块。List 按名称排序。Plan(target) 对单模块计算硬依赖闭包，对 all 选择全部注册模块；结果按依赖优先、同层名称排序。遇到未知模块、未知依赖或环返回错误，不执行部分计划。

- [ ] **Step 5: 运行模块注册表测试到绿色并提交**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/module -count=1
~~~

~~~powershell
git add -- internal/module
git commit -m "feat: add dynamic module registry"
~~~

### Task 4: host、network、process 独立模块

**Files:**
- Create: internal/modules/common.go
- Create: internal/modules/register.go
- Create: internal/modules/register_test.go
- Create: internal/modules/host/module.go
- Create: internal/modules/host/module_test.go
- Create: internal/modules/network/module.go
- Create: internal/modules/network/module_test.go
- Create: internal/modules/process/module.go
- Create: internal/modules/process/module_test.go

**Interfaces:**
- Produces: modules.NewRegistry() (*module.Registry, error)。
- Produces Module implementations named host、network、process。
- host Internal value is model.Host。
- process Internal value is []model.Process。

- [ ] **Step 1: 写 host 模块失败测试**

使用 fake HostProvider 返回固定主机，断言：

~~~go
result := hostModule.Collect(ctx, providers, module.Request{})
if result.Data.Status != model.StatusComplete { ... }
if len(result.Data.Records) != 1 { ... }
record := result.Data.Records[0]
if record.RecordType != "host" || record.HostID == "" || record.States.Running != true { ... }
if _, ok := result.Internal.(model.Host); !ok { ... }
~~~

Probe 在 host capability 不存在时必须返回 unsupported。

- [ ] **Step 2: 运行 host 模块测试到红色**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/modules/host -count=1
~~~

Expected: FAIL，因为模块尚不存在。

- [ ] **Step 3: 实现 host 模块并运行到绿色**

Descriptor 使用 default_interval 24h、resource_class light、timeout 15s。记录稳定 ID 优先使用 Host.ID，其次使用 machine_id、dmi_uuid、hostname 的 SHA-256 确定性摘要。Attributes 保留现有 Host 字段，Evidence provider 为 host。

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/modules/host -count=1
~~~

- [ ] **Step 4: 写 network 模块失败测试**

fake NetworkProvider 返回一个接口、地址和路由。断言生成 network_interface、address、route 三类记录，稳定 ID 在重复运行中相同，default_interval 为 6h。

- [ ] **Step 5: 运行 network 测试到红色、实现并运行到绿色**

Run RED/GREEN:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/modules/network -count=1
~~~

network 把 host 声明为硬依赖，使用 host 依赖 Internal 中的稳定主机 ID 填充全部网络记录的 HostID；单独执行 network 时仍只发布网络记录，不重复发布 host 记录。

- [ ] **Step 6: 写 process 依赖测试**

fake ProcessProvider 必须收到 host 依赖 Internal 中的 BootID。结果只输出 process 记录，依赖 host 记录不复制到 process 结果。Descriptor 的 HardDependencies 为 host，default_interval 为 12h。

- [ ] **Step 7: 运行 process 测试到红色、实现并运行到绿色**

Run RED/GREEN:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/modules/process -count=1
~~~

进程状态映射为 running=true；命令行沿用现有 collector 已脱敏结果；Attributes 不得包含 environ。

- [ ] **Step 8: 写默认注册表测试并实现**

register_test.go 断言当前 NewRegistry 的 List 精确包含 host、network、process；all 不作为真实 Collector 出现在 map 中，但 Registry.Plan("all") 能生成这三个模块的完整计划。port 和 connection 在 Task 5 实现后加入同一个默认注册表。

- [ ] **Step 9: 运行当前模块测试并提交**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/modules/... -count=1
~~~

~~~powershell
git add -- internal/modules
git commit -m "feat: add host network and process modules"
~~~

### Task 5: port 和 connection 模块拆分

**Files:**
- Create: internal/modules/port/module.go
- Create: internal/modules/port/module_test.go
- Create: internal/modules/connection/module.go
- Create: internal/modules/connection/module_test.go
- Modify: internal/modules/register.go
- Modify: internal/modules/register_test.go

**Interfaces:**
- port hard-depends on process and uses SocketProvider once。
- port Internal contains all raw sockets plus exact socket_process relationships。
- connection hard-depends on port and consumes port Internal，不重复调用 SocketProvider。
- Produces record types port and connection。

- [ ] **Step 1: 写 port 过滤和归属失败测试**

fake SocketProvider 返回：

~~~go
[]model.Socket{
    {ID: "listen", Protocol: "tcp", State: "LISTEN", LocalAddress: "0.0.0.0", LocalPort: 443, PIDs: []int{10}},
    {ID: "conn", Protocol: "tcp", State: "ESTABLISHED", LocalAddress: "10.0.0.1", LocalPort: 50000, RemoteAddress: "10.0.0.2", RemotePort: 5432, PIDs: []int{10}},
}
~~~

断言 port 只发布一个监听记录，状态 exposed=true，并生成 process 到 port 的 listens_on 证据关系。Descriptor 周期为 1h。

- [ ] **Step 2: 运行 port 测试到红色、实现并运行到绿色**

Run RED/GREEN:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/modules/port -count=1
~~~

TCP LISTEN 为监听端口；UDP remote_port=0 且远端地址为空、0.0.0.0 或 :: 时视为本地端口。其他 Socket 不发布为 port，但保存在 Internal。

- [ ] **Step 3: 写 connection 复用事实失败测试**

构造 port 依赖 Result.Internal，包含 LISTEN 和 ESTABLISHED。connection 模块不注册 SocketProvider 也应成功，只输出 ESTABLISHED 记录及 connects_to 关系；default_interval 为 1h。

- [ ] **Step 4: 运行 connection 测试到红色、实现并运行到绿色**

Run RED/GREEN:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/modules/connection -count=1
~~~

- [ ] **Step 5: 注册 port、connection 并运行模块全测**

修改 register_test.go 的期望列表为 connection、host、network、port、process，并断言 all 计划中 process 只执行一次，port 先于 connection。

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/modules/... -count=1
~~~

- [ ] **Step 6: 提交 Socket 语义拆分**

~~~powershell
git add -- internal/modules
git commit -m "feat: split port and connection modules"
~~~

### Task 6: 动态扫描编排器

**Files:**
- Create: internal/agent/scanner.go
- Create: internal/agent/scanner_test.go
- Create: internal/agent/invoke.go

**Interfaces:**
- Produces: Scanner、NewScanner、NewScannerWithClock。
- Produces: Scanner.Modules(context.Context) ([]module.Info, error)。
- Produces: Scanner.ScanTarget(context.Context, string) (model.Batch, error)。
- Preserves existing agent.Runtime until CLI migration task。

- [ ] **Step 1: 写动态 all 和单模块依赖失败测试**

scanner_test.go 使用 fake modules：

~~~go
func TestScannerAllUsesRegistryPlan(t *testing.T) {
    registry := module.NewRegistry()
    mustRegister(t, registry, recordingModule("custom"))
    scanner := NewScannerWithClock(registry, providerSet, agentInfo, fixedClock)
    batch, err := scanner.ScanTarget(context.Background(), "all")
    if err != nil { t.Fatal(err) }
    if batch.RequestedModule != "all" || len(batch.Results) != 1 {
        t.Fatalf("batch = %+v", batch)
    }
    if !batch.Results[0].Published { t.Fatal("all result not published") }
}

func TestScannerSingleModuleHidesDependencyRecords(t *testing.T) {
    // host -> process；两者均执行，但只有 process 的 Published 为 true。
}
~~~

另测：开始前 context 取消返回 context.Canceled；模块 panic 变 failed；超时变 timeout；硬依赖 failed 时目标模块不执行；非 Linux 空 Provider 返回 unsupported 模块状态而非伪空 complete；无 ProfileProvider 时 Doctor 返回当前 platform 的通用诊断而不是顶层错误。

- [ ] **Step 2: 运行 Scanner 测试到红色**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/agent -run TestScanner -count=1
~~~

Expected: FAIL，错误包含 undefined: NewScanner。

- [ ] **Step 3: 实现 Doctor 和模块信息探测**

Scanner.Doctor 优先调用 ProfileProvider；不存在时输出当前 platform、root=false 和空能力列表。module.Info 包含 Descriptor 和 SupportResult。Scanner.Modules 对 Registry.List 中每个模块调用 Probe；结果按模块名称排序。Probe panic 转为 unsupported，错误码为 probe_panic。

- [ ] **Step 4: 实现顺序依赖编排**

ScanTarget：

1. 在开始前检查 ctx；
2. 调用 Registry.Plan；
3. 生成 UTC scan ID 和 Batch；
4. 每个模块独立 WithTimeout；
5. 将已完成依赖放入 Request.Dependencies；
6. panic 转为 failed 和 module_panic；
7. context deadline 转为 timeout；
8. 单模块只标记目标 Published；all 标记全部 Published；
9. 依赖的完整 Internal 只保存在本次执行 map；写入 Batch 前清空未 Published 结果的 Records 和 Relationships，保留状态与错误；
10. 失败不清空已经成功的模块；
11. Batch Type 对 all 为 snapshot，其他为 module。

- [ ] **Step 5: 运行 Agent 新旧测试**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/agent -count=1
~~~

Expected: PASS；旧编排测试此时仍保留。

- [ ] **Step 6: 提交动态编排器**

~~~powershell
git add -- internal/agent/scanner.go internal/agent/scanner_test.go internal/agent/invoke.go
git commit -m "feat: orchestrate registered scan modules"
~~~

### Task 7: JSONL 分片和 manifest 原子批次发布

**Files:**
- Create: internal/report/batch.go
- Create: internal/report/batch_test.go
- Modify: internal/report/path.go
- Modify: internal/report/path_test.go

**Interfaces:**
- Produces: WriteBatch(outputRoot string, batch model.Batch) (string, error)。
- Produces internal writeBatchWithOptions for small deterministic shard tests。
- Produces: DefaultOutputRoot(executablePath string) (string, error)。
- Preserves WriteJSON 和 WriteJSONFile for explicit single-module export。

- [ ] **Step 1: 写原子批次失败测试**

batch_test.go 创建一个 host ModuleResult 和一个 relationship，调用 WriteBatch 后断言：

~~~text
root mode 0700
root/inbox mode 0700
formal batch directory exists
manifest.json is 0600
host-00001.jsonl is 0600
relationships-00001.jsonl is 0600
manifest schema is asset-agent.batch-manifest 2.0
manifest record count and SHA-256 match files
no .partial-* remains
~~~

- [ ] **Step 2: 运行批次测试到红色**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/report -run TestWriteBatch -count=1
~~~

Expected: FAIL，错误包含 undefined: WriteBatch。

- [ ] **Step 3: 实现目录和命名**

正式目录命名：

~~~text
snapshot-<UTC>-<scan-id>     for all
module-<module>-<UTC>-<scan-id> for single module
~~~

outputRoot 与 inbox 使用 MkdirAll 后 Chmod 0700。临时目录必须位于 inbox 内且名称精确为 .partial-<scan-id>。

- [ ] **Step 4: 写分片阈值失败测试**

使用 writeBatchWithOptions 的 MaxRecords=2，传入 5 条 host 记录，断言生成 host-00001.jsonl、host-00002.jsonl、host-00003.jsonl，记录数分别为 2、2、1。

- [ ] **Step 5: 实现流式分片、摘要和 manifest**

生产默认：

~~~go
MaxRecords = 100000
MaxBytes   = 64 << 20
MaxLine    = 1 << 20
~~~

每行使用 json.Encoder 且禁止 HTML 转义。写入时同步计算 SHA-256 和字节数。模块 Records 写入 <module>-NNNNN.jsonl；Relationship 写入 relationships-NNNNN.jsonl。manifest 最后写，列出每个执行模块状态以及每个分片。

- [ ] **Step 6: 写编码失败清理测试**

在 AssetRecord.Attributes 放入不可 JSON 编码的 channel，断言 WriteBatch 返回错误、没有正式目录、没有遗留 .partial 目录，也没有删除 outputRoot 中预先存在的其他批次。

- [ ] **Step 7: 实现精确失败清理和原子重命名**

失败 defer 只关闭并删除本次创建的临时目录。正式目录已存在时返回冲突错误，不覆盖。成功路径 Sync 文件、manifest 和目录后 Rename。

- [ ] **Step 8: 实现 DefaultOutputRoot 测试和函数**

DefaultOutputRoot 解析可执行文件绝对路径和符号链接，返回同级 output；目录实际创建交给 WriteBatch。

- [ ] **Step 9: 运行 report 全测并提交**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/report -count=1
~~~

~~~powershell
git add -- internal/report
git commit -m "feat: publish atomic asset batches"
~~~

### Task 8: 模块优先 CLI、兼容命令和批次输出集成

**Files:**
- Modify: internal/agent/runtime.go
- Modify: internal/cli/run.go
- Replace: internal/cli/scan_options.go
- Modify: internal/cli/output.go
- Modify: internal/cli/run_test.go
- Modify: cmd/asset-agent/main.go
- Modify: cmd/asset-agent/runtime_linux.go
- Modify: cmd/asset-agent/runtime_unsupported.go
- Delete: internal/agent/local_runtime.go
- Delete: internal/agent/module.go
- Delete: internal/agent/module_test.go
- Delete: internal/agent/scan.go
- Delete: internal/agent/scan_test.go
- Delete: internal/agent/strategy.go
- Delete: internal/model/module_report.go
- Delete: internal/model/module_report_test.go

**Interfaces:**
- Runtime becomes Doctor、Modules、ScanTarget。
- CLI discovers module names only from Runtime.Modules。
- Linux runtime uses modules.NewRegistry plus linux.New(platform.NewRoot("/"))。
- Non-Linux runtime uses同一模块注册表加空 provider.Set(runtime.GOOS)。

- [ ] **Step 1: 重写 fakeRuntime 并写模块优先失败测试**

run_test.go 新 Runtime fake 记录 target。至少覆盖：

~~~go
func TestRunModuleFirstScanUsesRegisteredModule(t *testing.T) {
    seen := ""
    runtime := fakeRuntime{
        infos: []module.Info{{Descriptor: module.Descriptor{Name: "custom"}}},
        targetSeen: &seen,
        batch: model.Batch{SchemaName: model.BatchSchemaName, SchemaVersion: model.BatchSchemaVersion},
    }
    code := Run(ctx, []string{"custom", "scan", "-o", "-"}, &stdout, &stderr, runtime)
    if code != 0 { t.Fatalf("code=%d stderr=%s", code, stderr.String()) }
    if seen != "custom" { t.Fatalf("target=%q", seen) }
}
~~~

再覆盖 modules、describe、status、schedule、all scan、未知模块、未知动作。

- [ ] **Step 2: 运行 CLI 测试到红色**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/cli -run 'TestRunModuleFirst|TestRunModules|TestRunDescribe' -count=1
~~~

Expected: FAIL，因为当前 CLI 只识别 scan 前缀。

- [ ] **Step 3: 写旧命令兼容失败测试**

覆盖：

~~~text
scan            -> all scan
scan all        -> all scan
scan host       -> host scan
scan network    -> network scan
scan process    -> process scan
scan socket     -> 退出码 2，并提示 socket 已拆分为 port 和 connection，避免静默丢失旧 socket 命令中的任一类数据
~~~

成功兼容的旧命令 stderr 必须包含 deprecated；旧 socket 命令必须包含迁移说明；正式模块优先命令不得产生迁移提示。

- [ ] **Step 4: 写输出参数失败测试**

覆盖：

~~~text
host scan --output-dir <dir> 生成 module-host 批次
all scan --output-dir <dir> 生成 snapshot 批次
host scan -o <file> 写单个 Batch JSON
host scan -o - 写 stdout JSON
all scan -o <file> 返回退出码 2
all scan -o - 返回退出码 2
--output-dir 与 -o 同时出现返回退出码 2
目录参数缺值返回退出码 2
~~~

- [ ] **Step 5: 实现动态命令路由**

Run 启动时调用 Runtime.Modules 构建 name -> Info map。第一个参数若为模块名或 all，则第二个参数必须为 scan、describe、status、schedule。modules 返回全部 Info JSON。帮助从 Info 动态生成，不出现未注册模块。

status 在本阶段输出 Probe 支持状态；schedule 输出 Descriptor.DefaultInterval、ResourceClass 和 Timeout。它们不声称常驻调度已经运行。

- [ ] **Step 6: 实现输出决策**

生产扫描默认使用 DefaultOutputRoot + WriteBatch。--output-dir 覆盖根目录。-o 仅单模块可用并调用 WriteJSON/WriteJSONFile。成功批次 stdout 只输出正式目录绝对路径。

- [ ] **Step 7: 切换 cmd Runtime 并验证非 Linux 状态**

main.go 将 newRuntime 改为返回 (agent.Runtime, error)，初始化失败时向 stderr 输出中文说明并返回退出码 1。Linux：

~~~go
providers, err := linux.New(platform.NewRoot("/"))
registry, err := modules.NewRegistry()
return agent.NewScanner(registry, providers)
~~~

非 Linux 使用 provider.NewSet(runtime.GOOS) 和同一个 modules.NewRegistry，使 modules/describe 正常，扫描模块得到 unsupported。

- [ ] **Step 8: 删除固定模块编排和旧伪空协议**

删除列出的旧 agent 与 ModuleReport 文件。保留旧 Snapshot 类型只供现有 doctor/兼容测试使用，但新扫描路径不返回 Snapshot。确认生产代码不存在 plannedModules map、ModuleHost 常量和模块名 switch。

- [ ] **Step 9: 运行 CLI、Agent 和全量测试**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./internal/cli ./internal/agent ./... -count=1
~~~

Expected: PASS。

- [ ] **Step 10: 提交 CLI 和运行时迁移**

~~~powershell
git add -A -- cmd/asset-agent internal/agent internal/cli internal/model
git commit -m "feat: add module-first scanner commands"
~~~

### Task 9: 中文文档、旧产物隔离和 Linux 验证脚本

**Files:**
- Modify: README.md
- Modify: scripts/verify-linux.sh
- Modify: docs/代码阅读地图.md
- Modify: docs/superpowers/specs/2026-08-12-linux-asset-agent-design.md
- Modify: docs/superpowers/specs/2026-08-12-modular-scan-cli-design.md
- Create: docs/legacy/README.md
- Modify: .gitignore
- Move outside Git worktree after validation: D:\ChatGPT\资产台账系统\docs\all-20260812T093156Z.json -> D:\ChatGPT\资产台账系统\docs\legacy\all-20260812T093156Z.json

**Interfaces:**
- Documents only implemented modules and module-first commands。
- Marks protocol 1.0 sample and old watch/scheduling design as historical。
- Verifies manifest and JSONL output on real Linux。

- [ ] **Step 1: 更新 README**

README 必须列出当前实际模块 host、network、process、port、connection、all；说明其他批准模块属于后续阶段。示例使用：

~~~bash
sudo ./asset-agent host scan --output-dir /var/lib/asset-agent/output
sudo ./asset-agent port scan --output-dir /var/lib/asset-agent/output
sudo ./asset-agent all scan --output-dir /var/lib/asset-agent/output
~~~

说明 manifest 完整性、0700/0600 权限和 legacy 命令迁移。

- [ ] **Step 2: 标记旧设计为历史**

两份 2026-08-12 规格顶部增加中文醒目说明：行为基线已被 2026-08-13 可扩展设计替代；watch、固定周期、单文件 Snapshot 和具体下游设想不再是当前实现目标。

- [ ] **Step 3: 更新验证脚本**

脚本执行模块优先命令，找到最新正式批次目录，使用 jq 校验 manifest.schema_version == "2.0"，逐个检查 manifest 列出的分片存在。脚本不得依赖旧 .sockets 顶层字段。

- [ ] **Step 4: 添加 legacy 说明和忽略规则**

docs/legacy/README.md 说明 JSON 只是真实 Linux 协议 1.0 样例，不代表新模块容量，也不允许伪造升级。gitignore 增加 docs/legacy/*.json，避免真实服务器资产明细被误提交。

- [ ] **Step 5: 运行文档一致性检查**

Run:

~~~powershell
rg -n "asset-agent scan (host|network|process|socket|all)" README.md scripts docs/代码阅读地图.md
rg -n "services.*packages.*containers.*files.*applications" README.md
git diff --check
~~~

Expected: 第一条仅允许出现在明确标记的 legacy 迁移说明；第二条不得把未实现模块描述为当前输出。

- [ ] **Step 6: 安全移动根工作区旧样例**

使用同一 PowerShell 进程验证源和目标均位于 D:\ChatGPT\资产台账系统\docs 后，创建精确目标目录并移动单个文件。若源不存在则记录已完成，不使用递归或通配符。

- [ ] **Step 7: 提交文档迁移**

~~~powershell
git add -- .gitignore README.md scripts/verify-linux.sh docs/代码阅读地图.md docs/legacy/README.md docs/superpowers/specs/2026-08-12-linux-asset-agent-design.md docs/superpowers/specs/2026-08-12-modular-scan-cli-design.md
git commit -m "docs: document extensible scanner core"
~~~

### Task 10: 全量验证和交付二进制

**Files:**
- Verify: all Go sources and tests。
- Build: dist/asset-agent-linux-amd64。
- Replace exact untracked artifact after verification: D:\ChatGPT\资产台账系统\asset-agent-linux-amd64。

**Interfaces:**
- Produces a static Linux amd64 asset-agent with injected version, commit, build time。
- Does not claim real Linux scan verification from Windows。

- [ ] **Step 1: 格式化并检查差异**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' fmt ./...
git diff --check
~~~

Expected: exit 0，无格式或空白错误。

- [ ] **Step 2: 运行静态检查和全量测试**

Run:

~~~powershell
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' vet ./...
& 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe' test ./... -count=1
~~~

Expected: 所有命令退出 0。

- [ ] **Step 3: 验证动态扩展约束**

Run:

~~~powershell
rg -n "case \"(host|network|process|port|connection)\"|plannedModules|ModuleHost|ModuleNetwork|ModuleProcess|ModuleSocket" internal/cli internal/agent
~~~

Expected: 无生产代码命中；测试 fixture 字符串不构成失败。

- [ ] **Step 4: 交叉构建 Linux amd64**

Run:

~~~powershell
$goExe = 'D:\go\go1.26.5.windows-amd64\go\bin\go.exe'
$commit = git rev-parse HEAD
$buildTime = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
& $goExe build -trimpath -ldflags "-s -w -X github.com/Theearthwormsplitsvertically/scan/internal/buildinfo.Version=v0.3.0-dev -X github.com/Theearthwormsplitsvertically/scan/internal/buildinfo.Commit=$commit -X github.com/Theearthwormsplitsvertically/scan/internal/buildinfo.BuildTime=$buildTime" -o dist/asset-agent-linux-amd64 ./cmd/asset-agent
~~~

Expected: exit 0，产物非空。

- [ ] **Step 5: 检查二进制协议字符串和摘要**

Run:

~~~powershell
$binary = Resolve-Path 'dist\asset-agent-linux-amd64'
$bytes = [IO.File]::ReadAllBytes($binary)
$ascii = [Text.Encoding]::ASCII.GetString($bytes)
if (-not $ascii.Contains('asset-agent.batch-manifest')) { throw 'batch protocol missing from binary' }
Get-FileHash -Algorithm SHA256 $binary
~~~

- [ ] **Step 6: 替换根目录过期二进制**

先分别解析源文件和目标父目录绝对路径，确认目标精确为 D:\ChatGPT\资产台账系统\asset-agent-linux-amd64，再使用 Copy-Item -LiteralPath -Force 替换。替换后比较两个 SHA-256 必须一致。

- [ ] **Step 7: 检查最终状态**

Run:

~~~powershell
git status --short --branch
git log --oneline -12
~~~

Expected: 只有被 .gitignore 排除的 dist 产物；所有源代码和文档变更均已提交。

## 本计划与完整规格的边界

本计划完成设计规格阶段 1。resource、filesystem、disk、常驻调度和 systemd 属于阶段 2；package、container、service 属于阶段 3；file、component、application 属于阶段 4；事件增量、ACK 和保留策略属于阶段 5；Windows、macOS Provider 实现和外部插件 IPC 属于阶段 6。阶段 1 已把注册表、Provider、命令、Schema 和输出边界固定，使后续阶段通过新增模块和 Provider 扩展，而不是修改 CLI 固定分支。
