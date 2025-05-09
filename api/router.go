// project/be/api/router.go
package api

import (
	"net/http"

	"github.com/Starath/Tubes2_BE_SayMyName/api/handlers"
)

func SetupRouter() *http.ServeMux {
	router := http.NewServeMux()
	
	// Register BFS pathfinding endpoint
	router.HandleFunc("/api/pathfinding/bfs", handlers.BFSPathfindingHandler)
	// Register DFS pathfinding endpoint
	router.HandleFunc("/api/pathfinding/dfs", handlers.DFSPathfindingHandler)
	
	return router
}