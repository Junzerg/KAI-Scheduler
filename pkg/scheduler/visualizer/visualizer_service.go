/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/

package visualizer

import (
	"strconv"

	v1 "k8s.io/api/core/v1"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/resource_info"
	vizinfo "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/visualizer_info"
	schedcache "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache"
)

// VisualizerService defines the interface for converting scheduler cache data
// into visualization-friendly models.
type VisualizerService interface {
	GetClusterSummary() (*vizinfo.ClusterSummary, error)
	GetQueues() ([]*vizinfo.QueueView, error)
	GetJobs(namespace string) ([]*vizinfo.JobView, error)
	GetNodes() ([]*vizinfo.NodeView, error)
}

// visualizerService implements the VisualizerService interface.
type visualizerService struct {
	schedulerCache schedcache.Cache
}

// NewVisualizerService creates a new instance of VisualizerService.
func NewVisualizerService(schedulerCache schedcache.Cache) VisualizerService {
	return &visualizerService{
		schedulerCache: schedulerCache,
	}
}

// GetClusterSummary returns the overall check summary of the cluster.
func (vs *visualizerService) GetClusterSummary() (*vizinfo.ClusterSummary, error) {
	snapshot, err := vs.schedulerCache.Snapshot()
	if err != nil {
		return nil, err
	}

	summary := &vizinfo.ClusterSummary{
		JobCounts: make(map[string]int),
	}

	// Nodes and GPUs
	// Snapshot returns api.ClusterInfo, which contains Nodes map[string]*node_info.NodeInfo
	for _, nodeInfo := range snapshot.Nodes {
		summary.TotalNodes++
		if isNodeReady(nodeInfo.Node) {
			summary.HealthyNodes++
		}
		// Assuming Allocatable and Used are *resource_info.Resource and have GPUs() returns float64
		// We cast to int as per the ClusterSummary struct definition
		summary.TotalGPUs += int(nodeInfo.Allocatable.GPUs())
		summary.AllocatedGPUs += int(nodeInfo.Used.GPUs())
	}

	// Queues
	summary.TotalQueues = len(snapshot.Queues)

	// Jobs
	for _, pgi := range snapshot.PodGroupInfos {
		status := getJobStatus(pgi)
		summary.JobCounts[status]++
	}

	return summary, nil
}

// GetQueues returns the queue hierarchy with resource usage.
func (vs *visualizerService) GetQueues() ([]*vizinfo.QueueView, error) {
	snapshot, err := vs.schedulerCache.Snapshot()
	if err != nil {
		return nil, err
	}

	// 0. Compute actual resource usage per queue from PodGroupInfos
	//    This is the most reliable source of truth—it works even without Prometheus.
	type queueAllocated struct {
		milliCPU int64
		memory   int64
		gpu      int64
	}
	queueUsageMap := make(map[string]*queueAllocated)

	for _, pgi := range snapshot.PodGroupInfos {
		queueName := string(pgi.Queue)
		if queueName == "" {
			continue
		}
		if _, exists := queueUsageMap[queueName]; !exists {
			queueUsageMap[queueName] = &queueAllocated{}
		}
		qa := queueUsageMap[queueName]

		for _, podInfo := range pgi.GetAllPodsMap() {
			if !pod_status.IsActiveUsedStatus(podInfo.Status) {
				continue
			}
			if podInfo.ResReq != nil {
				qa.milliCPU += int64(podInfo.ResReq.Cpu())
				qa.memory += int64(podInfo.ResReq.Memory())
				qa.gpu += int64(podInfo.ResReq.GPUs())
			}
		}
	}

	queueViews := make(map[string]*vizinfo.QueueView)

	// 1. Create all view objects
	for id, qi := range snapshot.Queues {
		// Build allocated stats from our computed usage
		allocated := vizinfo.ResourceStats{}
		if qa, found := queueUsageMap[string(id)]; found {
			allocated = vizinfo.ResourceStats{
				MilliCPU: qa.milliCPU,
				Memory:   qa.memory,
				GPU:      qa.gpu,
			}
		}

		view := &vizinfo.QueueView{
			Name:   qi.Name,
			Parent: string(qi.ParentQueue),
			Weight: int32(qi.Priority),
			Resources: &vizinfo.QueueResources{
				Guaranteed: convertQuotaToStats(qi.Resources),
				Allocated:  allocated,
				Max:        convertQuotaLimitToStats(qi.Resources),
			},
			Children: []*vizinfo.QueueView{},
		}
		queueViews[string(id)] = view
	}

	// 2. Build Hierarchy
	var roots []*vizinfo.QueueView
	for _, view := range queueViews {
		if view.Parent == "" {
			roots = append(roots, view)
			continue
		}

		parent, exists := queueViews[view.Parent]
		if !exists {
			// If parent is missing, treat as root to ensure visibility
			roots = append(roots, view)
		} else {
			parent.Children = append(parent.Children, view)
		}
	}

	// 3. Bubble up: accumulate child usage into parent queues (post-order)
	for _, root := range roots {
		accumulateChildUsage(root)
	}

	return roots, nil
}

