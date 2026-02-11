# Phase 2.5: 细节打磨与体验优化 — 实施计划

**日期**: 2026-02-11
**前置依赖**: Phase 2.1–2.4 全部完成

---

## 概述

Phase 2.5 聚焦三个体验优化方向：

1. **全局自动刷新控制** — 统一各页面分散的轮询逻辑
2. **错误处理** — API 不可达时的友好提示 + 首屏骨架加载
3. **Dashboard 环形图** — 按状态展示作业分布的 Donut Chart

---

## 1. 全局自动刷新控制 (RefreshService)

### 1.1 现状分析

| 组件 | 轮询方式 | 暂停/恢复 |
|:---|:---|:---|
| `DashboardComponent` | 无轮询，单次 fetch | ❌ |
| `JobsComponent` | 仅响应 namespace 变化 | ❌ |
| `NodesComponent` | 自建 `BehaviorSubject` + `timer(0, 5000)` | ✅ 组件内 toggle |
| `QueuesComponent` | `timer(0, 5000)` 硬编码 | ❌ |

**问题**: 每个组件独立管理轮询，用户无法全局暂停（例如排查问题时不想数据刷走）。

### 1.2 设计方案

创建 `RefreshService`（单例，`providedIn: 'root'`）：

```typescript
// refresh.service.ts
@Injectable({ providedIn: 'root' })
export class RefreshService {
  private paused$ = new BehaviorSubject<boolean>(false);
  private interval$ = new BehaviorSubject<number>(5000);

  /** 各组件订阅此 Observable，在 paused 时停止发出值 */
  get tick$(): Observable<number> {
    return combineLatest([this.paused$, this.interval$]).pipe(
      switchMap(([paused, ms]) => paused ? NEVER : timer(0, ms))
    );
  }

  get isPaused$(): Observable<boolean> { return this.paused$.asObservable(); }

  togglePause(): void { this.paused$.next(!this.paused$.value); }
  setPaused(v: boolean): void { this.paused$.next(v); }
}
```

### 1.3 各组件改造

- **DashboardComponent**: `this.refreshService.tick$.pipe(switchMap(() => api.getSummary()))`
- **JobsComponent**: `combineLatest([tick$, namespace$]).pipe(switchMap(...))`
- **NodesComponent**: 移除自建 `refreshState$`，改用 `RefreshService.tick$`
- **QueuesComponent**: 移除 `timer(0,5000)`，改用 `RefreshService.tick$`

### 1.4 UI 控件

在 `app.component.html` 的 Toolbar 右侧（namespace selector 之前）添加暂停/恢复按钮：

```html
<button mat-icon-button (click)="refreshService.togglePause()"
        [matTooltip]="(refreshService.isPaused$ | async) ? 'Resume auto-refresh' : 'Pause auto-refresh'">
  <mat-icon>{{ (refreshService.isPaused$ | async) ? 'play_arrow' : 'pause' }}</mat-icon>
</button>
```

### 1.5 文件变更

| 操作 | 文件 |
|:---|:---|
| 新增 | `web/src/app/refresh.service.ts` |
| 修改 | `web/src/app/app.component.ts` — 注入 `RefreshService` |
| 修改 | `web/src/app/app.component.html` — 添加暂停按钮 |
| 修改 | `web/src/app/dashboard/dashboard.component.ts` — 接入 tick$ |
| 修改 | `web/src/app/jobs/jobs.component.ts` — 接入 tick$ |
| 修改 | `web/src/app/nodes/nodes.component.ts` — 替换自建逻辑 |
| 修改 | `web/src/app/queues/queues.component.ts` — 替换 timer |

---

## 2. 错误处理 (Error Banner + Skeleton)

### 2.1 HTTP Interceptor

创建 `ApiErrorInterceptor`（`HttpInterceptor`），捕获所有 API 响应的 HTTP 错误，通过 `ErrorService` 广播：

```typescript
// error.interceptor.ts
@Injectable()
export class ApiErrorInterceptor implements HttpInterceptor {
  intercept(req, next) {
    return next.handle(req).pipe(
      catchError(err => {
        this.errorService.setError(err.status === 0
          ? 'API server is unreachable. Check if KAI Scheduler is running.'
          : `API Error: ${err.status} ${err.statusText}`);
        return throwError(() => err);
      })
    );
  }
}
```

### 2.2 ErrorService + ErrorBannerComponent

