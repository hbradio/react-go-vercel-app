package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/rs/cors"

	handler "react-go-vercel-app/api"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env vars")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", handler.HealthHandler)
	mux.HandleFunc("/api/user", handler.UserHandler)
	mux.HandleFunc("/api/create-checkout", handler.CreateCheckoutHandler)
	mux.HandleFunc("/api/webhook", handler.WebhookHandler)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5179"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8089"
	}

	fmt.Printf("API server listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, c.Handler(mux)))
}