// accumulateChildUsage recursively sums child allocated resources into the parent.
func accumulateChildUsage(node *vizinfo.QueueView) {
	for _, child := range node.Children {
		accumulateChildUsage(child)
		node.Resources.Allocated.MilliCPU += child.Resources.Allocated.MilliCPU
		node.Resources.Allocated.Memory += child.Resources.Allocated.Memory
		node.Resources.Allocated.GPU += child.Resources.Allocated.GPU
	}
}

// GetJobs returns the list of jobs and their tasks, optionally filtered by namespace.
func (vs *visualizerService) GetJobs(namespace string) ([]*vizinfo.JobView, error) {
	snapshot, err := vs.schedulerCache.Snapshot()
	if err != nil {
		return nil, err
	}

	var jobs []*vizinfo.JobView

	for _, pgi := range snapshot.PodGroupInfos {
		if namespace != "" && pgi.Namespace != namespace {
			continue
		}

		jobView := &vizinfo.JobView{
			UID:        string(pgi.UID),
			Name:       pgi.Name,
			Namespace:  pgi.Namespace,
			Queue:      string(pgi.Queue),
			Status:     getJobStatus(pgi),
			CreateTime: pgi.CreationTimestamp.Time,
			Tasks:      []*vizinfo.TaskView{},
		}

		for _, podInfo := range pgi.GetAllPodsMap() {
			taskView := &vizinfo.TaskView{
				Name:     podInfo.Name,
				Status:   podInfo.Status.String(),
				NodeName: podInfo.NodeName,
			}
			jobView.Tasks = append(jobView.Tasks, taskView)
		}

		jobs = append(jobs, jobView)
	}
	return jobs, nil
}

// GetNodes returns the list of nodes with their GPU slot usage.
func (vs *visualizerService) GetNodes() ([]*vizinfo.NodeView, error) {
	snapshot, err := vs.schedulerCache.Snapshot()
	if err != nil {
		return nil, err
	}

	var nodes []*vizinfo.NodeView
	for _, ni := range snapshot.Nodes {
		nv := &vizinfo.NodeView{
			Name: ni.Name,
			Status: func() string {
				if isNodeReady(ni.Node) {
					return "Ready"
				}
				return "NotReady"
			}(),
			Allocatable: convertResourceToStats(ni.Allocatable),
			Used:        convertResourceToStats(ni.Used),
			GPUSlots:    make([]*vizinfo.GPUSlot, 0),
		}

		// Fill GPUSlots
		// Assuming GetNumberOfGPUsInNode returns appropriate count (e.g., 8 for DGX)
		totalGPUs := ni.GetNumberOfGPUsInNode()
		for i := 0; i < int(totalGPUs); i++ {
			nv.GPUSlots = append(nv.GPUSlots, &vizinfo.GPUSlot{
				ID:         i,
				OccupiedBy: "",
				Fragmented: vs.isGPUFragmented(ni, i),
			})
		}

		// Map Pods to Slots
		// We iterate over all pods on the node to check which GPU they are using.
		// GPU mapping is typically found in GPUGroups field for KAI Scheduler.
		for _, pod := range ni.PodInfos {
			if !pod_status.IsActiveUsedStatus(pod.Status) {
				continue
			}

			// Parse GPUGroups to find occupied slots
			// GPUGroups typically contains strings like "0", "1", "0,1,2,3" etc.
			// TODO(Phase 1.3): Implement complex GPUGroups parsing (e.g., comma-separated strings, ranges) and accurate Fragmentation logic.
			for _, gpuIdxStr := range pod.GPUGroups {
				// Handle potential complex strings if necessary, but assuming single integer strings for slots
				idx, err := strconv.Atoi(gpuIdxStr)
				if err == nil && idx >= 0 && idx < len(nv.GPUSlots) {
					// If multiple pods share a GPU (e.g. MIG or Time-slicing), we append names
					if nv.GPUSlots[idx].OccupiedBy == "" {
						nv.GPUSlots[idx].OccupiedBy = pod.Name
					} else {
						nv.GPUSlots[idx].OccupiedBy += "," + pod.Name
					}
				}
			}
		}

		nodes = append(nodes, nv)
	}
	return nodes, nil
}

