# Phase 2.5 Frontend Implementation Summary — 细节打磨与体验优化

## Status: Completed ✅

成功实现了 **全局自动刷新控制**、**错误处理 (Error Banner + Skeleton Loading)** 和 **Dashboard 环形图 (Donut Chart)** 三大体验优化模块，并附带一项后端修复。

---

## 交付物清单

### 前端 — 新增文件

| 文件 | 说明 |
|:---|:---|
| `web/src/app/refresh.service.ts` | 全局自动刷新服务，基于 `BehaviorSubject` + `combineLatest` 实现统一的 `tick$` Observable，支持暂停/恢复 |
| `web/src/app/error.service.ts` | 错误状态管理服务，带 15s 自动清除计时器 |
| `web/src/app/error.interceptor.ts` | `HttpInterceptor`，捕获所有 API 的 HTTP 错误并广播到 `ErrorService` |
| `web/src/app/error-banner/error-banner.component.ts/html/scss` | 错误横幅 UI 组件，固定在主内容区顶部，红色警告条 + slide-in 动画 + dismiss 按钮 |
| `web/src/app/dashboard/job-donut-chart/job-donut-chart.component.ts/html/scss` | 纯 SVG 环形图组件（零外部依赖），展示作业按状态分布 |
| `docs/dev/examples/demo-workloads.yaml` | 测试用 demo workloads (Running / Pending / Completed 三种状态) |

### 前端 — 修改文件

| 文件 | 修改内容 |
|:---|:---|
| `web/src/app/app.module.ts` | 注册 `ErrorBannerComponent`、`JobDonutChartComponent` + `HTTP_INTERCEPTORS` provider |
| `web/src/app/app.component.ts` | 注入 `RefreshService` |
| `web/src/app/app.component.html` | Toolbar 添加暂停/恢复按钮 (⏸/▶)；`<mat-sidenav-content>` 内嵌 `<app-error-banner>` |
| `web/src/app/dashboard/dashboard.component.ts` | 单次 fetch → `RefreshService.tick$` 轮询；改用命令式数据绑定以支持 skeleton |
| `web/src/app/dashboard/dashboard.component.html` | 新增 skeleton 骨架屏 + Queues 计数卡片 + Job Distribution 环形图卡片 |
| `web/src/app/dashboard/dashboard.component.scss` | 新增 skeleton shimmer 动画 CSS |
| `web/src/app/jobs/jobs.component.ts` | 接入 `RefreshService.tick$` + `combineLatest(tick$, namespace$)`；支持 `?status=` query param 过滤（由 Donut Chart 点击触发） |
| `web/src/app/nodes/nodes.component.ts` | 移除自建 `BehaviorSubject` + `timer`，改用全局 `RefreshService.tick$` |
| `web/src/app/nodes/nodes.component.html` | 移除组件内 Pause/Resume 按钮（改由全局控制） |
| `web/src/app/queues/queues.component.ts` | 移除 `timer(0, 5000)`，改用 `RefreshService.tick$`；添加 `OnDestroy` 生命周期清理 |

### 后端修复

| 文件 | 修改内容 |
|:---|:---|
| `pkg/scheduler/visualizer/visualizer_service.go` | `getJobStatus()`: 新增 `Completed` 状态判定——检查 `PodStatusIndex[pod_status.Succeeded]` 区分正常完成与失败 |

---

## 技术决策记录

### 1. RefreshService 设计：全局 tick$ vs 各组件独立 timer

**最终选择：全局 `RefreshService` 单例**

- **之前**: Dashboard 无轮询、Jobs 仅响应 namespace 变化、Nodes 自建 `BehaviorSubject` + `timer`、Queues 硬编码 `timer(0, 5000)` — 四种不同方式，用户无法统一控制。
- **之后**: 所有组件统一订阅 `RefreshService.tick$`，Toolbar 一个按钮控制全局暂停/恢复。
- **核心实现**: `combineLatest([paused$, intervalMs$]).pipe(switchMap(...))` — 暂停时发出 `NEVER`，恢复时立即发出新 timer。

### 2. Donut Chart: 纯 SVG vs ECharts/D3

**最终选择：纯 SVG + Angular 模板（零依赖）**

