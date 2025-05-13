package handler

import (
	"net/http"
	"github.com/Starath/Tubes2_BE_SayMyName/api"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	router := api.SetupRouter()
	router.ServeHTTP(w, r)
}

/*
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", Handler)

	fmt.Printf("Server (local dev) running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, port))
}
*/