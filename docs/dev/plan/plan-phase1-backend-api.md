# 实施计划 第一阶段：后端 API 开发 (核心逻辑)

本阶段的目标是为控制台提供高性能、只读的调度器内部状态接口。所有数据将直接从调度器的内存 Cache (Snapshot) 中提取。

---

## 1. 任务目标

- [ ] 定义可视化专用的数据模型（VO/DTO）。
- [ ] 实现从 `api.ClusterInfo` 到可视化模型的转换逻辑及服务层。
- [ ] 在调度器的 HTTP Mux 中注册 API 路由。
- [ ] 确保 API 访问不阻塞主调度循环。

---

## 2. 详细拆解步骤

- [x] **步骤 1：定义视图模型 (View Models)**
  - 在 `pkg/scheduler/api` 下创建可视化专用包，定义精简的 JSON 返回结构，避免直接透传复杂的 K8s 对象。
  - **文件位置**：`pkg/scheduler/api/visualizer_info/visualizer_info.go`
  - **核心任务**：
    - [x] 定义 `ClusterSummary`：概览聚合数据。
    - [x] 定义 `QueueView`：递归结构的队列信息。
    - [x] 定义 `JobView`：包含任务状态简图的视图。
    - [x] 定义 `NodeView`：支持 GPU 插槽展示的节点视图。

- [x] **步骤 2：实现数据转换服务 (Visualizer Service)**
  - 创建逻辑层，负责将原始的 `api.ClusterInfo` 数据转换为可视化专用的视图模型。
  - **文件位置**：`pkg/scheduler/visualizer/visualizer_service.go`
  - **核心功能实现**：
    - [x] 实现 `GetClusterSummary()` 统计全局资源。
    - [x] 实现 `GetQueues()` 递归构建队列树逻辑。
    - [x] 实现 `GetJobs(namespace string)` 过滤与转换逻辑。
    - [x] 实现 `GetNodes()` 解析映射 GPU 物理占用。

- [ ] **步骤 3：实现 API 处理程序 (HTTP Handlers)**
  - 编写 HTTP 处理函数，处理路由请求参数并返回标准化 JSON 数据。
  - **文件位置**：`pkg/scheduler/visualizer_handlers.go`
  - **核心端点实现**：
    - [ ] `GET /api/v1/visualizer/summary`
    - [ ] `GET /api/v1/visualizer/queues`
    - [ ] `GET /api/v1/visualizer/jobs` (支持 namespace 过滤)
    - [ ] `GET /api/v1/visualizer/nodes`

- [ ] **步骤 4：生命周期集成 (Wiring)**
  - 在调度器启动和初始化流程中完成服务注入与路由注册。
  - **文件位置**：`pkg/scheduler/scheduler.go`
  - **集成任务**：
    - [ ] 在 `NewScheduler` 中初始化 `VisualizerService`。
    - [ ] 将所有 Visualizer 路由注册到 `s.mux`。
    - [ ] 实现安全的并发缓存访问（利用 Snapshot）。

---

## 3. 技术细节说明 (Internal)

- **性能保障**：API 请求时通过 `s.cache.Snapshot()` 获取一致性视图，不阻塞调度循环。
- **数据精简**：视图模型仅包含前端展示必要的字段，减少传输体积。
- **安全隔离**：纯只读 API，不提供修改集群状态的能力。

---

## 4. 交付物验收标准 (DoD)

- [ ] `pkg/scheduler/api/visualizer` 包及类型定义完成。
- [ ] `VisualizerService` 转换逻辑通过单元测试。
- [ ] 调度器 HTTP 接口返回正确格式的 JSON。
- [ ] 性能达标：大规模数据下响应时间 < 200ms。
