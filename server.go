package tubes2besaymyname

import (
	// "encoding/json" // Keep for potential future use if needed, Gin handles marshaling now
	"log"
	"net/http" // Keep for status constants like http.StatusOK
	"time"

	"github.com/gin-gonic/gin" // Import Gin
)

// Global variable to hold the loaded graph data
var alchemyGraph *BiGraphAlchemy

// Define a structure for the search results (same as before)
type SearchResult struct {
	RecipeTree     interface{} `json:"recipeTree"`
	TimeTakenMs    float64     `json:"timeTakenMs"`
	NodesVisited   int         `json:"nodesVisited"`
	Found          bool        `json:"found"`
	TargetElement  string      `json:"targetElement"`
	Algorithm      string      `json:"algorithm"`
	SearchType     string      `json:"searchType"`
}

// Mock data structure for the recipe tree (same as before)
type MockRecipeNode struct {
	Name     string            `json:"name"`
	Children []*MockRecipeNode `json:"children,omitempty"`
}

// Simple CORS Middleware for Gin
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Allow requests from your Next.js frontend development server
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000") 
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent) // Use 204 No Content for OPTIONS preflight
			return
		}

		c.Next()
	}
}

// searchHandlerGin handles requests to the /search endpoint using Gin context
func searchHandlerGin(c *gin.Context) {
	// --- Parameter Parsing using Gin context ---
	targetElement := c.Query("target")
	algorithm := c.Query("algorithm") // e.g., "bfs" or "dfs"
	searchType := c.Query("searchType") // e.g., "shortest" or "multiple"
	// maxPathsStr := c.Query("maxPaths") // Parse later

	// Basic validation
	if targetElement == "" || algorithm == "" || searchType == "" {
		log.Printf("[WARN] Bad request: Missing parameters. Target=%s, Algo=%s, Type=%s", targetElement, algorithm, searchType)
		// Return JSON error response using Gin helpers
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Missing required parameters: target, algorithm, searchType"})
		return
	}

	log.Printf("[INFO] Received search request: Target=%s, Algorithm=%s, Type=%s\n", targetElement, algorithm, searchType)

	// --- Mock Search Logic (Phase 1 - Same as before) ---
	var mockResult SearchResult
	startTime := time.Now()

	if searchType == "shortest" {
		if targetElement == "Mud" {
			mockResult = SearchResult{
				RecipeTree: &MockRecipeNode{
					Name: "Mud",
					Children: []*MockRecipeNode{
						{Name: "Water"},
						{Name: "Earth"},
					},
				},
				TimeTakenMs:  float64(time.Since(startTime).Microseconds()) / 1000.0,
				NodesVisited: 5,
				Found:        true,
				TargetElement: targetElement,
				Algorithm:    algorithm,
				SearchType:   searchType,
			}
		} else if targetElement == "Brick" {
			mockResult = SearchResult{
				RecipeTree: &MockRecipeNode{
					Name: "Brick",
					Children: []*MockRecipeNode{
						{
							Name: "Mud",
							Children: []*MockRecipeNode{
								{Name: "Water"},
								{Name: "Earth"},
							},
						},
						{Name: "Fire"},
					},
				},
				TimeTakenMs:  float64(time.Since(startTime).Microseconds()) / 1000.0,
				NodesVisited: 15,
				Found:        true,
				TargetElement: targetElement,
				Algorithm:    algorithm,
				SearchType:   searchType,
			}
		} else {
			mockResult = SearchResult{
				RecipeTree:     nil,
				TimeTakenMs:    float64(time.Since(startTime).Microseconds()) / 1000.0,
				NodesVisited:   2,
				Found:          false,
				TargetElement:  targetElement,
				Algorithm:      algorithm,
				SearchType:     searchType,
			}
		}
	} else if searchType == "multiple" {
		// TODO: Implement mock or real logic for multiple paths later
		mockResult = SearchResult{
			RecipeTree:     nil, 
			TimeTakenMs:    float64(time.Since(startTime).Microseconds()) / 1000.0,
			NodesVisited:   3, 
			Found:          false, 
			TargetElement:  targetElement,
			Algorithm:      algorithm,
			SearchType:     searchType,
		}
	} else {
		log.Printf("[WARN] Invalid searchType: %s", searchType)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid searchType parameter"})
		return
	}

	// --- Response using Gin context ---
	// Gin handles setting Content-Type to application/json
	// Use c.IndentedJSON for pretty-printed output during development
	c.IndentedJSON(http.StatusOK, mockResult) 
	log.Printf("[INFO] Sent response for Target=%s\n", targetElement)
}

// Main function to start the Gin server
func StartServer() { // Renamed from main to be callable if needed
	// Load the graph data when the server starts
	var err error
	graphPath := "elements.json" // Adjust path if needed
	alchemyGraph, err = LoadBiGraph(graphPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to load graph data from '%s': %v\n", graphPath, err)
	}
	log.Printf("[INFO] Alchemy graph loaded successfully from %s.\n", graphPath)

	// Initialize Gin router with default middleware (logger, recovery)
	router := gin.Default()

	// Use the CORS middleware
	router.Use(CORSMiddleware())

	// Register the handler function for the GET /search route
	router.GET("/search", searchHandlerGin)

	// Define the server port
	port := ":8080" // Default Gin port is 8080
	log.Printf("[INFO] Starting Gin server on port %s\n", port)

	// Start the Gin server
	err = router.Run(port)
	if err != nil {
		log.Fatalf("[FATAL] Failed to start Gin server: %v\n", err)
	}
}

// If you want this file to be the main executable, add this:
// func main() {
// 	StartServer()
// }