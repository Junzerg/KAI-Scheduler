# Summary: Phase 2.3 HotFix - Node Resource Usage Visualization

## Overview
This document summarizes the changes made to the KAI Scheduler backend and frontend to support detailed node resource usage visualization (Allocatable vs Used) and preliminary GPU fragmentation logic. This was a HotFix identified during the development of the Node Topology page.

## Backend Changes

### 1. Updated `NodeView` Structure
**File**: `pkg/scheduler/api/visualizer_info/visualizer_info.go`

- **Change**: Replaced the single `Resources` field with `Allocatable` and `Used` fields to provide granular resource data.
- **New Structure**:
  ```go
  type NodeView struct {
      Name        string        `json:"name"`
      Status      string        `json:"status"`
      Allocatable ResourceStats `json:"allocatable"` // Capacity (Total)
      Used        ResourceStats `json:"used"`        // Usage (Allocated)
      GPUSlots    []*GPUSlot    `json:"gpuSlots"`
  }
  ```

### 2. Implemented Resource Population Logic
**File**: `pkg/scheduler/visualizer/visualizer_service.go`

- **Change**: Updated `GetNodes()` method to populate `Allocatable` and `Used` from the scheduler cache snapshot.
- **Helper Added**: Added `isGPUFragmented` stub method (currently returns `false` as a placeholder) for future implementation of fragmentation detection logic.
  ```go
  // isGPUFragmented checks if a GPU slot is fragmented.
  func (vs *visualizerService) isGPUFragmented(ni *node_info.NodeInfo, gpuID int) bool {
      // TODO(Phase 2.3): Implement detailed fragmentation logic.
      return false
  }
  ```

## Frontend Changes

### 1. Updated API Interface
**File**: `web/src/app/visualizer.service.ts`

- **Change**: Updated `NodeView` interface to match the backend response structure.
  ```typescript
  export interface NodeView {
    name: string;
    status: string;
    allocatable: ResourceStats;
    used: ResourceStats;
    gpuSlots: GPUSlot[];
  }
  ```

### 2. Enhanced Node Card Component
**Files**: `web/src/app/nodes/node-card/node-card.component.{ts,html,scss}`

- **Logic**: Updated getters `cpuCapacity` and `memCapacity` to use `allocatable` data.
- **New Features**: 
  - Added `cpuUsagePercent` and `memUsagePercent` calculations.
  - Implemented visual progress bars for CPU and Memory usage in the UI.
  - Added styles for the usage indicators.

## Verification

Verified the implementation by comparing the backend API response with `kubectl describe node` output.

**Backend API (`/api/v1/visualizer/nodes`)**:
```json
{
  "name": "desktop-worker",
  "status": "Ready",
  "allocatable": { "milliCPU": 28000, "memory": 33557352448, "gpu": 0 },
  "used": { "milliCPU": 500, "memory": 1335885824, "gpu": 0 }
}
```

**Kubernetes Cluster Status (`kubectl describe node desktop-worker`)**:
- **Allocatable**: CPU: 28, Memory: ~32GiB (Matches `allocatable`)
- **Requests (Used)**: CPU: 500m, Memory: 1274Mi (Matches `used`)

The data is consistent, confirming that the backend correctly retrieves and exposes resource usage information from the scheduler cache.

## Next Steps
- Implement the detailed logic for `isGPUFragmented` in Phase 2.4 or later to accurately identify fragmented GPU slots based on topology constraints (NVLink, NUMA).
