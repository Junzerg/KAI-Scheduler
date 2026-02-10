# Phase 1.3: API Server 集成计划

**目标**: 将 `VisualizerService` 集成到 KAI-Scheduler 中，并通过 HTTP 接口暴露服务。

## 1. 组件架构

集成工作涉及添加一个 Handler 层，用于连接现有的 `VisualizerService` 和调度器的 HTTP 服务器。

- **Handler 层**: `pkg/scheduler/visualizer/visualizer_handler.go`
  - 负责 HTTP 请求解析和 JSON 响应格式化。
  - 与核心逻辑（位于 `VisualizerService`）解耦。
- **注册**: `pkg/scheduler/scheduler.go`
  - 调度器拥有 `http.ServeMux`。
  - `VisualizerService` 依赖于 `SchedulerCache`。
  - **初始化流程**:
    1.  初始化 `SchedulerCache`。
    2.  初始化 `VisualizerService` (注入 `SchedulerCache`)。
    3.  初始化 `VisualizerHandler` (注入 `VisualizerService`)。
    4.  注册路由到 `ServeMux`。
    5.  初始化 `Scheduler`。

## 2. 实施任务列表 (TODO)

### 步骤 1: 创建 API Handler

**文件**: `pkg/scheduler/visualizer/visualizer_handler.go`

- [x] 创建文件并定义包名 `package visualizer`
- [x] 定义 `VisualizerHandler` 结构体 (包含 `VisualizerService` 接口)
- [x] 实现构造函数 `NewVisualizerHandler`
- [x] 实现辅助方法 `writeJSON(w, data)`:
  - [x] 设置 `Content-Type: application/json`
  - [x] 设置 CORS 头 (可选，方便开发)
  - [x] 处理 JSON 编码错误
- [x] 实现具体的 Handler 方法:
  - [x] `handleClusterSummary`: 调用 `GetClusterSummary`
  - [x] `handleQueues`: 调用 `GetQueues`
  - [x] `handleJobs`: 解析 `namespace` 参数并调用 `GetJobs`
  - [x] `handleNodes`: 调用 `GetNodes`
- [x] 实现路由注册方法 `RegisterRoutes(mux *http.ServeMux)`:
  - [x] 注册 `/api/v1/visualizer/summary`
  - [x] 注册 `/api/v1/visualizer/queues`
  - [x] 注册 `/api/v1/visualizer/jobs`
  - [x] 注册 `/api/v1/visualizer/nodes`

### 步骤 2: 集成到 Scheduler

**文件**: `pkg/scheduler/scheduler.go`

- [x] 修改 `NewScheduler` 函数逻辑:
  - [x] 将 `schedcache.New(...)` 调用提取为单独的 `cache` 变量
  - [x] 在 `Scheduler` 结构体初始化之前添加 Visualizer 初始化逻辑
- [x] 实现依赖注入与注册:
  - [x] 检查 `mux` 是否为 nil
  - [x] 实例化 `VisualizerService` (传入 `cache`)
  - [x] 实例化 `VisualizerHandler` (传入 service)
  - [x] 调用 `handler.RegisterRoutes(mux)`
- [x] 更新 `Scheduler` 结构体初始化，使用提取出的 `cache` 变量

### 步骤 3: 验证与测试

- [x] **编译检查**:
  - [x] 确保 `pkg/scheduler/visualizer` 正确导入了 `pkg/scheduler/api/visualizer_info`
  - [x] 确保没有循环依赖（`scheduler` -> `visualizer` -> `scheduler` 是不允许的，应该是 `scheduler` -> `visualizer` -> `api/visualizer_info`）
- [x] **运行时验证**:
  - [x] 使用本地源码通过 `go run ./cmd/scheduler/main.go` 启动 Scheduler，连接当前 kind/docker-desktop 集群，确保进程无 Panic：

    ```bash
    go run ./cmd/scheduler/main.go \
      --scheduler-conf=/tmp/kai-scheduler-config.yaml \
      --scheduler-name=kai-scheduler \
      --namespace=kai-scheduler \
      --listen-address=":8080" \
      --plugin-server-port=8081 \
      --leader-elect=false
    ```

  - [x] 日志中仅存在可忽略的 CRD 缺失告警（例如 `topologies.kueue.x-k8s.io`），主循环正常运行
  - [x] 如需避免与集群内 scheduler 选主冲突，可在调试期间暂时将 `kai-scheduler-default` Deployment scale 到 0 副本
