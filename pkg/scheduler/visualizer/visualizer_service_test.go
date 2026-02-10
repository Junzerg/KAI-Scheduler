/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/

package visualizer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	k8sframework "k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/eviction_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/node_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/pod_status"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/podgroup_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/queue_info"
	vizinfo "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/visualizer_info"
	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/cache/cluster_info/data_lister"
	k8splugins "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/k8s_internal/plugins"
)

// MockCache implements schedcache.Cache for testing purposes.
type MockCache struct {
	snapshot *api.ClusterInfo
	err      error
}

func (m *MockCache) Snapshot() (*api.ClusterInfo, error) {
	return m.snapshot, m.err
}

// Stubs for other interface methods
func (m *MockCache) Run(stopCh <-chan struct{})              {}
func (m *MockCache) WaitForCacheSync(stopCh <-chan struct{}) {}
func (m *MockCache) Bind(podInfo *pod_info.PodInfo, hostname string, bindRequestAnnotations map[string]string) error {
	return nil
}
func (m *MockCache) Evict(ssnPod *v1.Pod, job *podgroup_info.PodGroupInfo, evictionMetadata eviction_info.EvictionMetadata, message string) error {
	return nil
}
func (m *MockCache) RecordJobStatusEvent(job *podgroup_info.PodGroupInfo) error { return nil }
func (m *MockCache) TaskPipelined(task *pod_info.PodInfo, message string)       {}
func (m *MockCache) KubeClient() kubernetes.Interface                           { return nil }
func (m *MockCache) KubeInformerFactory() informers.SharedInformerFactory       { return nil }
func (m *MockCache) SnapshotSharedLister() k8sframework.NodeInfoLister          { return nil }
func (m *MockCache) InternalK8sPlugins() *k8splugins.K8sPlugins                 { return nil }
func (m *MockCache) WaitForWorkers(stopCh <-chan struct{})                      {}
func (m *MockCache) GetDataLister() data_lister.DataLister                      { return nil }

type StubNodePodAffinityInfo struct{}

func (s *StubNodePodAffinityInfo) AddPod(*v1.Pod)                   {}
func (s *StubNodePodAffinityInfo) RemovePod(*v1.Pod) error          { return nil }
func (s *StubNodePodAffinityInfo) HasPodsWithPodAffinity() bool     { return false }
func (s *StubNodePodAffinityInfo) HasPodsWithPodAntiAffinity() bool { return false }
func (s *StubNodePodAffinityInfo) Name() string                     { return "" }

func TestGetClusterSummary(t *testing.T) {
	// Setup
	ci := api.NewClusterInfo()

	// Add Nodes
	node1 := createNode("node1", true, 8)
	node2 := createNode("node2", false, 4)
	ci.Nodes["node1"] = node1
	ci.Nodes["node2"] = node2

	// Add Queues
	ci.Queues["root"] = &queue_info.QueueInfo{Name: "root"}
	ci.Queues["root.a"] = &queue_info.QueueInfo{Name: "a"}

	// Add Jobs
	job1 := createJob("job1", "default", 2)
	ci.PodGroupInfos["job1"] = job1

	mockCache := &MockCache{snapshot: ci}
	service := NewVisualizerService(mockCache)

	// Execute
	summary, err := service.GetClusterSummary()

	// Verify
	assert.NoError(t, err)
	assert.Equal(t, 2, summary.TotalNodes)
	assert.Equal(t, 1, summary.HealthyNodes)
	assert.Equal(t, 12, summary.TotalGPUs) // 8 + 4
	assert.Equal(t, 2, summary.TotalQueues)
	assert.Equal(t, 1, summary.JobCounts["Pending"]) // Job has pending tasks
}

func TestGetQueues_DeepHierarchy(t *testing.T) {
	ci := api.NewClusterInfo()

	// Create deep hierarchy: root -> a -> b -> c
	qRoot := createQueue("root", "", 100)
	qA := createQueue("root.a", "root", 50)
	qB := createQueue("root.a.b", "root.a", 30)
	qC := createQueue("root.a.b.c", "root.a.b", 10)

	ci.Queues[qRoot.UID] = qRoot
	ci.Queues[qA.UID] = qA
	ci.Queues[qB.UID] = qB
	ci.Queues[qC.UID] = qC

	mockCache := &MockCache{snapshot: ci}
	service := NewVisualizerService(mockCache)

	queues, err := service.GetQueues()
	assert.NoError(t, err)

	// Find root
	var root *vizinfo.QueueView
	for _, q := range queues {
		if q.Name == "root" {
			root = q
			break
		}
	}
	assert.NotNil(t, root)
	assert.Equal(t, 1, len(root.Children))
	assert.Equal(t, "root.a", root.Children[0].Name)
	assert.Equal(t, "root.a.b", root.Children[0].Children[0].Name)
	assert.Equal(t, "root.a.b.c", root.Children[0].Children[0].Children[0].Name)

	// Verify Resources for root queue (set in createQueue)
	// Guaranteed CPU: 10 * 1000 = 10000m
	// Guaranteed Memory: 1024
	// Guaranteed GPU: 1
	assert.Equal(t, int64(10000), root.Resources.Guaranteed.MilliCPU)
	assert.Equal(t, int64(1024), root.Resources.Guaranteed.Memory)
	assert.Equal(t, int64(1), root.Resources.Guaranteed.GPU)
}

