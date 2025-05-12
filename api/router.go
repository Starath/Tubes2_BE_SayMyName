package api

import (
	"net/http"
	"github.com/Starath/Tubes2_BE_SayMyName/api/handlers"
)

func SetupRouter() *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("/api/pathfinding/bfs", handlers.BFSPathfindingHandler)
	router.HandleFunc("/api/pathfinding/dfs-single", handlers.DFSPathfindingHandler)
	router.HandleFunc("/api/pathfinding/dfs-multiple", handlers.DFSMultiplePathfindingHandler)

	return router
}