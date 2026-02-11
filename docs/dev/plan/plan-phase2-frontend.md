# Phase 2: 前端实施计划 (Frontend Implementation Plan) - Angular

**目标**: 构建一个现代化的、响应式的 Web 控制台，使用 Phase 1 开发的后端 API 来可视化 KAI-Scheduler 的内部调度状态，并作为微应用无缝集成到 Kubeflow Dashboard 中。

**技术栈建议 (Strict Compliance)**:

- **框架**: Angular 16+ (Kubeflow 的主要技术栈)
- **UI 组件库**: Angular Material (官方组件库，保证风格统一)
- **数据获取**: RxJS (Angular 核心库，用于流式数据处理和轮询)
- **可视化图表**:
  - ECharts 或 ngx-charts (用于资源图表)
  - D3.js (用于队列树/GPU拓扑等复杂定制视图)
- **依赖管理**: npm

---

## 1. 项目初始化与基础设施

### 1.1 项目脚手架搭建 (Scaffold)

- [x] 在 `web/` 目录下初始化 Angular 项目结构。
- [x] 配置 Angular Material 主题 (Indigo/Pink 或根据 Kubeflow 自定义主题)。
- [x] 配置 `proxy.conf.json` 开发服务器代理，将 API 请求转发到本地 Scheduler API (例如 `http://127.0.0.1:8081`)。

### 1.2 核心布局与导航

- [x] 实现响应式的 **侧边栏/导航栏 (Sidenav)** (Dashboard, Queues, Jobs, Nodes)。
- [x] 创建 **顶部栏 (Toolbar)**，包含全局命名空间选择器 (Namespace Selector)。
- [x] 实现 **全局状态服务** (RxJS BehaviorSubject)，用于管理当前选中的 Namespace (`NamespaceService`) 和全局刷新控制 (`RefreshService`)。
- [ ] (可选) 用户偏好设置持久化 (如主题、刷新间隔等)。

---

## 2. 仪表盘概览 (Dashboard / Summary Page)

**目标**: 提供集群状态的“健康检查”视图，一目了然。

### 2.1 核心指标卡片 (Metrics Cards)

- [x] 使用 `mat-card` 展示聚合计数器：节点总数、健康/异常节点数、GPU 总数、已分配 GPU 数。
- [ ] (可选) 如果未来有历史数据，可添加迷你趋势图。

### 2.2 作业状态分布

- [x] 环形图 (Donut Chart): 按状态展示作业分布 (Pending, Running, Failed, Completed)。
- [x] 点击图表扇区可快速跳转到 Jobs 页面并过滤对应状态。

---

## 3. 队列可视化 (Queues Page)

**目标**: 可视化层级化的资源配额 (Quota) 和公平共享 (Fair-share) 分布。

### 3.1 树形视图 / 矩形树图 (Treemap)

- [x] 实现交互式的 **树形组件** (`mat-tree`) 来展示嵌套的队列结构。
- [x] 可视化标识 (`QueueResourceBarComponent`)：
  - **保证配额 (Guaranteed)** (最小资源保障)。
  - **上限 (Max Limit)** (突发容量)。
  - **当前使用量 (Usage)** (实时的资源条，颜色随使用率变化)。

### 3.2 队列详情面板

- [ ] 点击队列节点时，弹出侧边栏 (`mat-sidenav`) 或模态框 (`mat-dialog`)，展示详细的 CPU/Memory/GPU 使用统计。
- [x] 展示队列权重 (Weight)，已在树节点上直接显示。
- [ ] (增强) 展示详细的"公平共享得分"分析面板。

---

## 4. 作业管理 (Jobs Page)

**目标**: 查看和检查调度单元 (PodGroups)。

### 4.1 高级数据表格

- [x] 使用 `mat-table` 实现支持排序 (`MatSort`)、过滤、分页 (`MatPaginator`) 的作业列表。
- [x] 列定义：作业名称、命名空间、所属队列、状态 (带颜色 Chip)、提交时间、任务数。
- [x] **命名空间联动**: 列表内容响应全局 Namespace 选择器的变化 (`combineLatest` + `switchMap`)。

### 4.2 作业详情视图

- [x] 点击行展开详情 (Expandable Row, `multiTemplateDataRows` + `@detailExpand` 动画)。
- [x] 列出 **Task/Pod** 细目：每个任务的名称、状态、以及被分配到了哪个节点。
- [ ] 展示每个 Task 的资源请求详情 (CPU/Memory/GPU requests)。

---

## 5. 节点与 GPU 拓扑 (Nodes Page)

**目标**: “杀手级功能” —— 可视化的资源碎片分析。

### 5.1 节点网格 (Node Grid)

- [x] 以网格形式 (`mat-grid-list` 或 Flexbox) 展示所有节点卡片，根据健康状态标记颜色。
- [x] 每个卡片上展示 CPU/内存的简要使用率条 (`mat-progress-bar`)。

### 5.2 GPU 插槽可视化 (Visual Mapping)

- [x] 对于 GPU 节点，渲染 **GPU 插槽 (Slots)** 的物理布局 (例如 DGX 节点的 8 卡布局)。
- [x] **占用状态**: 清晰标识哪个插槽正在被哪个 Job/Pod 占用 (`mat-tooltip` 显示详情)。
- [x] **碎片识别**: 高亮显示那些因为拓扑/亲和性限制而闲置但无法被利用的“碎片”插槽。

---

## 6. 集成与打磨

### 6.1 自动刷新 (Auto-Refresh)

- [x] 实现轮询机制 (RxJS `timer` + `switchMap`)，保持数据实时性，无需手动刷新页面。
- [x] 添加“暂停/恢复”实时更新的控制按钮。

### 6.2 错误处理

- [x] 针对 API 错误 (例如 Scheduler 不可达) 设计友好的 UI 状态 (`mat-card` Empty State / Error Banner)。
- [x] 添加加载指示器 (`mat-spinner`, `mat-progress-bar`) 优化首屏加载体验。

### 6.3 构建与嵌入 (可选/高级)

- [ ] 研究：如何使用 Go `embed` 特性将前端构建产物 (Static Assets) 打包进 Scheduler 二进制文件中，实现单文件分发？

---

## 7. 分阶段执行路线图 (Roadmap)

- **Phase 2.1**: 项目工程搭建 + 仪表盘 (Dashboard) + 基础导航框架 (Angular) (DONE)
- **Phase 2.2**: 作业列表 (Jobs) & 命名空间过滤 (DONE)。
- **Phase 2.3**: 节点网格 (Nodes) & GPU 插槽可视化 (DONE)。
- **Phase 2.4**: 队列层级可视化 (Queue Hierarchy) (DONE)。
- **Phase 2.5**: 细节打磨、自动刷新机制、Error Banner、Donut Chart (DONE)。
- **Phase 2.6**: 作业资源详情 (Task Resource Details) (Ready)
- **Phase 2.7**: 队列详情面板 (Queue Detail Panel) (Ready)
- **Phase 2.8**: Go Embed 生产打包 (Single Binary Distribution) (Ready)