func TestGetJobs_NamespaceFilter(t *testing.T) {
	ci := api.NewClusterInfo()

	job1 := createJob("job1", "ns1", 1)
	job2 := createJob("job2", "ns2", 1)

	ci.PodGroupInfos[job1.UID] = job1
	ci.PodGroupInfos[job2.UID] = job2

	mockCache := &MockCache{snapshot: ci}
	service := NewVisualizerService(mockCache)

	// Filter by ns1
	jobs, err := service.GetJobs("ns1")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(jobs))
	assert.Equal(t, "job1", jobs[0].Name)

	// No filter
	jobsAll, err := service.GetJobs("")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(jobsAll))
}

func TestGetNodes_GPUSlots(t *testing.T) {
	ci := api.NewClusterInfo()

	node := createNode("node-gpu", true, 4)
	task := pod_info.NewTaskInfo(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "task1", Namespace: "default", UID: "task1"},
		Spec:       v1.PodSpec{NodeName: "node-gpu"},
	})
	task.GPUGroups = []string{"1", "3"} // Occupy slot 1 and 3
	task.Status = pod_status.Running    // Must be active used

	// Manually add task to node
	node.PodInfos[task.UID] = task
	ci.Nodes["node-gpu"] = node

	mockCache := &MockCache{snapshot: ci}
	service := NewVisualizerService(mockCache)

	nodes, err := service.GetNodes()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(nodes))

	gpuNode := nodes[0]
	assert.Equal(t, 4, len(gpuNode.GPUSlots))

	// Check slots
	assert.Equal(t, "", gpuNode.GPUSlots[0].OccupiedBy)
	assert.Equal(t, "task1", gpuNode.GPUSlots[1].OccupiedBy)
	assert.Equal(t, "", gpuNode.GPUSlots[2].OccupiedBy)
	assert.Equal(t, "task1", gpuNode.GPUSlots[3].OccupiedBy)
}

// Additional robustness tests

func TestSnapshotError(t *testing.T) {
	mockCache := &MockCache{
		snapshot: nil,
		err:      errors.New("snapshot failed"),
	}
	service := NewVisualizerService(mockCache)

	_, err := service.GetClusterSummary()
	assert.Error(t, err)
	assert.Equal(t, "snapshot failed", err.Error())

	_, err = service.GetQueues()
	assert.Error(t, err)

	_, err = service.GetJobs("")
	assert.Error(t, err)

	_, err = service.GetNodes()
	assert.Error(t, err)
}

func TestJobStates(t *testing.T) {
	ci := api.NewClusterInfo()

	// Pending Job
	jobPending := createJob("job-pending", "default", 1)
	ci.PodGroupInfos["job-pending"] = jobPending

	// Running Job
	jobRunning := createJob("job-running", "default", 1)
	// Force task status to Running
	for _, task := range jobRunning.GetAllPodsMap() {
		// We need to use UpdateTaskStatus to ensure internal state consistency
		jobRunning.UpdateTaskStatus(task, pod_status.Running)
	}
	ci.PodGroupInfos["job-running"] = jobRunning

	// Failed/Other Job (Simulated by having no active or pending tasks)
	// Create with 0 tasks to simulate no active/pending
	jobFailed := createJob("job-failed", "default", 0)
	ci.PodGroupInfos["job-failed"] = jobFailed

	mockCache := &MockCache{snapshot: ci}
	service := NewVisualizerService(mockCache)

	summary, err := service.GetClusterSummary()
	assert.NoError(t, err)
	assert.Equal(t, 1, summary.JobCounts["Pending"])
	assert.Equal(t, 1, summary.JobCounts["Running"])
	assert.Equal(t, 1, summary.JobCounts["Failed"])

	jobs, err := service.GetJobs("")
	assert.NoError(t, err)
	for _, j := range jobs {
		if j.Name == "job-pending" {
			assert.Equal(t, "Pending", j.Status)
		} else if j.Name == "job-running" {
			assert.Equal(t, "Running", j.Status)
		} else if j.Name == "job-failed" {
			assert.Equal(t, "Failed", j.Status)
		}
	}
}