- [x] **功能测试 (curl)**:
  - [x] `GET /api/v1/visualizer/summary` -> 返回 JSON 格式的集群概况：

    ```bash
    curl -sS http://127.0.0.1:8081/api/v1/visualizer/summary | jq .
    ```

  - [x] `GET /api/v1/visualizer/queues` -> 返回队列层级树：

    ```bash
    curl -sS http://127.0.0.1:8081/api/v1/visualizer/queues | jq .
    ```

  - [x] `GET /api/v1/visualizer/jobs` -> 返回所有作业：

    ```bash
    curl -sS http://127.0.0.1:8081/api/v1/visualizer/jobs | jq .
    ```

  - [x] `GET /api/v1/visualizer/jobs?namespace=default` -> 返回指定命名空间的作业：

    ```bash
    curl -sS "http://127.0.0.1:8081/api/v1/visualizer/jobs?namespace=default" | jq .
    ```

  - [x] `GET /api/v1/visualizer/nodes` -> 返回节点及 GPU 详情：

    ```bash
    curl -sS http://127.0.0.1:8081/api/v1/visualizer/nodes | jq .
    ```

## 3. 本地调试与 kind 集群联动

集成 Visualizer API 后，推荐在本地使用 `go run` 直接连接现有 kind/docker-desktop 集群进行调试，而不是每次都重打镜像 / 重新部署。

### 3.1 从集群复制 scheduler 配置到本地

集群中 `kai-scheduler-default` Shard 的配置存放在同名 ConfigMap 中：

```bash
kubectl -n kai-scheduler get configmap kai-scheduler-default \
  -o jsonpath='{.data.config\.yaml}' \
  > /tmp/kai-scheduler-config.yaml
```

该文件即为生产中正在使用的 scheduler 配置，之后本地调试直接复用。

### 3.2 可选：避免与集群内 scheduler 抢调度

如果希望只让本地进程调度，调试期间可以暂时将集群内 Deployment 缩到 0：

```bash
kubectl -n kai-scheduler scale deployment kai-scheduler-default --replicas=0
```

调试结束后恢复：

```bash
kubectl -n kai-scheduler scale deployment kai-scheduler-default --replicas=1
```

### 3.3 本地启动 scheduler（使用最新代码）

在项目根目录执行：

```bash
cd /home/junzerg/projects/KAI-Scheduler

go run ./cmd/scheduler/main.go \
  --scheduler-conf=/tmp/kai-scheduler-config.yaml \
  --scheduler-name=kai-scheduler \
  --namespace=kai-scheduler \
  --listen-address=":8080" \
  --plugin-server-port=8081 \
  --leader-elect=false
```

说明：

- 使用 `clientconfig.GetConfigOrDie()`，会自动复用当前 `kubectl` 的 context（例如 `docker-desktop`），直接连到同一个集群。
- Visualizer HTTP API 绑定在 `--plugin-server-port` 指定的端口（上例为 `8081`），metrics 在 `--listen-address`（上例为 `:8080`）。

### 3.4 通过 curl 验证 Visualizer API

在另一个终端窗口中，保持 `go run` 进程运行不退出：

```bash
curl -sS http://127.0.0.1:8081/api/v1/visualizer/summary | jq .
curl -sS http://127.0.0.1:8081/api/v1/visualizer/queues  | jq .
curl -sS http://127.0.0.1:8081/api/v1/visualizer/jobs    | jq .
curl -sS "http://127.0.0.1:8081/api/v1/visualizer/jobs?namespace=default" | jq .
curl -sS http://127.0.0.1:8081/api/v1/visualizer/nodes   | jq .
```

当集群中存在诸如 `cpu-only-pod` 的工作负载时，`jobs` 接口应返回对应的 PodGroup / Task 信息，`nodes` 接口应列出 kind 节点及资源情况。

> 注意：日志中如出现 `failed to list *v1alpha1.Topology`（`topologies.kueue.x-k8s.io`）之类的错误，仅表示当前集群未安装对应 CRD，不影响 Visualizer 基本功能，可在 Phase 1.3 暂时忽略。

## 4. API 规范参考

| 方法 | 端点                         | 描述                             | 查询参数           |
| ---- | ---------------------------- | -------------------------------- | ------------------ |
| GET  | `/api/v1/visualizer/summary` | 获取集群健康状况和统计信息       | -                  |
| GET  | `/api/v1/visualizer/queues`  | 获取队列层级结构和资源使用情况   | -                  |
| GET  | `/api/v1/visualizer/jobs`    | 获取作业和任务列表               | `namespace` (可选) |
| GET  | `/api/v1/visualizer/nodes`   | 获取节点列表及 GPU 拓扑/槽位信息 | -                  |

## 5. 风险与注意事项

1.  **导入路径**: 务必注意使用 `pkg/scheduler/api/visualizer_info` 作为数据模型的导入路径，避免与旧的 `types.go` 或其他包混淆。
2.  **并发安全**: `VisualizerService` 使用了 `SchedulerCache.Snapshot()`，这是线程安全的操作（基于 K8s 调度器通用模式），确保可视化请求不会破坏核心调度逻辑的数据一致性。
3.  **性能影响**: API 直接通过 Snapshot 获取数据。虽然 Snapshot 相对高效，但在极高并发下可能会增加调度器内存压力。Phase 1 阶段假设为低频访问（控制台刷新）。
