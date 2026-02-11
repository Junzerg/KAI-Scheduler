# Phase 2.4 Frontend Implementation Summary — Queue Hierarchy Visualization

## Status: Completed ✅

成功实现了 **队列层级可视化 (Queue Hierarchy)** 模块，包括前端组件开发和后端数据适配修复。

---

## 交付物清单

### 前端

| 文件 | 说明 |
|:---|:---|
| `web/src/app/queues/queues.component.ts/html/scss` | 队列主页面，基于 `mat-tree` (FlatTreeControl) 实现嵌套层级展示，默认展开所有节点，5 秒轮询刷新 |
| `web/src/app/queues/queue-resource-bar/queue-resource-bar.component.ts/html/scss` | 可复用的资源进度条组件，同时展示 Allocated / Guaranteed / Max 三元关系 |
| `web/src/app/visualizer.service.ts` | 新增 `QueueView`、`QueueResources` 接口 + `getQueues()` API 调用 |
| `web/src/app/shared.module.ts` | 新增 `MatTreeModule` |
| `web/src/app/app.module.ts` | 注册 `QueuesComponent`、`QueueResourceBarComponent` |
| `web/src/app/app-routing.module.ts` | 添加 `/queues` 路由 |
| `web/src/app/app.component.html` | 侧边栏 "Queues" 链接已激活（移除 disabled 和 Phase 2.4 标记） |

### 后端修复

| 文件 | 修改内容 |
|:---|:---|
| `pkg/scheduler/visualizer/visualizer_service.go` | **3 项修复**（详见下文） |

---

## 技术决策记录

### 可视化方案：`mat-tree` (树形表格) vs D3.js (Treemap/Sunburst)

**最终选择：`mat-tree` 层次化树形表格**

- **理由**: 队列管理场景下，管理员需要精准对比 Usage / Guaranteed / Max 的三元数值关系。树形表格可以在每行内嵌进度条，支持多列水平对齐，远优于 Treemap 在深层嵌套时的标签重叠问题。
- **详细对比分析**: 见 `docs/dev/plan/plan-phase2.4-frontend.md` Section 3。

---

## 后端修复详情

### 修复 1: 队列实际使用量计算方式

**问题**: `qi.ResourceUsage` 和 `snapshot.QueueResourceUsage` 均为空（前者初始化时未填充，后者依赖 Prometheus 且本地无连接）。  
**方案**: 直接从 `snapshot.PodGroupInfos` 中聚合——遍历每个 PodGroup 的所有 Active Pod，按 `Queue` 字段累加其 `ResReq`（CPU/Memory/GPU）。  
**性能影响**: 可忽略。`Snapshot()` 本身是开销大头，额外遍历 PodGroupInfos 只是线性扫描。

### 修复 2: 父队列资源冒泡

**问题**: Jobs 仅分配到叶子队列，父队列天然无直接占用量。  
**方案**: 新增 `accumulateChildUsage()` 递归函数，在构建树形层级后做一次后序遍历 (Post-order)，将子队列 Allocated 逐层向上累加到父队列。

### 修复 3: 负值配额处理

**问题**: KAI CRD 中 Quota/Limit 的 `-1` 表示"无限制"，直接传到前端会导致进度条倒转或异常显示。  
**方案**: 新增 `clampNeg()` 辅助函数，将负值 clamp 为 0。前端进度条逻辑已处理 `max <= 0` 为"无上限"模式。

---

## 前端组件设计亮点

### QueueResourceBarComponent

- **三元进度条**: 一根进度条同时展示 Usage（前景色条）、Guaranteed（黑色刻度线）、Max（灰色底色 = 100%）。
- **动态色彩**: 
  - 🟢 绿色: Usage < Guaranteed（正常）
  - 🟠 橙色: Usage > Guaranteed（超配额）
  - 🔴 红色: Usage > Max（超限预警）
- **智能单位格式化**: 
  - CPU: `100m` / `1.5 cores`（自动根据量级切换）
  - Memory: `238.4 MiB` / `1.25 GiB`（自动从 bytes 转换）
  - GPU: 原值

---

## 已知限制 & 后续规划

1. **队列详情侧边栏**: 计划在 Phase 2.5 中实现点击队列行弹出详情面板（含 CPU/MEM/GPU 分别的利用率图表）。
2. **Guaranteed 显示为 0**: 当前集群的默认队列 Quota 为 `-1`（无限制），clamp 后显示为 `0`。当用户通过 CRD 设置了实际配额后，进度条的 Guaranteed 刻度线和占比关系将自动生效。
3. **Treemap 可选视图**: 未来可作为"图形化概览"的补充视图，通过 Tab 切换实现。

---

## 文件变更汇总

```
# 新增
web/src/app/queues/queues.component.ts
web/src/app/queues/queues.component.html
web/src/app/queues/queues.component.scss
web/src/app/queues/queue-resource-bar/queue-resource-bar.component.ts
web/src/app/queues/queue-resource-bar/queue-resource-bar.component.html
web/src/app/queues/queue-resource-bar/queue-resource-bar.component.scss
docs/dev/plan/plan-phase2.4-frontend.md
docs/dev/summary/summary-phase2.4-frontend.md (本文档)

# 修改
web/src/app/visualizer.service.ts          (新增 QueueView 接口 + getQueues)
web/src/app/shared.module.ts               (添加 MatTreeModule)
web/src/app/app.module.ts                  (注册新组件)
web/src/app/app-routing.module.ts          (添加 /queues 路由)
web/src/app/app.component.html             (激活侧边栏 Queues 链接)
pkg/scheduler/visualizer/visualizer_service.go  (3 项后端修复)
```