func TestComplexGPUGroups(t *testing.T) {
	ci := api.NewClusterInfo()
	node := createNode("node-complex", true, 4)

	// Task 1: "0,1" - Currently not fully supported, but we want to ensure no crash
	task1 := pod_info.NewTaskInfo(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "task1", UID: "task1"},
		Spec:       v1.PodSpec{NodeName: "node-complex"},
	})
	task1.GPUGroups = []string{"0,1"} // Complex string
	task1.Status = pod_status.Running
	node.PodInfos[task1.UID] = task1

	// Task 2: Invalid string "abc" - Ensure no crash
	task2 := pod_info.NewTaskInfo(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "task2", UID: "task2"},
		Spec:       v1.PodSpec{NodeName: "node-complex"},
	})
	task2.GPUGroups = []string{"abc"}
	task2.Status = pod_status.Running
	node.PodInfos[task2.UID] = task2

	ci.Nodes["node-complex"] = node
	mockCache := &MockCache{snapshot: ci}
	service := NewVisualizerService(mockCache)

	nodes, err := service.GetNodes()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(nodes))

	// Currently, "0,1" -> Atoi error -> Ignored.
	// "abc" -> Atoi error -> Ignored.
	// So slots should be empty. We verify this behavior as "current limitation".
	gpuNode := nodes[0]
	for _, slot := range gpuNode.GPUSlots {
		assert.Equal(t, "", slot.OccupiedBy, "Slot should be empty due to parsing limitation")
	}
}

func TestEmptyCluster(t *testing.T) {
	ci := api.NewClusterInfo()
	// No Nodes, Queues, or Jobs added

	mockCache := &MockCache{snapshot: ci}
	service := NewVisualizerService(mockCache)

	// 1. GetClusterSummary
	summary, err := service.GetClusterSummary()
	assert.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Equal(t, 0, summary.TotalNodes)
	assert.Equal(t, 0, summary.TotalQueues)
	assert.Equal(t, 0, summary.TotalGPUs)

	// 2. GetQueues
	queues, err := service.GetQueues()
	assert.NoError(t, err)
	assert.Empty(t, queues)

	// 3. GetJobs
	jobs, err := service.GetJobs("")
	assert.NoError(t, err)
	assert.Empty(t, jobs)

	// 4. GetNodes
	nodes, err := service.GetNodes()
	assert.NoError(t, err)
	assert.Empty(t, nodes)
}

// Helper functions

func createNode(name string, ready bool, gpus int) *node_info.NodeInfo {
	status := v1.ConditionFalse
	if ready {
		status = v1.ConditionTrue
	}

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"nvidia.com/gpu.count": asString(gpus),
			},
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{Type: v1.NodeReady, Status: status},
			},
			Allocatable: v1.ResourceList{
				v1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(int64(gpus), resource.DecimalSI),
			},
			Capacity: v1.ResourceList{
				v1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(int64(gpus), resource.DecimalSI),
			},
		},
	}

	ni := node_info.NewNodeInfo(node, &StubNodePodAffinityInfo{})
	return ni
}

func createJob(name, namespace string, numTasks int) *podgroup_info.PodGroupInfo {
	pgi := podgroup_info.NewPodGroupInfo(common_info.PodGroupID(name))
	pgi.Name = name
	pgi.Namespace = namespace
	pgi.CreationTimestamp = metav1.Now()

	for i := 0; i < numTasks; i++ {
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name + "-task-" + asString(i),
				Namespace: namespace,
				UID:       types.UID("uid-" + asString(i)),
			},
			Status: v1.PodStatus{Phase: v1.PodPending},
		}
		task := pod_info.NewTaskInfo(pod)
		task.Status = pod_status.Pending
		pgi.AddTaskInfo(task)
	}
	return pgi
}

func createQueue(name, parent string, weight int) *queue_info.QueueInfo {
	return &queue_info.QueueInfo{
		UID:         common_info.QueueID(name),
		Name:        name,
		ParentQueue: common_info.QueueID(parent),
		Priority:    weight,
		Resources: queue_info.QueueQuota{
			CPU:    queue_info.ResourceQuota{Quota: 10},
			Memory: queue_info.ResourceQuota{Quota: 1024},
			GPU:    queue_info.ResourceQuota{Quota: 1},
		},
		ResourceUsage: queue_info.QueueUsage{
			v1.ResourceCPU: 0,
		},
	}
}

func asString(i int) string {
	return resource.NewQuantity(int64(i), resource.DecimalSI).String()
}
