package visualizer

import "time"

// ClusterSummary represents the overall cluster health and resource usage for visualization.
type ClusterSummary struct {
	// TotalNodes is the total number of nodes in the cluster.
	TotalNodes int `json:"totalNodes"`
	// HealthyNodes is the number of nodes that are in Ready state.
	HealthyNodes int `json:"healthyNodes"`
	// TotalGPUs is the total number of GPU devices available in the cluster.
	TotalGPUs int `json:"totalGPUs"`
	// AllocatedGPUs is the number of GPU devices currently allocated to running jobs.
	AllocatedGPUs int `json:"allocatedGPUs"`
	// TotalQueues is the total number of queues in the system.
	TotalQueues int `json:"totalQueues"`
	// JobCounts maps job status (e.g., Pending, Running, Failed) to the count of jobs in that status.
	JobCounts map[string]int `json:"jobCounts"`
}

// ResourceStats represents the resources view (CPU, Memory, GPU).
type ResourceStats struct {
	// MilliCPU is the amount of CPU in milli-cores.
	MilliCPU int64 `json:"milliCPU"`
	// Memory is the amount of memory in bytes.
	Memory int64 `json:"memory"`
	// GPU is the number of GPUs.
	GPU int64 `json:"gpu"`
	// ScalarResources stores other scalar resources if any.
	ScalarResources map[string]int64 `json:"scalarResources,omitempty"`
}

// QueueResources represents the resource usage and limits of a queue.
type QueueResources struct {
	Guaranteed ResourceStats `json:"guaranteed"`
	Allocated  ResourceStats `json:"allocated"`
	Max        ResourceStats `json:"max"`
}

// QueueView represents a queue in the hierarchy.
type QueueView struct {
	Name      string          `json:"name"`
	Parent    string          `json:"parent"`
	Weight    int32           `json:"weight"`
	Resources *QueueResources `json:"resources"`
	Children  []*QueueView    `json:"children"`
}

// TaskView represents a task (pod) within a job.
type TaskView struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	NodeName string `json:"nodeName"`
}

// JobView represents a job with its status and tasks.
type JobView struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Queue     string `json:"queue"`
	Status    string `json:"status"`
	// CreateTime is the creation timestamp of the job.
	CreateTime time.Time   `json:"createTime"`
	Tasks      []*TaskView `json:"tasks"`
}

// GPUSlot represents a single GPU slot on a node.
type GPUSlot struct {
	// ID is the physical index of the GPU (0-7).
	ID int `json:"id"`
	// OccupiedBy is the Task ID or Pod Name occupying this slot. Empty if available.
	OccupiedBy string `json:"occupiedBy"`
	// Fragmented indicates if the slot is idle but unusable due to constraints (e.g. topology).
	Fragmented bool `json:"fragmented"`
}

// NodeView represents a cluster node with detailed resource and GPU info.
type NodeView struct {
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Resources ResourceStats `json:"resources"`
	GPUSlots  []*GPUSlot    `json:"gpuSlots"`
}
