/*
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/

package visualizer

import (
	"encoding/json"
	"net/http"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/log"
)

type VisualizerHandler struct {
	service VisualizerService
}

func NewVisualizerHandler(service VisualizerService) *VisualizerHandler {
	return &VisualizerHandler{
		service: service,
	}
}

func (h *VisualizerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/visualizer/summary", h.handleClusterSummary)
	mux.HandleFunc("/api/v1/visualizer/queues", h.handleQueues)
	mux.HandleFunc("/api/v1/visualizer/jobs", h.handleJobs)
	mux.HandleFunc("/api/v1/visualizer/nodes", h.handleNodes)
}

func (h *VisualizerHandler) handleClusterSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summary, err := h.service.GetClusterSummary()
	if err != nil {
		log.InfraLogger.Errorf("Failed to get cluster summary: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, summary)
}

func (h *VisualizerHandler) handleQueues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	queues, err := h.service.GetQueues()
	if err != nil {
		log.InfraLogger.Errorf("Failed to get queues: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, queues)
}

func (h *VisualizerHandler) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	jobs, err := h.service.GetJobs(namespace)
	if err != nil {
		log.InfraLogger.Errorf("Failed to get jobs: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, jobs)
}

func (h *VisualizerHandler) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodes, err := h.service.GetNodes()
	if err != nil {
		log.InfraLogger.Errorf("Failed to get nodes: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, nodes)
}

func (h *VisualizerHandler) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// Add CORS headers if necessary (optional for now, but good for local dev)
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.InfraLogger.Errorf("Failed to encode JSON response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
