# Phase 1.3 后端 API 测试用例设计

本文档覆盖 `Visualizer` 后端 API（Phase 1.3）的测试范围与用例设计，分为：单元测试、HTTP Handler 测试、端到端（e2e）测试三层。

---

## 1. 测试范围与目标

- **服务层（Service）**：`VisualizerService` 的数据转换逻辑（已在 `visualizer_service_test.go` 中有较多覆盖，本节只列关键补充点）。
- **Handler 层**：`VisualizerHandler` 的 HTTP 行为（方法校验、状态码、Header、错误路径）。
- **集成 / e2e 层**：在真实或模拟集群中，通过 HTTP 调用 4 个接口，验证端到端行为与性能。

目标：

- 确认 4 个 API 在正常与异常场景下均返回**正确结构**、**合理的 HTTP 状态码**，且不会 panic。
- 对重要边界条件（空集群、大规模数据、错误快照、复杂 GPU 映射等）有明确的预期行为并固化为测试。

---

## 2. 服务层（VisualizerService）测试用例补充

> 主要在现有 `visualizer_service_test.go` 上补充/确认。

- **用例 S-1：大规模队列层级**
  - 场景：构造包含 3~5 层深度、几十个队列节点的层级树。
  - 预期：
    - `GetQueues()` 返回的根节点数量正确。
    - 任意叶子队列沿父指针回溯，路径与构造一致。
    - 不应出现循环引用或丢失的子节点。

- **用例 S-2：大规模 Job / Pod 数量**
  - 场景：构造上千个 `PodGroupInfo`，每个包含多个 Pod。
  - 预期：
    - `GetJobs("")` 能在合理时间内返回全部 Job。
    - Job 数量、每个 Job 的 Task 数量与构造一致。
    - 不发生内存爆炸或超时（单测层面主要关注不超时/不 OOM）。

- **用例 S-3：多命名空间混合过滤**
  - 场景：构造 3 个命名空间（`ns-a`, `ns-b`, `ns-c`），每个若干 Jobs。
  - 预期：
    - `GetJobs("ns-a")` 只返回 ns-a。
    - 传入不存在的命名空间 `GetJobs("ns-x")` 返回空数组。

- **用例 S-4：节点状态与 GPU 统计多样性**
  - 场景：混合 Ready/NotReady 节点，不同 GPU 数量。
  - 预期：
    - `GetClusterSummary()` 中 `TotalNodes`、`HealthyNodes`、`TotalGPUs`、`AllocatedGPUs` 与构造一致。
    - `GetNodes()` 中每个节点的 `Status` 与节点 Ready 条件匹配。

- **用例 S-5：Snapshot 变化容错**
  - 场景：`MockCache.Snapshot()` 在第一次返回正常数据，第二次开始返回 error。
  - 预期：
    - 调用 `GetClusterSummary()` 多次时，当出错时返回 error，不 panic。
    - `GetQueues` / `GetJobs` / `GetNodes` 同样能正确透传错误。

---

## 3. HTTP Handler 层测试用例设计

> 建议新建 `pkg/scheduler/visualizer/visualizer_handler_test.go`，使用 `httptest` + `MockVisualizerService`。

### 3.1 通用行为

- **用例 H-1：不支持的 HTTP 方法**
  - 请求：对 4 个路由分别发送 `POST` / `PUT` / `DELETE`。
  - 预期：
    - 返回 HTTP 405。
    - 响应体可以是简单错误文本（当前实现），不要求 JSON。

- **用例 H-2：CORS 与 Content-Type 头**
  - 请求：对每个 `GET` 接口发起一次请求，Mock Service 返回简单数据。
  - 预期：
    - `Content-Type: application/json`。
    - `Access-Control-Allow-Origin: *` 存在。

### 3.2 正常路径

- **用例 H-3：/summary 正常返回**
  - Mock：`GetClusterSummary()` 返回包含若干节点与 Job 统计的对象。
  - 预期：
    - HTTP 200。
    - 响应 JSON 可正确反序列化为 `ClusterSummary`。

- **用例 H-4：/queues 正常返回**
  - Mock：`GetQueues()` 返回多层级队列树。
  - 预期：
    - HTTP 200。
    - 根节点数量与队列结构与 Mock 一致。

- **用例 H-5：/jobs namespace 过滤**
  - Mock：`GetJobs(namespace)` 记录入参。
  - 请求：
    - `/api/v1/visualizer/jobs`（不带参数）。
    - `/api/v1/visualizer/jobs?namespace=ns-a`。
  - 预期：
    - 对应调用中 `namespace` 参数分别为 `""` 和 `"ns-a"`。
    - HTTP 200 且 body 为 JSON 数组。

