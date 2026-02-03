# Scheduler 核心概念

- [Scheduler 核心概念](#scheduler-核心概念)
  - [概述](#概述)
  - [调度周期](#调度周期)
  - [Cache](#cache)
    - [Cache 职责](#cache-职责)
  - [Snapshot](#snapshot)
    - [Snapshot 的重要性](#snapshot-的重要性)
  - [PodGroup](#podgroup)
  - [Queue](#queue)
  - [Session](#session)
    - [Session 职责](#session-职责)
  - [Actions](#actions)
  - [Plugins](#plugins)
  - [Statements 与事务模型](#statements-与事务模型)
  - [Scenarios](#scenarios)
  - [BindRequest](#bindrequest)
  - [相关文档](#相关文档)

## 概述

KAI Scheduler 围绕一系列核心概念构建，这些概念共同协作以做出调度决策。本文档面向参与调度器开发的开发者，解释这些概念。

调度器以**周期**方式运行。每个周期会获取集群状态的快照，并通过一系列 Actions 做出调度决策。

## 调度周期

调度器按固定周期运行（可通过 `schedulePeriod` 配置）。每个周期遵循以下流程：

```mermaid
flowchart LR
    Start([周期开始]) --> Cache[Cache 同步]
    Cache --> Snapshot[生成 Snapshot]
    Snapshot --> Session[打开 Session]
    Session --> Actions[执行 Actions]
    Actions --> Close[关闭 Session]
    Close --> End([周期结束])
    
    style Start fill:#f5f5f5,stroke:#333
    style End fill:#f5f5f5,stroke:#333
    style Snapshot fill:#d4f1f9,stroke:#333
    style Session fill:#d5f5e3,stroke:#333
    style Actions fill:#fcf3cf,stroke:#333
```

1. **Cache 同步**：确保所有 Kubernetes 资源的 Informer 数据是最新的
2. **Snapshot**：捕获某一时刻的集群状态
3. **Session**：基于 Snapshot 数据创建调度上下文
4. **Actions**：按顺序执行调度 Actions（Allocate → Consolidate → Reclaim → Preempt → StaleGangEviction）
   - 每个 Action 独立处理各个 Job，按 Job 创建并提交或丢弃 Statement
5. **Session 关闭**：清理并准备下一周期

## Cache

**Cache** 是集群状态的权威数据源，由 Kubernetes API 的 Informer 构建而成。

### Cache 职责

- **数据收集**：聚合来自多种 API 资源的信息
- **状态维护**：跟踪资源随时间的变化
- **Snapshot 生成**：创建一致的时间点视图
- **变更传播**：将已提交的调度决策应用回集群

## Snapshots

**Snapshot** 在每个调度周期开始时捕获集群状态。

Snapshot 包含调度决策所需的全部集群资源和状态信息，包括 Pod、Node、Queue、PodGroup、BindRequest 以及其他相关 Kubernetes 对象。

关于 Snapshot 和 Snapshot 插件的更多信息，请参阅 [Snapshot Plugin](../plugins/snapshot.md)。

### Snapshot 的重要性

1. **一致性**：一个周期内的所有调度决策都基于同一份集群状态
2. **性能**：避免在调度过程中重复发起 API 调用
3. **调试**：提供可复现的状态用于分析

## PodGroups

**PodGroup** 定义工作负载的 gang 调度需求，规定多个 Pod 应如何一起被调度。

PodGroup 由 pod-grouper 组件根据工作负载类型自动创建，可指定最小成员数、队列归属和优先级类。

关于 PodGroup 创建和 gang 调度的更多信息，请参阅 [Pod Grouper](pod-grouper.md)。

## Queues

调度器实现了用于资源管理和公平分配的**层级队列系统**。**Queue** 表示带有配额、优先级和限制的逻辑资源容器。

更多信息请参阅 [Scheduling Queues](../queues/README.md) 和 [Fairness](../fairness/README.md)。

## Sessions

**Session** 表示单个调度周期的调度上下文。它包含 Snapshot 数据、插件回调，并为调度操作提供框架。

### Session 职责

- **状态管理**：在周期内维护一致的集群视图
- **插件协调**：提供插件回调的扩展点
- **Statement 工厂**：为 Actions 创建 Statement 对象
- **资源统计**：跟踪资源分配和使用情况

关于 Session 实现、生命周期和插件集成的更多信息，请参阅 [Plugin Framework](plugin-framework.md)。

## Actions

**Actions** 是在每个周期内按顺序执行的离散调度操作。每个 Action 基于 Session 的 Snapshot 数据工作，并使用 Statement 保证原子性。

关于 Action 类型、执行顺序和实现细节，请参阅 [Action Framework](action-framework.md)。

## Plugins

调度器采用基于插件的架构，通过多种扩展点扩展功能。插件在 Session 生命周期中注册回调，以影响调度行为。

关于插件开发、扩展点和示例，请参阅 [Plugin Framework](plugin-framework.md)。

## Statements 与事务模型

**Statement** 为调度操作提供类似事务的机制，允许将变更分组，并以单元形式提交或回滚。Actions 使用 Statement 确保调度决策的原子性。此外，Statement 在内存中模拟调度场景，支持在提交前评估潜在变更。

关于 Statement 操作和使用模式，请参阅 [Action Framework - Statements](action-framework.md#3-statement)。

## Scenarios

**Scenario** 表示用于在提交前评估潜在决策的假设调度状态。它们支持“假设”建模和调度操作的验证。

关于 Scenario 实现和验证机制，请参阅 [Action Framework - Scenarios](action-framework.md#1-scenarios)。

## BindRequests

**BindRequest** 用于调度器与 Binder 组件之间的通信。当调度器决定 Pod 应在何处运行时，会创建一个 BindRequest，包含 Pod、选中的 Node 以及资源分配详情。

Binder 异步处理 BindRequest，执行实际的 Pod 绑定以及所需的资源准备（如卷挂载或动态资源分配）。

关于绑定流程和 BindRequest 生命周期，请参阅 [Binder](binder.md)。

## 相关文档

- [Action Framework](action-framework.md) - Action 实现详解
- [Plugin Framework](plugin-framework.md) - 插件开发指南
- [Binder](binder.md) - Pod 绑定流程
- [Pod Grouper](pod-grouper.md) - Gang 调度实现
- [Snapshot Plugin](../plugins/snapshot.md) - Snapshot 捕获与分析工具
- [Scheduling Queues](../queues/README.md) - Queue 配置与管理
- [Fairness](../fairness/README.md) - 资源公平性与分配算法
