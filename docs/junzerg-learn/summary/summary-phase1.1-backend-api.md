# Phase 1.1: 后端视图模型定义完成总结

## 完成内容

本阶段完成了 KAI-Scheduler 可视化控制台后端 API 的基础数据模型定义。这些模型将作为后端调度器与前端控制台之间的通信契约。

### 1. 包结构初始化

- 创建了新的包 `pkg/scheduler/api/visualizer`。
- 创建了模型定义文件 `pkg/scheduler/api/visualizer/types.go`。

### 2. 模型定义详情

#### 2.1 概览模型 (ClusterSummary)

定义了 `ClusterSummary` 结构体，用于展示集群的整体健康状况：

- **节点统计**: `TotalNodes`, `HealthyNodes`
- **GPU 统计**: `TotalGPUs`, `AllocatedGPUs`
- **队列与作业统计**: `TotalQueues`, `JobCounts` (按状态聚合)

#### 2.2 资源模型 (ResourceStats)

定义了通用的资源统计结构 `ResourceStats`，包含：

- `MilliCPU` (mCore)
- `Memory` (Bytes)
- `GPU` (Count)
- `ScalarResources` (其他扩展资源)

#### 2.3 队列模型 (QueueView)

定义了树状结构的队列视图 `QueueView`，包含：

- 基础信息：`Name`, `Parent`, `Weight`
- 资源配额：`Guaranteed`, `Allocated`, `Max` (使用 `QueueResources` 封装)
- 层级关系：`Children` (递归列表)

#### 2.4 作业模型 (JobView & TaskView)

定义了作业及其下属任务的视图：

- **JobView**: 包含 `UID`, `Name`, `Namespace`, `Queue`, `Status`, `CreateTime`。
- **TaskView**: 包含任务粒度的 `Name`, `Status`, `NodeName`，用于追踪 Pod 调度情况。

#### 2.5 节点与 GPU 模型 (NodeView & GPUSlot)

定义了节点详情及 GPU 物理插槽视图：

- **NodeView**: 包含节点状态及资源总量。
- **GPUSlot**: 详细描述 GPU 插槽状态，包括 `ID` (物理索引), `OccupiedBy` (占用者), `Fragmented` (碎片化标记)。

## 下一步计划 (Phase 1.2)

进入 **Phase 1.2: 实现数据转换服务 (Visualizer Service)**，主要任务包括：

1. 创建 `VisualizerService` 结构。
2. 实现从 `api.ClusterInfo` 到上述视图模型的转换逻辑。
3. 实现并发安全的 Snapshot 访问。
