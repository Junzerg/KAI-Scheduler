# Phase 2.6: 作业资源详情 (Task Resource Details) — 实施计划

**日期**: 2026-02-11
**前置依赖**: Phase 2.5 完成
**范围**: 后端 + 前端，独立闭环

---

## 概述

在 Jobs 页面的展开行中，为每个 Task 展示 CPU/Memory/GPU 资源请求量。
当前展开行仅显示 Task 名称、状态和所在节点，缺少资源维度的信息。

**交付物**: 展开任意 Job 行后，每个 Task 卡片上可以看到 `2.0 cores / 4.0 GiB / 1 GPU` 形式的资源标签。

---

## 1. 后端改造 (Go)

### 1.1 扩展 TaskView 结构体

文件: `pkg/scheduler/api/visualizer_info/visualizer_info.go`

```go
type TaskView struct {
    Name      string         `json:"name"`
    Status    string         `json:"status"`
    NodeName  string         `json:"nodeName"`
    // Phase 2.6: 每个 Task 的资源请求
    Requested *ResourceStats `json:"requested,omitempty"`
}
```

### 1.2 填充资源数据

文件: `pkg/scheduler/visualizer/visualizer_service.go` — `GetJobs()` 方法

在构建 `TaskView` 时，汇总 Pod 的 `spec.containers[].resources.requests`，填入 `Requested` 字段。

---

## 2. 前端改造 (Angular)

### 2.1 更新 TypeScript Interface

文件: `web/src/app/visualizer.service.ts`

```typescript
export interface TaskView {
    name: string;
    status: string;
    nodeName: string;
    requested?: {
        milliCPU: number;
        memory: number;
        gpu: number;
    };
}
```

### 2.2 添加格式化方法

文件: `web/src/app/jobs/jobs.component.ts`

```typescript
formatCPU(milliCPU: number): string {
    return milliCPU >= 1000 ? `${(milliCPU / 1000).toFixed(1)} cores` : `${milliCPU}m`;
}

formatMemory(bytes: number): string {
    const gib = bytes / (1024 ** 3);
    if (gib >= 1) return `${gib.toFixed(1)} GiB`;
    return `${(bytes / (1024 ** 2)).toFixed(0)} MiB`;
}
```

### 2.3 更新展开行模板

文件: `web/src/app/jobs/jobs.component.html`

在 `task-card` 内追加资源标签区域：

```html
<div class="task-resources" *ngIf="task.requested">
    <span class="resource-tag"><mat-icon>memory</mat-icon> {{formatCPU(task.requested.milliCPU)}}</span>
    <span class="resource-tag"><mat-icon>storage</mat-icon> {{formatMemory(task.requested.memory)}}</span>
    <span class="resource-tag" *ngIf="task.requested.gpu > 0">
        <mat-icon>developer_board</mat-icon> {{task.requested.gpu}} GPU
    </span>
</div>
```

### 2.4 样式

文件: `web/src/app/jobs/jobs.component.scss`

```scss
.task-resources {
    display: flex; gap: 8px; margin-top: 4px; flex-wrap: wrap;
    .resource-tag {
        display: inline-flex; align-items: center; gap: 2px;
        font-size: 12px; color: rgba(255,255,255,0.7);
        mat-icon { font-size: 14px; width: 14px; height: 14px; }
    }
}
```

---

## 3. 文件变更清单

| 操作 | 文件 |
|:---|:---|
| 修改 | `pkg/scheduler/api/visualizer_info/visualizer_info.go` — `TaskView` 添加 `Requested` |
| 修改 | `pkg/scheduler/visualizer/visualizer_service.go` — `GetJobs()` 填充资源请求 |
| 修改 | `web/src/app/visualizer.service.ts` — 更新 `TaskView` interface |
| 修改 | `web/src/app/jobs/jobs.component.ts` — 添加 `formatCPU`/`formatMemory` |
| 修改 | `web/src/app/jobs/jobs.component.html` — 展开行添加资源标签 |
| 修改 | `web/src/app/jobs/jobs.component.scss` — 资源标签样式 |

---

## 4. 验收标准

- [ ] 后端 API `/api/jobs` 返回的每个 Task 包含 `requested` 字段
- [ ] 展开 Job 行后，每个 Task 卡片显示 CPU/Memory 资源标签
- [ ] GPU 请求为 0 时，GPU 标签不显示
- [ ] 资源值格式化为人类可读形式 (`2.5 cores`、`4.0 GiB`)
- [ ] 无资源信息时 (requested 为 null)，不显示资源区域，不报错
