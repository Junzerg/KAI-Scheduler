/*
Copyright 2026 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/

package visualizer

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	vizinfo "github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/visualizer_info"
	"github.com/stretchr/testify/assert"
)

// mockVisualizerService is a lightweight mock implementing VisualizerService
// used for HTTP handler tests.
type mockVisualizerService struct {
	clusterSummary *vizinfo.ClusterSummary
	queues         []*vizinfo.QueueView
	jobs           []*vizinfo.JobView
	nodes          []*vizinfo.NodeView

	errClusterSummary error
	errQueues         error
	errJobs           error
	errNodes          error

	lastNamespace string
}

func (m *mockVisualizerService) GetClusterSummary() (*vizinfo.ClusterSummary, error) {
	if m.errClusterSummary != nil {
		return nil, m.errClusterSummary
	}
	return m.clusterSummary, nil
}

func (m *mockVisualizerService) GetQueues() ([]*vizinfo.QueueView, error) {
	if m.errQueues != nil {
		return nil, m.errQueues
	}
	return m.queues, nil
}

func (m *mockVisualizerService) GetJobs(namespace string) ([]*vizinfo.JobView, error) {
	m.lastNamespace = namespace
	if m.errJobs != nil {
		return nil, m.errJobs
	}
	return m.jobs, nil
}

func (m *mockVisualizerService) GetNodes() ([]*vizinfo.NodeView, error) {
	if m.errNodes != nil {
		return nil, m.errNodes
	}
	return m.nodes, nil
}

// newDefaultMockService returns a mock service populated with minimal but valid data.
func newDefaultMockService() *mockVisualizerService {
	return &mockVisualizerService{
		clusterSummary: &vizinfo.ClusterSummary{
			TotalNodes:    2,
			HealthyNodes:  1,
			TotalGPUs:     4,
			AllocatedGPUs: 2,
			TotalQueues:   1,
			JobCounts: map[string]int{
				"Pending": 1,
			},
		},
		queues: []*vizinfo.QueueView{
			{
				Name:   "root",
				Parent: "",
				Weight: 100,
			},
		},
		jobs: []*vizinfo.JobView{
			{
				UID:        "job-uid-1",
				Name:       "job-1",
				Namespace:  "default",
				Queue:      "root",
				Status:     "Pending",
				CreateTime: time.Now(),
				Tasks: []*vizinfo.TaskView{
					{
						Name:     "job-1-task-0",
						Status:   "Pending",
						NodeName: "",
					},
				},
			},
		},
		nodes: []*vizinfo.NodeView{
			{
				Name:   "node-1",
				Status: "Ready",
			},
		},
	}
}

func setupTestServer(t *testing.T, svc VisualizerService) *httptest.Server {
	t.Helper()

	handler := NewVisualizerHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return httptest.NewServer(mux)
}

// --- Method validation tests ---

func TestVisualizerHandler_MethodNotAllowed(t *testing.T) {
	svc := newDefaultMockService()
	server := setupTestServer(t, svc)
	defer server.Close()

	endpoints := []string{
		"/api/v1/visualizer/summary",
		"/api/v1/visualizer/queues",
		"/api/v1/visualizer/jobs",
		"/api/v1/visualizer/nodes",
	}

	for _, ep := range endpoints {
		req, err := http.NewRequest(http.MethodPost, server.URL+ep, nil)
		assert.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, "endpoint=%s", ep)
		_ = resp.Body.Close()
	}
}

// --- Header behavior tests ---

func TestVisualizerHandler_SetsJSONAndCORSHeaders(t *testing.T) {
	svc := newDefaultMockService()
	server := setupTestServer(t, svc)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/visualizer/summary")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
}

// --- Happy path tests ---

func TestVisualizerHandler_SummaryOK(t *testing.T) {
	svc := newDefaultMockService()
	server := setupTestServer(t, svc)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/visualizer/summary")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestVisualizerHandler_QueuesOK(t *testing.T) {
	svc := newDefaultMockService()
	server := setupTestServer(t, svc)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/visualizer/queues")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestVisualizerHandler_JobsNamespaceFilter(t *testing.T) {
	mockSvc := newDefaultMockService()
	server := setupTestServer(t, mockSvc)
	defer server.Close()

	// 1) without namespace
	resp, err := http.Get(server.URL + "/api/v1/visualizer/jobs")
	assert.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, "", mockSvc.lastNamespace)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// 2) with namespace
	resp, err = http.Get(server.URL + "/api/v1/visualizer/jobs?namespace=ns-a")
	assert.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, "ns-a", mockSvc.lastNamespace)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestVisualizerHandler_NodesOK(t *testing.T) {
	svc := newDefaultMockService()
	server := setupTestServer(t, svc)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/visualizer/nodes")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// --- Error path tests ---

func TestVisualizerHandler_SummaryServiceError(t *testing.T) {
	mockSvc := &mockVisualizerService{
		errClusterSummary: errors.New("summary failed"),
	}
	server := setupTestServer(t, mockSvc)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/visualizer/summary")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestVisualizerHandler_QueuesServiceError(t *testing.T) {
	mockSvc := &mockVisualizerService{
		errQueues: errors.New("queues failed"),
	}
	server := setupTestServer(t, mockSvc)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/visualizer/queues")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestVisualizerHandler_JobsServiceError(t *testing.T) {
	mockSvc := &mockVisualizerService{
		errJobs: errors.New("jobs failed"),
	}
	server := setupTestServer(t, mockSvc)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/visualizer/jobs")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestVisualizerHandler_NodesServiceError(t *testing.T) {
	mockSvc := &mockVisualizerService{
		errNodes: errors.New("nodes failed"),
	}
	server := setupTestServer(t, mockSvc)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/visualizer/nodes")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

