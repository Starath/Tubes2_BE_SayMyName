package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Starath/Tubes2_BE_SayMyName/api"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	router := api.SetupRouter()

	// Bungkus router dengan middleware CORS
	corsMiddleware(router).ServeHTTP(w, r)
}

// Middleware CORS
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://tubes2-fe-say-my-name-yvmx-git-main-rafif-farras-projects.vercel.app")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Tangani langsung preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", Handler)

	fmt.Printf("Server (local dev) running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil)) // Ganti `port` dengan `nil`
}
