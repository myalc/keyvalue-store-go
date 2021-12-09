package main

import (
	"log"
	"net/http"
	"os"
)

// @title GOAPP API documentation
// @description In memory key-value store
// @version 1.0.0
// @host localhost:8080
// @BasePath /api/v1/

func main() {

	// Get port from env or default 8080, heroku sets PORT dynamically
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}

	s := NewService()
	http.HandleFunc("/", s.Handle)
	log.Printf("GOAPP listenting at :%v\r\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
