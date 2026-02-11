# Phase 2.3 HotFix / Feature Request Plan

This document tracks backend API gaps and feature requests identified during frontend development of Phase 2.3 (Nodes & GPU Topology).

## 1. Node Resource Usage (Missing in `NodeView`)

**Problem**:
The current `NodeView` struct only returns `Resources` which corresponds to the node's **Allocatable** (Capacity).
To display utilization bars (Unused vs Used), the frontend needs access to both **Allocatable** and **Used** resources.

**Current API Response (`/api/v1/visualizer/nodes`)**:
```json
{
  "name": "node-1",
  "status": "Ready",
  "resources": { "milliCPU": 32000, "memory": 64000000000, "gpu": 8 }, // Only Allocatable
  "gpuSlots": [...]
}
```

**Proposed Change**:
Update `visualizer_info.go`'s `NodeView` struct to include usage stats.

```go
type NodeView struct {
    Name        string          `json:"name"`
    Status      string          `json:"status"`
    Allocatable ResourceStats   `json:"allocatable"` // Renamed or kept as Resources(Capacity)
    Used        ResourceStats   `json:"used"`        // New field
    GPUSlots    []*GPUSlot      `json:"gpuSlots"`
}
```

**Impact on Frontend**:
- Frontend currently cannot calculate or display accurate CPU/Memory utilization percentages.
- Will display "Capacity: X" instead of progress bars for now.

## 2. GPU Fragmentation Logic

**Problem**:
The `GPUSlot` struct has a `Fragmented` boolean field, but the backend implementation currently defaults this to `false`.

**Proposed Change**:
Implement logic in `GetNodes` to analyze the node's topology constraints (e.g., NVLink, NUMA) and pending pods to determine if a free slot is truly usable or fragmented.

## 3. Detailed Task Info in Slots

**Problem**:
`GPUSlot.OccupiedBy` is a simple string. It might be better to have structured data (Pod Name, Namespace, Link to Job) to enable clicking through to the Job details.
