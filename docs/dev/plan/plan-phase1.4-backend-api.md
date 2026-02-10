# 实施计划 Phase 1.4: 生命周期集成与接线 (Wiring)

本阶段的目标是将 Visualizer 模型、服务和处理器集成到 KAI-Scheduler 的启动流程中，确保控制台功能跟随调度器一同启动。

---

## 1. 任务目标

- [ ] 在 `Scheduler` 结构体中集成可视化组件。
- [ ] 在系统初始化阶段完成“接线”逻辑。
- [ ] 注册路由到系统主 `http.ServeMux`。

---

## 2. 详细执行步骤

### 2.1 扩展 Scheduler 结构体

- [ ] 修改 `pkg/scheduler/scheduler.go` 中的 `Scheduler` 结构：
  - 添加 `visualizerService` 字段。
  - 添加 `visualizerHandler` 字段。

### 2.2 实现服务初始化

- [ ] 修改 `NewScheduler` 函数：
  - 在创建缓存 `s.cache` 之后，初始化 `s.visualizerService`。
  - 初始化 `s.visualizerHandler` 并注入 Service。

### 2.3 路由注册 (The Hook)

- [ ] 在 `NewScheduler` 末尾或专门的 `registerHandlers` 函数中，将路由映射到注入的 `mux`：
  - `mux.HandleFunc("/api/v1/visualizer/summary", s.visualizerHandler.GetSummary)`
  - `mux.HandleFunc("/api/v1/visualizer/queues", s.visualizerHandler.GetQueues)`
  - `mux.HandleFunc("/api/v1/visualizer/jobs", s.visualizerHandler.GetJobs)`
  - `mux.HandleFunc("/api/v1/visualizer/nodes", s.visualizerHandler.GetNodes)`

### 2.4 确认并发安全

- [ ] 走读代码，确保 Handler 调用的 Service 方法内部确实使用了 `s.cache.Snapshot()` 副本，而非直接访问不稳定的正在变动的缓存对象。

---

## 3. 设计原则

1.  **低耦合**：尽量不改变 `NewScheduler` 的原有逻辑流，仅在合适的位置插入初始化代码。
2.  **单体化**：保持调度器“一个二进制文件、一个服务入口”的特性，方便在 Kubeflow 环境下直接转发。

---

## 4. 验收标准

- [ ] 调度器在生产模式下成功启动。
- [ ] 访问 KAI-Scheduler 的默认 HTTP 端口（由命令行参数指定），Visualizer API 路径生效。
- [ ] 即使可视化部分出现运行期错误，不应导致主调度逻辑挂掉。
