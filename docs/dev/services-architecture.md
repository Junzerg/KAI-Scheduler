# KAI Scheduler 服务架构与协作关系

## 一、服务总览

```
                    ┌─────────────────────────────────────────────────────────┐
                    │                    Operator（编排层）                      │
                    │  管理 Config / SchedulingShard CRD，部署并协调所有组件        │
                    └─────────────────────────────────────────────────────────┘
                                              │
         ┌────────────────────────────────────┼────────────────────────────────────┐
         │                                    │                                    │
         ▼                                    ▼                                    ▼
┌─────────────────┐              ┌─────────────────────┐              ┌─────────────────────┐
│   Pod Grouper   │              │      Scheduler      │              │       Binder         │
│  （前置：分组）   │   ────────►  │   （核心：决策）     │   ────────►  │   （后置：执行绑定）   │
└─────────────────┘              └─────────────────────┘              └─────────────────────┘
         │                                    │                                    │
         │                                    │                                    │
         ▼                                    ▼                                    ▼
   PodGroup CRD                         BindRequest CRD                         Pod → Node
```

## 二、核心调度链路

### 2.1 数据流

```
用户提交 Pod（schedulerName: kai-scheduler, queue 标签）
        │
        ▼
┌───────────────┐    创建/更新 PodGroup  ┌────────────────────┐
│  Pod Grouper  │ ─────────────────────► │ PodGroupController │
└───────────────┘                        └────────────────────┘
        │
        │ Pod 进入 pending
        ▼
┌───────────────┐    读取 Queue 配额      ┌──────────────────┐
│   Scheduler   │ ◄───────────────────── │  QueueController  │
└───────────────┘                        └──────────────────┘
        │
        │ 创建 BindRequest
        ▼
┌───────────────┐    执行绑定            ┌──────────────────┐
│    Binder     │ ─────────────────────► │  Pod 运行在 Node  │
└───────────────┘                        └──────────────────┘
```

### 2.2 各阶段职责

| 阶段 | 组件 | 输入 | 输出 |
|------|------|------|------|
| 分组 | Pod Grouper | Pod（owner 等） | PodGroup |
| 状态管理 | PodGroupController | PodGroup | 更新 Phase、Conditions |
| 配额管理 | QueueController | Queue | 配额、状态 |
| 调度决策 | Scheduler | Snapshot（Pod、Node、Queue、PodGroup） | BindRequest |
| 绑定执行 | Binder | BindRequest | Pod 绑定到 Node |

## 三、各服务详解

### 3.1 Scheduler（核心）

- **职责**：周期性做调度决策，决定每个 pending Pod 落在哪个 Node
- **流程**：Cache 同步 → Snapshot → Session → Actions（Allocate → Consolidate → Reclaim → Preempt → StaleGangEviction）→ 创建 BindRequest
- **依赖**：Queue（配额）、PodGroup（gang）、Node、Pod、BindRequest 状态

### 3.2 Binder（执行层）

- **职责**：监听 BindRequest，执行 Pod 绑定（含 DRA、PV 等）
- **与 Scheduler 解耦**：Scheduler 只负责决策，Binder 负责耗时/易失败的绑定
- **依赖**：BindRequest、Pod、Node

### 3.3 Pod Grouper（前置）

- **职责**：根据 Pod 的 owner（Job、Deployment、PyTorchJob 等）创建/更新 PodGroup
- **目的**：为 Scheduler 提供 gang scheduling 所需的 MinMember 等信息
- **依赖**：Pod、各类 workload CRD

### 3.4 PodGroupController

- **职责**：维护 PodGroup 的 Phase、Conditions，反映 gang 是否满足
- **依赖**：PodGroup、Pod

### 3.5 QueueController

- **职责**：管理 Queue 的配额、使用量、状态
- **依赖**：Queue、Pod（用于统计使用量）

### 3.6 Admission

- **职责**：校验/改写 Pod（scheduler、queue 标签等）
- **依赖**：Pod 创建/更新请求

### 3.7 ResourceReservation

- **职责**：在 Binder 绑定前，为 GPU 等资源做预留（创建临时 Pod）
- **依赖**：Binder 调用

### 3.8 NodeScaleAdjuster

- **职责**：与 Karpenter 等扩缩容组件集成
- **依赖**：节点池、Node

### 3.9 Operator

- **职责**：根据 Config、SchedulingShard 部署和协调上述所有组件
- **依赖**：Config、SchedulingShard CRD

## 四、如何配合完成项目目的

### 4.1 Gang Scheduling

1. **Pod Grouper**：识别 Job 等 workload，创建 PodGroup，设置 MinMember
2. **Scheduler**：在 Allocate 中检查 PodGroup，只有满足 MinMember 时才分配
3. **StaleGangEviction**：驱逐不再满足 MinMember 的 Job，避免死锁

### 4.2 队列与公平

1. **QueueController**：维护 Queue 配额和使用量
2. **Scheduler**：在 Allocate 中按 Queue 配额和 DRF 分配；Reclaim 回收借用；Preempt 按优先级抢占

### 4.3 决策与执行分离

1. **Scheduler**：快速产生 BindRequest，不等待绑定完成
2. **Binder**：异步处理 BindRequest，失败可重试，不阻塞 Scheduler

### 4.4 可扩展部署

1. **Operator**：通过 Config 和 SchedulingShard 管理多 shard、多配置
2. **Admission**：统一校验和改写 Pod，保证调度策略可执行