- `ErrorService`: 管理当前错误消息，带 5s 自动清除（API 恢复时立即清除）。
- `ErrorBannerComponent`: 固定在主内容区顶部，用 `mat-toolbar` + warn 色条展示错误消息，支持手动 dismiss。

```html
<!-- error-banner.component.html -->
<div class="error-banner" *ngIf="errorService.error$ | async as errorMsg" @slideIn>
  <mat-icon>error_outline</mat-icon>
  <span>{{ errorMsg }}</span>
  <button mat-icon-button (click)="errorService.clearError()">
    <mat-icon>close</mat-icon>
  </button>
</div>
```

### 2.3 Skeleton Loading

在 Dashboard 使用 CSS skeleton 占位符（纯 CSS 动画，替代当前的 `mat-spinner`），为首屏提供更好的加载体验。

### 2.4 文件变更

| 操作 | 文件 |
|:---|:---|
| 新增 | `web/src/app/error.service.ts` |
| 新增 | `web/src/app/error.interceptor.ts` |
| 新增 | `web/src/app/error-banner/error-banner.component.ts/html/scss` |
| 修改 | `web/src/app/app.module.ts` — 注册 ErrorBanner + HTTP_INTERCEPTORS |
| 修改 | `web/src/app/app.component.html` — 嵌入 `<app-error-banner>` |
| 修改 | `web/src/app/dashboard/dashboard.component.html` — 替换 spinner 为 skeleton |
| 修改 | `web/src/app/dashboard/dashboard.component.scss` — skeleton CSS |

---

## 3. Dashboard 环形图 (Donut Chart)

### 3.1 技术选型

**方案: 纯 SVG + Angular 模板 (零依赖)**

- 理由: 仅需展示 4–5 个扇区的 Donut Chart，引入 ECharts/D3 过于笨重。
- ~100 行 TypeScript 即可完成 SVG 路径计算 (conic-section / stroke-dasharray)。
- Angular 14 template 足以处理动态 SVG 绑定。

### 3.2 组件设计

```
dashboard/
  job-donut-chart/
    job-donut-chart.component.ts    -- 接收 jobCounts，计算 arc 路径
    job-donut-chart.component.html  -- SVG 模板
    job-donut-chart.component.scss  -- 颜色与 hover 效果
```

**功能**:
- 输入: `jobCounts: { [status: string]: number }` (来自 `ClusterSummary`)
- 显示: Donut Chart 按状态着色 (Running=绿, Pending=橙, Failed=红, 其他=灰)
- 中心: 显示作业总数
- 交互: hover 高亮扇区 + tooltip；click 跳转 `/jobs?status=<status>` (使用 `Router.navigate`)

### 3.3 颜色映射

| 状态 | 颜色 | HEX |
|:---|:---|:---|
| Running | 绿色 | `#4caf50` |
| Pending | 橙色 | `#ff9800` |
| Failed | 红色 | `#f44336` |
| Completed | 蓝色 | `#2196f3` |
| Unknown/Other | 灰色 | `#9e9e9e` |

### 3.4 文件变更

| 操作 | 文件 |
|:---|:---|
| 新增 | `web/src/app/dashboard/job-donut-chart/job-donut-chart.component.ts/html/scss` |
| 修改 | `web/src/app/app.module.ts` — 注册 `JobDonutChartComponent` |
| 修改 | `web/src/app/dashboard/dashboard.component.html` — 添加 donut chart 卡片 |

---

## 4. 实施顺序

| Step | 内容 | 预估 |
|:---|:---|:---|
| 4.1 | `RefreshService` + Toolbar 暂停按钮 | 10 min |
| 4.2 | 改造 Dashboard / Jobs / Nodes / Queues 接入 tick$ | 15 min |
| 4.3 | `ErrorService` + `ApiErrorInterceptor` + `ErrorBannerComponent` | 15 min |
| 4.4 | Dashboard Skeleton Loading | 5 min |
| 4.5 | `JobDonutChartComponent` (SVG Donut) | 20 min |
| 4.6 | 集成测试 & 验收 | 10 min |

**总计**: ~75 min

---

## 5. 验收标准

- [x] Toolbar 暂停按钮可切换全局轮询状态，所有页面同步响应
- [x] API 不可达时，顶部出现红色 Error Banner，可手动关闭
- [x] API 恢复后，Banner 自动消失，数据自动刷新
- [x] Dashboard 首屏显示 Skeleton 占位，数据加载后平滑过渡
- [x] Dashboard 新增 Donut Chart，正确展示各状态作业数量
- [x] 点击 Donut Chart 扇区可跳转到 Jobs 页面
