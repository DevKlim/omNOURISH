package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/lib/pq"
	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

func main() {
	loadCSVData()

	app := &App{}
	app.InitDB()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:[]string{"*"},
		AllowedMethods:[]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:[]string{"Accept", "Authorization", "Content-Type"},
	}))

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		status := "healthy"
		if app.DB == nil {
			status = "degraded"
		}
		w.Write([]byte(fmt.Sprintf("Status: %s. Records loaded: GM=%d, Tax=%d.", status, len(gmData), len(taxData))))
	})

	r.Get("/api/business-profiles", app.handleBusinessProfiles)
	r.Get("/api/recommend-business", app.handleRecommendBusiness)
	r.Get("/api/opportunity-map", app.handleManualOpportunityMap)
	r.Get("/api/find-best-match", app.handleFindBestMatch)

	r.Get("/api/evaluate-location", app.handleEvaluateLocation)
	r.Post("/api/evaluate-location", app.handleEvaluateLocation)

	r.Get("/api/demographics", app.handleGetDemographics)
	r.Get("/api/competitors", app.handleGetCompetitors)

	r.Post("/api/agent/chat", app.handleAgentChat)
	r.Get("/api/explore-db", app.handleExploreDB)

	r.Get("/swagger", app.handleSwaggerUI)
	r.Get("/api/swagger.json", app.handleSwaggerJSON)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	ngrokAuthToken := os.Getenv("NGROK_AUTHTOKEN")
	if ngrokAuthToken != "" {
		go func() {
			ctx := context.Background()
			tun, err := ngrok.Listen(ctx,
				config.HTTPEndpoint(),
				ngrok.WithAuthtoken(ngrokAuthToken),
			)
			if err != nil {
				log.Printf("Failed to establish ngrok tunnel: %v", err)
			} else {
				log.Printf("ngrok tunnel established at: %s", tun.URL())
				err = http.Serve(tun, r)
				if err != nil {
					log.Printf("ngrok server error: %v", err)
				}
			}
		}()
	}

	log.Printf("Starting server on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
