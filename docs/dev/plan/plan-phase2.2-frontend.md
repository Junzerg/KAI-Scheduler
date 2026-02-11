# 实施计划 Phase 2.2: 作业列表与命名空间过滤 (Jobs & Namespace Filtering)

**状态**: COMPLETED ✅

**目标**: 构建功能完善的作业管理页面，支持通过命名空间筛选查看作业，提供排序、分页、状态过滤功能，并能查看作业的详细任务 (Task) 分配情况。

---

## 1. 阶段目标

- [x] **全局状态管理**: 实现 `NamespaceService`，在 Toolbar 中添加命名空间下拉框，实现全局联动。
- [x] **高级数据表格**: 升级现有的 Jobs Table，支持 `MatPaginator` (分页) 和 `MatSort` (排序)。
- [x] **多维过滤**: 实现按“作业状态” (Pending/Running/Completed) 和“作业名称”搜索。
- [x] **详情视图**: 实现作业行的展开视图 (Expandable Row) 或详情模态框，展示 Tasks/Pods 的节点分配信息。
- [x] **UI 优化**: 为不同的作业状态添加视觉标识 (Chips/Badges)。

---

## 2. 详细实施步骤

### 2.1 步骤 1: 全局命名空间服务 (Namespace Service)

1.  **创建服务**:
    ```bash
    ng generate service services/namespace
    ```
    - 使用 `BehaviorSubject<string>` 管理当前选中的 Namespace (默认 'All' 或 'default')。
    - 提供 `selectedNamespace$` Observable 供组件订阅。

2.  **更新 Toolbar 组件**:
    - 在 `AppComponent` (或独立的 Toolbar 组件) 中添加 `mat-select` 下拉框。
    - 下拉框选项应包含 "All Namespaces" 及后端返回的实际 Namespace 列表（初期可硬编码或从 Job 列表提取）。
    - 选中项变化时，调用 `NamespaceService.setNamespace()`。

3.  **API 联动**:
    - 确保 `VisualizerService.getJobs(namespace)` 方法能正确处理 namespace 参数（已存在，需验证）。

### 2.2 步骤 2: 增强作业列表 (Advanced Jobs Table)

1.  **引入 Material 模块**:
    - 在 `SharedModule` 中引入 `MatPaginatorModule`, `MatSortModule`, `MatInputModule`, `MatFormFieldModule`, `MatChipsModule`。

2.  **改造 JobsComponent**:
    - 将数据源从普通数组改为 `MatTableDataSource<JobView>`。
    - 添加 `<table mat-table ... matSort>` 和 `<mat-paginator>`。
    - 实现 `ngAfterViewInit` 绑定 Sort 和 Paginator。

3.  **响应全局状态**:
    - 在 `ngOnInit` 中订阅 `NamespaceService.selectedNamespace$`。
    - 使用 `switchMap` 操作符，当 Namespace 变化时自动重新拉取 Job 列表。
    - 添加 Loading 状态指示器。

### 2.3 步骤 3: 状态渲染与过滤 (Status & Rendering)

1.  **状态徽章 (Status Chips)**:
    - 针对不同状态 (Pending, Running, Succeeded, Failed) 定义 CSS 类。
    - 在表格 "Status" 列使用 `<mat-chip>` 或自定义 Badge 展示。
        - Running: Green/Accent
        - Pending: Yellow/Warn
        - Failed: Red
        - Completed: Gray

2.  **客户端过滤**:
    - 实现搜索框 (`<input matInput>`)，绑定到 `dataSource.filter`。
    - 自定义 `dataSource.filterPredicate`，使其能同时支持“名称搜索”和“状态筛选”（如果需要）。

### 2.4 步骤 4: 作业详情与任务视图 (Job Details)

1.  **展开行设计 (Expandable Rows)**:
    - 改造 `mat-table` 为支持展开行结构 (`multiTemplateDataRows`)。
    - 点击作业行时，展开一个详情区域。

2.  **任务列表 (Tasks List)**:
    - 在展开区域内展示该 Job 下的 `tasks` 列表。
    - 展示字段：Task Name, Status, **Assigned Node** (最关键信息), Resource Requests。
    - 如果 Task 处于 Pending，尝试展示原因 (虽然目前后端 API 可能未返回详细 Events，先预留位置)。

---

## 3. 验收标准

1.  **联动性**: 切换 Toolbar 上的 Namespace，Job 列表应立即刷新显示对应数据。
2.  **交互性**: Job 列表可以点击表头排序，可以分页浏览，且能通过搜索框过滤作业名。
3.  **可视化**: 作业状态一目了然 (颜色区分)，且能看到作业内部的具体 Task 分配到了哪个节点。
4.  **健壮性**: 当没有作业或 API 失败时，显示友好的 Empty/Error 状态。
