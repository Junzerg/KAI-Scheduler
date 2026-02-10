# 实施计划 Phase 1.3: 实现 API 处理程序 (HTTP Handlers)

本阶段的目标是基于 `VisualizerService` 构建 HTTP 接口层，封装路由逻辑、参数解析和标准 JSON 输出。

---

## 1. 任务目标

- [ ] 实现统一的 HTTP 处理器层。
- [ ] 实现标准化的错误处理与 JSON 响应封装。
- [ ] 完成四个核心端点的逻辑实现。

---

## 2. 详细执行步骤

### 2.1 基础设施准备

- [ ] 创建文件：`pkg/scheduler/visualizer_handlers.go`
- [ ] 定义 `VisualizerHandler` 结构体，注入 `VisualizerService`。
- [ ] 实现 `writeJSON(w, data)` 辅助方法，统一设置 `Content-Type: application/json` 和 CORS 头（若需要）。

### 2.2 实现 Summary 端点

- [ ] 端点：`GET /api/v1/visualizer/summary`
- [ ] 逻辑：直接调用 Service 的 `GetClusterSummary` 并返回。

### 2.3 实现 Queues 端点

- [ ] 端点：`GET /api/v1/visualizer/queues`
- [ ] 逻辑：调用 Service 的 `GetQueues`，返回完整的层级树。

### 2.4 实现 Jobs 端点

- [ ] 端点：`GET /api/v1/visualizer/jobs`
- [ ] 参数解析：支持从 URL Query 获取 `namespace`。
- [ ] 逻辑：调用 Service 的 `GetJobs(namespace)`，支持空 Namespace 时返回所有。

### 2.5 实现 Nodes 端点

- [ ] 端点：`GET /api/v1/visualizer/nodes`
- [ ] 逻辑：调用 Service 的 `GetNodes`，返回包含详细 GPU Slot 信息的节点列表。

---

## 3. 设计原则

1.  **Restful 风格**：遵循标准的 HTTP 动词和状态码。
2.  **错误隔离**：如果 Cache Snapshot 失败，返回 500 并附带友好的错误信息。
3.  **响应流控**：确保大数据量下（如成千上万个 Job）响应不会导致内存溢出。

---

## 4. 验收标准

- [ ] 通过本地 `curl` 测试，四个端点均能正常响应 JSON。
- [ ] `Content-Type` 必须为 `application/json`。
- [ ] 在 Namespace 不存在时正确返回空列表而非报错。
