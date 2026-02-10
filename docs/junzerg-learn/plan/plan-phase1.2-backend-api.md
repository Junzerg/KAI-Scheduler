# 实施计划 Phase 1.2: 实现数据转换服务 (Visualizer Service)

本阶段的目标是实现核心逻辑层，负责将 KAI-Scheduler 内部复杂的缓存数据（`ClusterInfo`）转换为 Phase 1.1 中定义的视图模型。

---

## 1. 任务目标

- [ ] 实现 `VisualizerService` 结构体，用于持有对调度器 Cache 的引用。
- [ ] 实现基础转换逻辑：将 `api.ClusterInfo` 转换为视图模型。
- [ ] 实现递归处理逻辑：构建层级队列树。
- [ ] 实现复杂映射逻辑：将任务占用情况映射到节点的物理 GPU Slot 上。

---

## 2. 详细执行步骤

### 2.1 基础实现

3

- [x] 创建文件：`pkg/scheduler/visualizer/visualizer_service.go`
- [x] 定义 `VisualizerService` 接口与实现类。
- [x] 实现 `NewVisualizerService(cache schedcache.Cache)` 构造函数。

### 2.2 实现概览转换 (`GetClusterSummary`)

- [x] 从 `Snapshot` 中统计健康节点与故障节点。
- [x] 汇总 GPU 总量与已分配量（从 `NodeInfo` 中提取）。
- [x] 根据 `PodGroupInfo` 的状态进行计数统计。

### 2.3 实现队列树构建 (`GetQueues`)

- [x] 将 `Queues` Map 转换为具备树状结构的列表。
- [x] 处理层级关系：通过 `Parent` 字段建立树形链接。
- [x] 计算各层级队列的资源水位（Guaranteed vs Allocated）。

### 2.4 实现任务视图转换 (`GetJobs`)

- [x] 实现根据 Namespace 过滤 PodGroup 的逻辑。
- [x] 转换 `PodGroupInfo` 为 `JobView`，提取创建时间、优先级等关键信息。
- [x] 遍历所属 Pods，构建 `TaskView` 并关联其所在的节点名称。

### 2.5 实现节点与 GPU 槽位映射 (`GetNodes`)

- [x] 遍历 `Nodes` Map。
- [x] 核心算法：解析每个节点上的 Pod 绑定信息，确定其占用的 GPU 索引。
- [x] 处理“空闲但无法使用”的逻辑：如果某节点由于特定调度策略（如 Bin-packing/Fragment）导致当前不接受新任务，标记对应的 Slot 为 `Fragmented`。

---

## 3. 设计原则

1.  **无状态性**：Service 内部不维护长期状态，每次调用均实时转换最新的 Snapshot。
2.  **健壮性**：处理 Cache 为空或数据状态不一致（如 Pod 绑定的 Node 已删除）的情况，避免 Panic。
3.  **计算解耦**：将复杂的统计逻辑封装在 Service 中，保持 Handler 层的简洁。

---

## 4. 验收标准

- [x] 所有转换函数均有对应的单元测试。
- [x] 测试覆盖：能正确处理空集群数据。
- [x] 测试覆盖：能正确处理具有三层以上深度的队列树。
- [x] 测试覆盖：GPU Slot 映射准确反映了 Pod 的实际分布。
