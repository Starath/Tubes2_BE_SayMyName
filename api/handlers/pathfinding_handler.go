// project/be/api/handlers/pathfinding_handler.go
package handlers

import (
	"encoding/json"
	"net/http"

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
}

type DFSRequest struct {
	TargetElementName string `json:"targetElementName"`
}

type DFSResponse struct {
	Results *pathfinding.Result `json:"results"`
}

// DFSPathfindingHandler handles the DFS pathfinding API request
func DFSPathfindingHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*") // Sebaiknya ganti dengan domain frontend Anda
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Ensure method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req DFSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create graph
	graph, err := loadrecipes.LoadBiGraph("elements.json")
	if err != nil {
		respondWithError(w, "Failed to load graph", http.StatusInternalServerError)
		return
	}

	// Call DFS function
	result, err := dfs.DFSFindPathString(graph, req.TargetElementName)
	if err != nil {
		respondWithError(w, "Failed to find paths: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DFSResponse{
		Results: result,
	})
}

// BFSPathfindingHandler handles the BFS pathfinding API request
func BFSPathfindingHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*") // Sebaiknya ganti dengan domain frontend Anda
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Ensure method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req BFSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create graph
	graph, err := loadrecipes.LoadBiGraph("elements.json")
	if err != nil {
		respondWithError(w, "Failed to load graph", http.StatusInternalServerError)
		return
	}

	// Call BFS function
	result, err := bfs.BFSFindXDifferentPathsBackward(graph, req.TargetElementName, req.MaxPaths)
	if err != nil {
		respondWithError(w, "Failed to find paths: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BFSResponse{
		Results: result,
		Error:   "",
	})
}

func respondWithError(w http.ResponseWriter, errorMsg string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(BFSResponse{
		Error: errorMsg,
	})
}