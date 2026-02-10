# 实施计划 Phase 1.1: 定义后端视图模型 (View Models)

本阶段专注于定义前后端通信的契约（Data Transfer Objects）。模型必须足够轻量，且包含前端展示所需的所有核心维度。

---

## 1. 任务目标

- [x] 创建 `pkg/scheduler/api/visualizer` 包。
- [x] 定义 `ClusterSummary` 模型：用于展示集群整体健康度和资源水位。
- [x] 定义 `QueueView` 模型：支持层级树状展示，包含配额与消耗。
- [x] 定义 `JobView` 与 `TaskView` 模型：展示任务生命周期与资源绑定。
- [x] 定义 `NodeView` 与 `GPUSlot` 模型：解决 GPU 物理占用与碎片的展示。

---

## 2. 详细执行步骤

### 2.1 基础结构准备

- [x] 创建目录：`pkg/scheduler/api/visualizer/`
- [x] 创建文件：`pkg/scheduler/api/visualizer/types.go`

### 2.2 定义概览模型 (ClusterSummary)

- [x] 包含字段：
  - `NodesCount`: 总节点数、健康节点数。
  - `GPUsCount`: 总 GPU 数、已分配 GPU 数。
  - `JobsCount`: 各状态（Pending/Running/Failed）的任务总数。
  - `QueueCount`: 总队列数。

### 2.3 定义队列树模型 (QueueView)

- [x] 包含字段：
  - `Name`: 队列名称（带路径）。
  - `Parent`: 父队列名称。
  - `Weight`: 调度权重。
  - `Resources`: 包含 `Guaranteed`, `Allocated`, `Max` 的三维资源视图。
  - `Children`: 递归包含子队列的 `QueueView` 列表。

### 2.4 定义作业视图模型 (JobView)

- [x] 包含字段：
  - `Name/UID`: 任务唯一标识。
  - `Namespace`: 所属命名空间。
  - `Queue`: 所属队列。
  - `Status`: 任务状态（Running, Pending, Pipelined, Failed）。
  - `CreateTime`: 创建时间戳。
  - `Tasks`: 包含 `TaskView` 列表（具体到 Pod 的状态和绑定的节点）。

### 2.5 定义节点与 GPU 视图模型 (NodeView)

- [x] 包含字段：
  - `Name`: 节点名称。
  - `Status`: 节点状态（Ready/NotReady）。
  - `Resources`: 节点总资源。
  - `GPUSlots`: 列表，每个 Slot 包含：
    - `ID`: 物理索引 (0-7)。
    - `OccupiedBy`: 占用的 Task ID / Pod Name（空则表示可用）。
    - `Fragmented`: 是否因为资源冲突导致虽然空闲但无法使用的标记。

---

## 3. 设计原则

1.  **JSON 互操作性**：所有结构体字段必须带有 `json:"camelCase"` 标签。
2.  **避免冗余**：不直接包含 K8s 原生的 `ObjectMeta` 或 `Spec`，只提取 UI 关心的属性。
3.  **时间格式化**：时间戳统一使用 `RFC3339` 或 Unix 时间戳，方便前端处理。

---

## 4. 验收标准

- [x] `types.go` 文件能成功编译。
- [x] 所有的结构体字段均有对应的 JSON 标签。
- [x] 能够通过简单的序列化测试（即构造一个 Dummy 对象并打印 JSON）。