- **用例 H-6：/nodes 正常返回**
  - Mock：`GetNodes()` 返回包含多个节点与 GPU 槽位的列表。
  - 预期：
    - HTTP 200。
    - 响应 JSON 数组长度与 Mock 一致。

### 3.3 异常路径

- **用例 H-7：Service 返回错误（summary）**
  - Mock：`GetClusterSummary()` 返回 `error`。
  - 预期：
    - HTTP 500。
    - 不泄露内部错误细节（仅简单错误文本）。

- **用例 H-8：Service 返回错误（queues/jobs/nodes）**
  - 对 3 个接口分别 Mock 返回 `error`。
  - 预期：
    - HTTP 500。
    - 日志中记录错误（可通过注入测试 logger 或检查 log hook）。

---

## 4. 端到端（e2e）测试用例设计

> 建议在 `docs/dev/test` 或 `hack/` 下补充脚本，例如 `e2e-phase1.3-visualizer.sh`，并结合 kind / docker-desktop 集群执行。

### 4.1 环境准备

- 复用 `plan-phase1.3-backend-api.md` 中的步骤：
  - 从集群导出 scheduler 配置 `config.yaml`。
  - 将集群内 `kai-scheduler-default` Deployment scale 到 0。
  - 使用本地源码运行：
    - `go run ./cmd/scheduler/main.go --scheduler-conf=/tmp/kai-scheduler-config.yaml --plugin-server-port=8081 ...`

### 4.2 功能验证用例

- **用例 E-1：空负载场景**
  - 准备：集群中无 Kueue Workload / 无 KAI 特定 Job。
  - 步骤：
    - 调用 4 个接口。
  - 预期：
    - `/summary`：节点数 > 0，JobCounts 中各项为 0。
    - `/queues`：至少包含 root 队列或空数组，接口不报错。
    - `/jobs`：空数组。
    - `/nodes`：返回若干节点，GPU 槽位可以为 0 或若干。

- **用例 E-2：CPU-only Pod 工作负载**
  - 准备：部署 `docs/quickstart/pods/cpu-only-pod.yaml`。
  - 预期：
    - `/jobs`：出现对应 namespace 下的 Job / Task 记录（如果集成到 KAI PodGroup，需要按实际实现调整）。
    - `/nodes`：节点 GPU 情况与原始集群一致，不会因 CPU-only Pod 出现异常。

- **用例 E-3：GPU 工作负载与 Slot 映射**
  - 准备：提交 1~2 个 GPU 作业，确保在某节点上运行。
  - 预期：
    - `/summary`：`TotalGPUs` 与 `AllocatedGPUs` 随运行作业变化。
    - `/nodes`：至少若干 GPU slot 的 `OccupiedBy` 字段非空，包含对应 Pod 名称。

- **用例 E-4：多命名空间 Job 过滤**
  - 准备：在不同 namespace 部署作业。
  - 步骤：
    - 调用 `/jobs`（不带 namespace）。
    - 分别调用 `/jobs?namespace=ns-a`、`/jobs?namespace=ns-b`。
  - 预期：
    - 不带参数时返回所有作业。
    - 带 `namespace` 参数时只返回对应命名空间作业。

### 4.3 健壮性与性能用例

- **用例 E-5：高负载列表规模**
  - 场景：集群内有大量节点、队列和 Job（可通过脚本批量提交）。
  - 预期：
    - 4 个接口在合理时间内返回（例如 p99 延迟 < 200ms，视环境配置）。
    - 不产生明显的调度抖动或 CPU 飙高（可通过 metrics/Grafana 观察）。

- **用例 E-6：异常退出恢复**
  - 场景：在 scheduler 运行过程中短暂重启（Ctrl+C 再重新 `go run`）。
  - 预期：
    - 重启后接口能够恢复服务，返回的数据与集群当前状态一致。

---

## 5. 回归与维护建议

- 新增或修改 `VisualizerService` 字段/行为时：
  - 必须同步更新对应的 Service 单测 + Handler 单测 + e2e 脚本预期。
- 为防止接口回归，建议：
  - 将部分 e2e 检查脚本集成到 CI 中（在 kind 集群上跑最小集测试）。
  - 对 4 个 API 的响应结构（JSON Schema）进行简单快照测试（例如使用 `jq` 提取关键字段进行比对）。

