# KAI-Scheduler 可视化控制台实施计划 (Project Visualization Plan)

本计划旨在 KAI-Scheduler 内部实现一个可视化的管理控制台，并将其无感集成到 Kubeflow Dashboard 中，以便于运维人员和开发者实时监控调度器内部状态、队列分布以及 GPU 资源消耗情况。

---

## 1. 项目目标

- **内部状态透明化**：可视化展示调度器 Cache 中的 `ClusterInfo`，包含作业、队列、节点及资源碎片。
- **Kubeflow 集成**：采用与 Kubeflow Notebooks 一致的 UI 风格，支持多租户（Namespace）筛选。
- **交互式监控**：提供 GPU 插槽级别的资源占用视图，帮助追踪调度决策过程。

---

## 2. 核心模块规划

### 2.1 后端 API (Go)

后端将基于 KAI-Scheduler 现有的快照机制提供数据支持，不直接请求 K8s API，保证低延迟。

- **技术选型**：在 `pkg/scheduler/scheduler.go` 的 `mux (http.ServeMux)` 中注册 API 路由。
- **核心接口定义**：
  - `GET /api/v1/summary`：集群整体概览（节点总数、GPU 总量、当前待处理 Job 数）。
  - `GET /api/v1/namespaces/{ns}/jobs`：特定 Namespace 下的任务列表（状态、优先级、资源请求、所属队列）。
  - `GET /api/v1/queues`：层级队列树（展示 Quota, Usage, Fairness 得分）。
  - `GET /api/v1/nodes`：节点详细视图（重点展示 GPU 插槽占用和资源碎片）。

### 2.2 前端控制台 (Angular)

前端将设计为一个独立的微前端应用，通过 Iframe 嵌入 Kubeflow。

- **技术栈**：Angular + Angular Material。由于 Kubeflow 的主要组件（如 Notebooks, Pipelines）均采用 Angular，选择 Angular 可以最大化复用社区组件、样式规范及开发习惯。
- **UI 布局要求**：
  - **风格对标**：采用 Kubeflow 的设计语言，确保与主界面无缝集成。
  - **命名空间感知**：通过 `dashboard-lib` 监听 Kubeflow 的 Namespace 切换事件。

* **核心组件**：
  - **Job Table**：仿 Notebooks 风格的任务状态表，支持搜索和快捷操作。
  - **GPU Heatmap**：可视化的 GPU 插槽视图，直观展示“碎片化”程度。
  - **Queue Tree**：树形图表展示层级队列的资源分配。

---

## 3. 实施流程

### 第一阶段：后端 API 开发 (核心逻辑)

1.  在 `pkg/scheduler/api` 目录下定义可视化的数据结构（DTOs）。
2.  在 `pkg/scheduler` 中实现简单的 API 处理函数，从 `cache.Snapshot()` 中提取并转换数据。
3.  通过现有的 `http.ServeMux` 暴露接口。

### 第二阶段：前端原型与集成 (UI 实现)

1.  初始化 React + Vite 项目。
2.  实现基础布局（Header, Toolbar, Table）。
3.  添加 `dashboard-lib` 集成代码，处理 Namespace 选择逻辑。

### 第三阶段：高级功能与美化 (增强)

1.  开发 GPU 插槽可视化组件。
2.  实现自动刷新/轮询机制。
3.  使用 Go `embed` 特性将前端静态资源打包进调度器二进制文件。

---

## 4. 设计原则

- **非侵入性**：不修改现有的调度算法核心逻辑，仅作为“观察者”读取 Cache 数据。
- **高性能**：API 访问必须极快，避免阻塞调度循环。
- **一致性**：UI 体验必须让用户感觉这就是 Kubeflow 原生功能的一部分。
