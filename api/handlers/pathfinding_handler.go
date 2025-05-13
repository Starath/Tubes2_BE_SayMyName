package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Starath/Tubes2_BE_SayMyName/loadrecipes"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding/bfs"
	"github.com/Starath/Tubes2_BE_SayMyName/pathfinding/dfs"
)

type BFSRequest struct {
	TargetElementName string `json:"targetElementName"`
	MaxPaths          int    `json:"maxPaths"`
}

type BFSResponse struct {
	Results *pathfinding.MultipleResult `json:"results"`
	Error   string                      `json:"error,omitempty"`
	ExecutionTime float64                  `json:"executionTimeMs"`
}

type DFSRequest struct {
	TargetElementName string `json:"targetElementName"`
	MaxPaths          int    `json:"maxPaths"`
}

type DFSSingleResponse struct {
	Results *pathfinding.Result `json:"results"`
	ExecutionTime float64                  `json:"executionTimeMs"`
}

type DFSMultipleResponse struct {
	Results      []pathfinding.Result `json:"results"`
	NodesVisited int                  `json:"nodesVisited"`
	ExecutionTime float64                  `json:"executionTimeMs"`
}

// handling dfs single recipee
func DFSPathfindingHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Menerima request %s ke %s", r.Method, r.URL.Path)
    for name, headers := range r.Header {
        for _, h := range headers {
            log.Printf("Header: %v = %v", name, h)
        }
    }

	w.Header().Set("Access-Control-Allow-Origin", "https://tubes2-fe-say-my-name-yvmx-git-main-rafif-farras-projects.vercel.app")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	// Method harus POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DFSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	graph, err := loadrecipes.LoadBiGraph("elements_filtered.json")
	if err != nil {
		respondWithError(w, "Failed to load graph", http.StatusInternalServerError)
		return
	}

	start := time.Now()
	result, err := dfs.DFSFindPathString(graph, req.TargetElementName)
	
	if err != nil {
		respondWithError(w, "Failed to find paths: "+err.Error(), http.StatusInternalServerError)
		return
	}

	executionTime := time.Since(start).Seconds() * 1000

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DFSSingleResponse{
		Results: result,
		ExecutionTime: float64(executionTime), 
	})
}
// handling dfs multi recipis
func DFSMultiplePathfindingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "https://tubes2-fe-say-my-name-yvmx-git-main-rafif-farras-projects.vercel.app")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DFSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	graph, err := loadrecipes.LoadBiGraph("elements_filtered.json")
	if err != nil {
		respondWithError(w, "Failed to load graph", http.StatusInternalServerError)
		return
	}

	start := time.Now()
	result, nodesVisited, err := dfs.DFSFindMultiplePathsString(graph, req.TargetElementName, req.MaxPaths)
	executionTime := time.Since(start).Seconds() * 1000
	if err != nil {
		respondWithError(w, "Failed to find paths: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DFSMultipleResponse{
		Results:      result.Results,
		NodesVisited: nodesVisited,
		ExecutionTime: float64(executionTime),
	})
}

// bfs handler
func BFSPathfindingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "https://tubes2-fe-say-my-name-yvmx-git-main-rafif-farras-projects.vercel.app")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BFSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	graph, err := loadrecipes.LoadBiGraph("elements_filtered.json")
	if err != nil {
		respondWithError(w, "Failed to load graph", http.StatusInternalServerError)
		return
	}

	start := time.Now()
	result, err := bfs.BFSFindXDifferentPathsBackward_ProxyParallel(graph, req.TargetElementName, req.MaxPaths)
	executionTime := time.Since(start).Seconds() * 1000

	if err != nil {
		respondWithError(w, "Failed to find paths: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BFSResponse{
		Results: result,
		Error:   "",
		ExecutionTime: float64(executionTime),
	})
}

func respondWithError(w http.ResponseWriter, errorMsg string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(BFSResponse{
		Error: errorMsg,
	})
}