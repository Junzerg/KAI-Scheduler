# Phase 2.3: 节点与 GPU 拓扑可视化实施计划 (Nodes & GPU Topology)

**目标**: 实现集群节点健康状态的网格视图，并提供细粒度的 GPU 插槽 (Slot) 可视化，以识别资源碎片和分配状态。

## 1. 数据层接口定义 (Data Layer)

### 1.1 更新 `VisualizerService`

在 `web/src/app/visualizer.service.ts` 中添加节点相关的数据获取方法和对应的 TypeScript 接口定义。

**接口定义 (DTOs)**:
由于后端 `pkg/scheduler/api/visualizer_info/visualizer_info.go` 已经有了 `NodeView` 和 `GPUSlot` 结构，我们需要在前端建立对应的映射。

```typescript
// 资源统计 (对应 Go: ResourceStats)
export interface ResourceStats {
  milliCPU: number;
  memory: number;
  gpu: number;
  scalarResources?: { [key: string]: number };
}

// GPU 插槽 (对应 Go: GPUSlot)
export interface GPUSlot {
  id: number;          // 物理槽位 ID (0-7)
  occupiedBy: string;  // 占用者 (Task Name / Pod Name)，为空表示 Free
  fragmented: boolean; // 是否为碎片 (因拓扑限制不可用)
}

// 节点视图 (对应 Go: NodeView)
export interface NodeView {
  name: string;
  status: string;      // 例如 "Ready", "NotReady"
  resources: ResourceStats;
  gpuSlots: GPUSlot[]; // 仅 GPU 节点有此字段，非 GPU 节点为空或 null
}
```

**API 方法**:
- `getNodes(): Observable<NodeView[]>`: 调用后端 `/api/v1/visualizer/nodes`。

---

## 2. 前端组件架构

我们将主要开发以下组件：

### 2.1 页面容器 (`NodesPageComponent`)
- **路径**: `src/app/pages/nodes-page/nodes-page.component`
- **职责**:
  - 调用 `VisualizerService.getNodes()` 获取数据。
  - 处理简单的客户端过滤（例如按状态筛选：All / Healthy / Unhealthy）。
  - 控制自动刷新逻辑 (使用 `RxJS timer` + `switchMap`)。
  - 布局：包含顶部工具栏（过滤、刷新按钮）和下方的节点网格。

### 2.2 节点网格 (`NodeGridComponent`)
- **路径**: `src/app/components/node-grid/node-grid.component`
- **职责**:
  - 接收 `NodeView[]` 输入。
  - 使用 `mat-grid-list` 或 Flexbox 响应式布局展示节点卡片。
  - 根据屏幕宽度动态调整列数。

### 2.3 节点卡片 (`NodeCardComponent`)
- **路径**: `src/app/components/node-card/node-card.component`
- **职责**:
  - 展示单个节点信息。
  - **头部**: 节点名称、状态徽章 (Green/Red)。
  - **内容区**:
    - **基础资源**: CPU/Memory 使用率（由于当前后端仅返回 Usage/Request，暂展示 Capacity vs Alleged Usage，或者仅展示数值）。
    - **GPU 拓扑**: 调用 `GpuSlotsComponent` 展示 GPU 布局。

### 2.4 GPU 插槽可视化 (`GpuSlotsComponent`)
- **路径**: `src/app/components/gpu-slots/gpu-slots.component`
- **职责**:
  - 接收 `GPUSlot[]` 输入。
  - **布局**: 模拟物理布局。对于标准的 8 卡机器 (如 DGX)，采用 2x4 或 4x2 网格布局。
  - **样式状态**:
    - **Free (空闲)**: 灰色/空心边框。
    - **Used (占用)**: 绿色/实心填充。
    - **Fragmented (碎片)**: 黄色/橙色斜纹或半透明，表示“有空位但受限”。
  - **交互**:
    - `mat-tooltip`: 鼠标悬停显示占用者名称 (`occupiedBy`) 和 Slot ID。

---

## 3. UI/UX 设计细节

- **颜色编码**:
  - **Status**: Ready (Green-500), NotReady (Red-500).
  - **GPU**:
    - Free: `bg-gray-100` / `border-gray-300`
    - Used: `bg-green-500` (或根据 Job 颜色 Hash)
    - Fragmented: `bg-yellow-100` + `border-yellow-400` (Striped pattern if possible)

- **响应式**:
  - 宽屏 (Desktop): 4-5 列节点卡片。
  - 平板 (Tablet): 2-3 列。
  - 手机 (Mobile): 1 列。

---

## 4. 实施步骤 (Execution Steps)

1.  **Step 1: 服务层更新**
    - 修改 `visualizer.service.ts`，添加接口定义和 `getNodes` 方法。

2.  **Step 2: 组件脚手架**
    - 创建 `NodesPageComponent`, `NodeGridComponent`, `NodeCardComponent`, `GpuSlotsComponent`。
    - 配置路由 `app-routing.module.ts` 添加 `/nodes`。

3.  **Step 3: 基础网格实现**
    - 实现 `NodesPageComponent` 获取数据。
    - 实现 `NodeCardComponent` 展示基础信息 (Name, Status)。
    - 验证数据联通性。

4.  **Step 4: GPU 拓扑可视化 (核心)**
    - 实现 `GpuSlotsComponent`。
    - CSS Grid 布局 8 个 GPU Slot。
    - 根据 `occupiedBy` 和 `fragmented` 字段应用样式。
    - 添加 Tooltip 交互。

5.  **Step 5: 样式打磨与集成**
    - 调整卡片间距、阴影。
    - 确保与 Dashboard 和 Sidebar 的导航联动。
    - 增加自动刷新。

---

## 5. 验证标准 (Acceptance Criteria)

- [ ] 访问 `/nodes` 页面能看到后端返回的所有节点。
- [ ] 节点卡片正确显示节点名称和健康状态。
- [ ] GPU 节点正确渲染出 8 个 (或对应数量) 插槽方块。
- [ ] 已分配的 GPU 插槽显示为绿色，Tooltip 显示 Job/Task 名称。
- [ ] 模拟的“碎片”状态 (如果有) 能被视觉区分 (例如黄色高亮)。
- [ ] 页面在后端数据变化时能自动刷新 (或手动刷新)。
