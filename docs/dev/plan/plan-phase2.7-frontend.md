# Phase 2.7: 队列详情面板 (Queue Detail Panel) — 实施计划

**日期**: 2026-02-11
**前置依赖**: Phase 2.6 完成
**范围**: 后端 + 前端，独立闭环

---

## 概述

在 Queues 页面添加点击交互：点击任意队列节点，右侧滑出详情面板，展示该队列的完整资源统计和队列内作业状态分布。

**交付物**: 点击队列树节点 → 右侧面板滑出，展示 CPU/Memory/GPU 详细数值 + 该队列下的作业计数 (Running/Pending/Failed)。

---

## 1. 后端改造 (Go)

### 1.1 扩展 QueueView 结构体

文件: `pkg/scheduler/api/visualizer_info/visualizer_info.go`

```go
type QueueView struct {
    Name      string          `json:"name"`
    Parent    string          `json:"parent"`
    Weight    int32           `json:"weight"`
    Resources *QueueResources `json:"resources"`
    Children  []*QueueView    `json:"children"`
    // Phase 2.7: 队列内作业状态统计
    JobCounts map[string]int  `json:"jobCounts,omitempty"`
}
```

### 1.2 填充作业统计

文件: `pkg/scheduler/visualizer/visualizer_service.go` — `GetQueues()` 方法

遍历 Scheduler 内部的 Job 列表，按 `job.Queue` 分组统计各状态计数，填入对应队列的 `JobCounts`。

---

## 2. 前端改造 (Angular)

### 2.1 更新 TypeScript Interface

文件: `web/src/app/visualizer.service.ts`

```typescript
export interface QueueView {
    name: string;
    parent: string;
    weight: number;
    resources: QueueResources;
    children: QueueView[];
    jobCounts?: { [status: string]: number };  // Phase 2.7
}
```

### 2.2 新建 QueueDetailPanelComponent

```
web/src/app/queues/queue-detail-panel/
    queue-detail-panel.component.ts
    queue-detail-panel.component.html
    queue-detail-panel.component.scss
```

**输入**: `@Input() queue: QueueFlatNode | null`
**输出**: `@Output() close = new EventEmitter<void>()`

面板内容：
- 队列名称 + Weight
- 三行资源条 (复用 `QueueResourceBarComponent`)，展示精确数值
- 作业状态统计列表 (Running / Pending / Failed / Completed 计数)
- 关闭按钮

### 2.3 改造 QueuesComponent

文件: `web/src/app/queues/queues.component.ts`

```typescript
selectedQueue: QueueFlatNode | null = null;

onQueueClick(node: QueueFlatNode): void {
    this.selectedQueue =
        this.selectedQueue?.name === node.name ? null : node;
}
```

### 2.4 改造模板布局

文件: `web/src/app/queues/queues.component.html`

将现有 `mat-tree` 包裹在 `mat-sidenav-container` 中，添加右侧滑出面板：

```html
<mat-sidenav-container class="queues-sidenav-container">
    <mat-sidenav-content>
        <!-- 现有 mat-tree 内容 -->
    </mat-sidenav-content>

    <mat-sidenav #detailPanel mode="side" position="end"
                 [opened]="!!selectedQueue" class="queue-detail-sidenav">
        <app-queue-detail-panel
            [queue]="selectedQueue"
            (close)="selectedQueue = null">
        </app-queue-detail-panel>
    </mat-sidenav>
</mat-sidenav-container>
```

### 2.5 更新 QueueFlatNode

在 `queues.component.ts` 中扩展 `QueueFlatNode` interface，添加 `jobCounts` 字段，并在 `_transformer` 中传递。

---

## 3. 文件变更清单

| 操作 | 文件 |
|:---|:---|
| 修改 | `pkg/scheduler/api/visualizer_info/visualizer_info.go` — `QueueView` 添加 `JobCounts` |
| 修改 | `pkg/scheduler/visualizer/visualizer_service.go` — 统计队列内作业数 |
| 新增 | `web/src/app/queues/queue-detail-panel/queue-detail-panel.component.ts/html/scss` |
| 修改 | `web/src/app/queues/queues.component.ts` — `selectedQueue` + click handler |
| 修改 | `web/src/app/queues/queues.component.html` — 包装 `mat-sidenav-container` |
| 修改 | `web/src/app/queues/queues.component.scss` — 面板布局样式 |
| 修改 | `web/src/app/visualizer.service.ts` — `QueueView` interface 添加 `jobCounts` |
| 修改 | `web/src/app/app.module.ts` — 注册 `QueueDetailPanelComponent` |

---

## 4. 验收标准

- [ ] 后端 API `/api/queues` 返回的每个队列包含 `jobCounts` 字段
- [ ] 点击队列树节点，右侧面板滑出，展示队列名称和 Weight
- [ ] 面板展示 CPU/Memory/GPU 三组资源条 (Guaranteed / Allocated / Max)
- [ ] 面板展示该队列内的作业状态分布计数
- [ ] 再次点击同一节点或点击关闭按钮，面板收起
- [ ] 切换选中不同队列节点，面板内容即时更新