// Helper functions

func isNodeReady(node *v1.Node) bool {
	if node == nil {
		return false
	}
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func getJobStatus(pgi *podgroup_info.PodGroupInfo) string {
	if pgi.GetNumActiveUsedTasks() > 0 {
		return "Running"
	}
	if pgi.GetNumPendingTasks() > 0 {
		return "Pending"
	}
	// Fallback status for jobs that are neither running nor pending (e.g. Failed, Succeeded, Unknown)
	// For visualization purpose, we categorize them as Failed or Completed if we had that state.
	// Based on requirement generic "Failed" for non-active/pending.
	// Based on requirement generic "Failed" for non-active/pending.
	return "Failed"
}

// Resource conversion helpers

// clampNeg clamps a value to 0 if negative (negative means "unlimited" in KAI CRD spec).
func clampNeg(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}

func convertQuotaToStats(q queue_info.QueueQuota) vizinfo.ResourceStats {
	return vizinfo.ResourceStats{
		MilliCPU: int64(clampNeg(q.CPU.Quota) * 1000),
		Memory:   int64(clampNeg(q.Memory.Quota)),
		GPU:      int64(clampNeg(q.GPU.Quota)),
	}
}

func convertQuotaLimitToStats(q queue_info.QueueQuota) vizinfo.ResourceStats {
	return vizinfo.ResourceStats{
		MilliCPU: int64(clampNeg(q.CPU.Limit) * 1000),
		Memory:   int64(clampNeg(q.Memory.Limit)),
		GPU:      int64(clampNeg(q.GPU.Limit)),
	}
}

func convertUsageToStats(u queue_info.QueueUsage) vizinfo.ResourceStats {
	// QueueUsage is map[v1.ResourceName]float64
	return vizinfo.ResourceStats{
		MilliCPU: int64(u[v1.ResourceCPU] * 1000),
		Memory:   int64(u[v1.ResourceMemory]),
		GPU:      int64(u[resource_info.GPUResourceName]),
	}
}

func convertResourceToStats(r *resource_info.Resource) vizinfo.ResourceStats {
	if r == nil {
		return vizinfo.ResourceStats{}
	}
	return vizinfo.ResourceStats{
		MilliCPU: int64(r.Cpu()),
		Memory:   int64(r.Memory()),
		GPU:      int64(r.GPUs()),
	}
}

// isGPUFragmented checks if a GPU slot is fragmented.
func (vs *visualizerService) isGPUFragmented(ni *node_info.NodeInfo, gpuID int) bool {
	// TODO(Phase 2.3): Implement detailed fragmentation logic.
	// This would involve checking:
	// 1. Is the GPU idle?
	// 2. Are there pending pods that need this GPU but can't use it due to topology constraints (e.g. NVLink)?
	// For now, return false as placeholder.
	return false
}