- **理由**: 仅需 4–5 个扇区的简单环形图，引入 ECharts (~800KB) 或 D3 过于笨重。
- **实现**: ~130 行 TypeScript 完成 SVG arc path 计算 (M → A → L → A → Z)，Angular 模板直接绑定。
- **交互**: hover 高亮 + tooltip + click 跳转 Jobs 页（通过 `Router.navigate` + queryParams）。

### 3. Error Handling: Interceptor + Service 模式

**设计**: `ApiErrorInterceptor` (HttpInterceptor) 负责捕获、`ErrorService` 负责状态管理、`ErrorBannerComponent` 负责展示。

- **关注点分离**: 组件不需要关心错误处理，拦截器全局兜底。
- **自动恢复**: Error Banner 15s 自动消失；当 API 恢复后，新的成功请求不会触发 error，banner 自然清除。
- **slide-in 动画**: 使用 Angular `@trigger` 实现平滑进出。

---

## 后端修复详情

### `getJobStatus()` Completed 状态判定

**问题**: 原逻辑只有 Running / Pending / Failed 三种 fallback，Pod 正常退出 (`Succeeded`) 后既不是 active 也不是 pending，被一律归为 `Failed`。

**方案**: 在 fallback 前新增 `PodStatusIndex[pod_status.Succeeded]` 检查——如果有 Succeeded 状态的 Pod，返回 `"Completed"`；否则才返回 `"Failed"`。

**影响**: Dashboard 环形图与 Jobs 列表现在能正确区分完成和失败的作业。

---

## 功能验收结果

| 验收项 | 结果 |
|:---|:---|
| Toolbar ⏸ / ▶ 按钮切换全局轮询，所有页面同步 | ✅ |
| API 不可达时顶部出现红色 Error Banner | ✅ |
| Error Banner 可手动 dismiss | ✅ |
| API 恢复后 Banner 自动消失 | ✅ |
| Dashboard 首屏 skeleton 骨架屏 | ✅ |
| Donut Chart 按状态着色 (Running/Pending/Completed/Failed) | ✅ |
| Donut Chart 中心显示作业总数 | ✅ |
| Donut Chart hover tooltip | ✅ |
| Donut Chart 点击扇区跳转 Jobs 页并自动过滤 | ✅ |
| 后端正确区分 Completed vs Failed | ✅ |

---

## 颜色映射

| 状态 | 颜色 | HEX |
|:---|:---|:---|
| Running | 🟢 绿色 | `#4caf50` |
| Pending | 🟠 橙色 | `#ff9800` |
| Failed | 🔴 红色 | `#f44336` |
| Completed | 🔵 蓝色 | `#2196f3` |
| Unknown/Other | ⚪ 灰色 | `#9e9e9e` |

---

## 文件变更汇总

```
# 新增 (前端)
web/src/app/refresh.service.ts
web/src/app/error.service.ts
web/src/app/error.interceptor.ts
web/src/app/error-banner/error-banner.component.ts
web/src/app/error-banner/error-banner.component.html
web/src/app/error-banner/error-banner.component.scss
web/src/app/dashboard/job-donut-chart/job-donut-chart.component.ts
web/src/app/dashboard/job-donut-chart/job-donut-chart.component.html
web/src/app/dashboard/job-donut-chart/job-donut-chart.component.scss

# 新增 (文档/测试)
docs/dev/plan/plan-phase2.5-frontend.md
docs/dev/summary/summary-phase2.5-frontend.md         (本文档)
docs/dev/examples/demo-workloads.yaml

# 修改 (前端)
web/src/app/app.module.ts
web/src/app/app.component.ts
web/src/app/app.component.html
web/src/app/dashboard/dashboard.component.ts
web/src/app/dashboard/dashboard.component.html
web/src/app/dashboard/dashboard.component.scss
web/src/app/jobs/jobs.component.ts
web/src/app/nodes/nodes.component.ts
web/src/app/nodes/nodes.component.html
web/src/app/queues/queues.component.ts

# 修改 (后端)
pkg/scheduler/visualizer/visualizer_service.go
```

---

## 后续规划

Phase 2 全部子阶段 (2.1–2.5) 已完成。潜在的下一步：

1. **Phase 2.6 (可选)**: 队列详情侧边栏 — 点击队列行弹出详情面板
2. **Phase 2.7 (可选)**: Go `embed` 打包 — 将前端静态资源嵌入 Scheduler 二进制
3. **Phase 3**: 集成测试 + 端到端测试覆盖
