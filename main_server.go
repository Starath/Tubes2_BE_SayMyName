package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Starath/Tubes2_BE_SayMyName/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := api.SetupRouter()
	
	fmt.Printf("Server running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}