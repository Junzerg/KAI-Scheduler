# Phase 2.4: 队列层级可视化实施计划 (Queue Hierarchy Visualization)

## 1. 目标 (Objectives)
实现 KAI-Scheduler 队列层级的深度可视化，展示各级队列的父子关系、资源保障量 (Guaranteed)、使用量 (Usage) 以及资源上限 (Max)。

## 2. 核心需求分析 (Core Requirements)
- **层级展示**: 支持嵌套队列的展开/折叠。
- **资源配额可视化**: 展示 CPU、内存、GPU 的分配情况。
- **Usage vs Quota**: 直观表现当前负载与保障额度、上限额度的关系。
- **队列属性**: 展示权重 (Weight/Priority) 等元数据。

## 3. 技术方案对比与决策 (Technical Analysis)

### 3.1 方案对比

| 特性 | Angular Material `mat-tree` (表格化) | D3.js (Treemap / Sunburst) |
| :--- | :--- | :--- |
| **清晰度** | **极高**。类似文件管理器，属性对齐，易于阅读。 | **中**。深层嵌套时标签容易重叠。 |
| **交互性** | 支持搜索、折叠、排序、点击弹出详情。 | 支持缩放 (Zooming) 和悬停。 |
| **资源展示** | 可以在每一行插入精确的进度条 (Progress Bar)。 | 只能通过区块大小或颜色深浅表达单一指标。 |
| **实施难度** | 较低。原生 Angular 整合。 | 较高。需要处理复杂的坐标计算。 |
| **适用场景** | **配额管理、日常运维、精准状态检查**。 | **资源分布概览、性能瓶颈分析**。 |

### 3.2 最终建议
**推荐使用：层次化树形表格 (Hierarchical Tree-Table)** (基于 `mat-tree`)。
- **理由**: 对于调度系统而言，管理员更关心“哪个队列超额了”、“配额还剩多少”等精准数值。树形表格模式可以完美展现 `Usage / Guaranteed / Max` 的三元关系，且能通过列对齐方便水平对比。
- **视觉优化**: 使用带色彩的渐变进度条来“Wow”用户（例如：Usage 超过 Guaranteed 时进度条从蓝变橙，达到 Max 时变红）。

---

## 4. 实施阶段 (Implementation Phases)

### Phase 4.1: 前端服务与数据模型
- [ ] 在 `web/src/app/visualizer.service.ts` 中添加 `QueueView` 和 `QueueResources` 接口定义。
- [ ] 实现 `getQueues()` 方法调用后端 `/api/v1/visualizer/queues` 接口。

### Phase 4.2: 队列主页面与树形列表
- [ ] 创建 `QueuesComponent` 及其路由。
- [ ] 实现基于 `mat-tree` 的基本层级渲染。
- [ ] **视觉核心**: 开发一个 `QueueResourceBarComponent`，用于在一个进度条中同时展示：
  - 底色条：Max Limit (100%)
  - 背景段：Guaranteed Quota
  - 前景条：Current Usage
  - 标记线：Guaranteed 刻度线

### Phase 4.3: 属性展示与交互
- [ ] 在列表中展示权重 (Weight) 和作业数。
- [ ] 实现队列详情侧边栏 (`mat-sidenav`)，点击队列展示详细的资源利用率图表（CPU/MEM/GPU 分别展示）。

### Phase 4.4: 自动刷新与打磨
- [ ] 将队列数据集成到全局轮询机制（默认 5s 刷新）。
- [ ] 添加加载状态 (Skeleton Screen) 和空状态处理。

## 5. 预期 UI 效果说明
- **Tree-Table 结构**:
  | Name | Status | Weight | CPU Usage (Bar) | GPU Usage (Bar) |
  | :--- | :--- | :--- | :--- | :--- |
  | ▾ root | - | 1 | [=======---] | [====-------] |
  |   ▸ dev | - | 10 | [===-------] | [==---------] |
  |   ▾ prod | - | 50 | [==========] | [=========--] |
  |     - cluster-a | - | 25 | [====------] | [====-------] |

- **进度条逻辑**:
  - 绿色: `Usage < Guaranteed`
  - 琥珀色: `Guaranteed <= Usage < Max`
  - 红色: `Usage >= Max` (超额预警)

## 6. 后端验证 (Verification)
- 后端 `pkg/scheduler/visualizer/visualizer_service.go:GetQueues()` 已初步实现，需在实施过程中停掉 Mock 数据进行联调，确保 `qi.ResourceUsage` 的实时性。
