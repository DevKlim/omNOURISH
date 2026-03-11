package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/lib/pq"
)

type App struct {
	DB *sql.DB
}

type ChatRequest struct {
	Message  string `json:"message"`
	ApiKey   string `json:"apiKey"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
	BaseUrl  string `json:"baseUrl"`
}

type MapPoint struct {
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Score int     `json:"score"`
	Name  string  `json:"name"`
	Type  string  `json:"type"`
}

type MapConfigResponse struct {
	Reply           string     `json:"reply"`
	ActiveLayers    []string   `json:"activeLayers"`
	FocusArea       string     `json:"focusArea"`
	ZoomLevel       int        `json:"zoomLevel"`
	MapPoints       []MapPoint `json:"mapPoints"`
	ActiveWorkspace string     `json:"activeWorkspace"`
}

// OpenAI API Structs
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message OpenAIMessage `json:"message"`
	} `json:"choices"`
}

// Gemini API Structs
type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiRequest struct {
	SystemInstruction *GeminiContent  `json:"systemInstruction,omitempty"`
	Contents          []GeminiContent `json:"contents"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type BusinessConfig struct {
	NAICS                string  `json:"naics"`
	Name                 string  `json:"name"`
	TrafficWeight        float64 `json:"trafficWeight"`
	CompPenaltyWeight    float64 `json:"compPenaltyWeight"`
	SuppBonusWeight      float64 `json:"suppBonusWeight"`
	CostPenaltyWeight    float64 `json:"costPenaltyWeight"`
	RatingBonusWeight    float64 `json:"ratingBonusWeight"`
	FoodDesertBonus      float64 `json:"foodDesertBonus"`
	GentrificationWeight float64 `json:"gentrificationWeight"`
}

var BusinessProfiles = []BusinessConfig{
	{NAICS: "445", Name: "Food and Beverage Stores (Grocery)", TrafficWeight: 1.5, CompPenaltyWeight: 10.0, SuppBonusWeight: 2.0, CostPenaltyWeight: 5.0, RatingBonusWeight: 0.0, FoodDesertBonus: 25.0, GentrificationWeight: -2.0},
	{NAICS: "722", Name: "Food Services and Drinking Places", TrafficWeight: 2.0, CompPenaltyWeight: 8.0, SuppBonusWeight: 1.5, CostPenaltyWeight: 8.0, RatingBonusWeight: 15.0, FoodDesertBonus: 0.0, GentrificationWeight: 5.0},
	{NAICS: "454", Name: "Nonstore Retailers (Food Trucks/Stands)", TrafficWeight: 3.0, CompPenaltyWeight: 5.0, SuppBonusWeight: 3.0, CostPenaltyWeight: 2.0, RatingBonusWeight: 10.0, FoodDesertBonus: 10.0, GentrificationWeight: 2.0},
	{NAICS: "311811", Name: "Retail Bakeries", TrafficWeight: 2.0, CompPenaltyWeight: 6.0, SuppBonusWeight: 2.0, CostPenaltyWeight: 6.0, RatingBonusWeight: 12.0, FoodDesertBonus: 5.0, GentrificationWeight: 4.0},
	{NAICS: "445110", Name: "Supermarkets (Healthy Grocery)", TrafficWeight: 1.5, CompPenaltyWeight: 12.0, SuppBonusWeight: 2.5, CostPenaltyWeight: 7.0, RatingBonusWeight: 5.0, FoodDesertBonus: 30.0, GentrificationWeight: -5.0},
}

type ScoreConfig struct {
	UseFootTraffic      bool
	UseCosts            bool
	UseCompetitors      bool
	AllowApproximations bool
	BusinessType        string
	ComputationMethod   string

	// Dynamic Weights
	TrafficWeight        float64
	CompPenaltyWeight    float64
	SuppBonusWeight      float64
	CostPenaltyWeight    float64
	RatingBonusWeight    float64
	FoodDesertBonus      float64
	GentrificationWeight float64
}

// Structs for Enterprise Planning Engine (EPE) JSON response
type DetailedCosts struct {
	EstimatedRent      *float64 `json:"estimatedRent"`
	EstimatedUtilities *float64 `json:"estimatedUtilities"`
	LaborCostPct       *float64 `json:"laborCostPct"`
	Source             string   `json:"source"`
}

type Demographics struct {
	IncomeLevel             *float64 `json:"incomeLevel"`
	GentrificationIndicator *float64 `json:"gentrificationIndicator"`
	TargetPopulationGrowth  *float64 `json:"targetPopulationGrowth"`
	FoodDesertStatus        bool     `json:"foodDesertStatus"`
	LowIncomeLowAccess      bool     `json:"lowIncomeLowAccess"`
	FoodInsecurityRate      *float64 `json:"foodInsecurityRate"`
	Source                  string   `json:"source"`
}

type LocationEvalResponse struct {
	Lat                         float64       `json:"lat"`
	Lng                         float64       `json:"lng"`
	OpportunityScore            float64       `json:"opportunityScore"`
	FootTraffic                 *int          `json:"footTraffic"`
	FootTrafficSource           string        `json:"footTrafficSource"`
	IsApproximated              bool          `json:"isApproximated"`
	NearbyCompetitors           int           `json:"nearbyCompetitors"`
	SupportiveBusinesses        int           `json:"supportiveBusinesses"`
	Demographics                Demographics  `json:"demographics"`
	OperatingCosts              DetailedCosts `json:"operatingCosts"`
	DemographicProfile          string        `json:"demographicProfile"`
	ReviewCount                 int           `json:"reviewCount"`
	StatsExtra                  string        `json:"statsExtra"`
	CalcLog                     string        `json:"calcLog"`
	CitywideActiveTaxCompetitor int           `json:"citywideActiveTaxCompetitor"`
}

type GMRecord struct {
	Title        string
	Lat          float64
	Lng          float64
	ReviewCount  int
	Rating       float64
	Category     string
	PriceRange   int
	Reservations bool
	HasPopTimes  bool
}

type TaxRecord struct {
	DBAName string
	Address string
	NAICS   string
}

type BusinessRecommendation struct {
	Profile BusinessConfig `json:"profile"`
	Score   float64        `json:"score"`
	Details string         `json:"details"`
}

var gmData []GMRecord
var taxData []TaxRecord
var calculationCache sync.Map

func getCacheKey(prefix, keyword, foodDesert, rent, popularity string, latStart, latEnd, lngStart, lngEnd float64, config ScoreConfig) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%.3f|%.3f|%.3f|%.3f|%v|%v|%v|%v|%s|%s|%.2f|%.2f|%.2f|%.2f|%.2f|%.2f|%.2f",
		prefix, keyword, foodDesert, rent, popularity, latStart, latEnd, lngStart, lngEnd,
		config.UseFootTraffic, config.UseCosts, config.UseCompetitors, config.AllowApproximations,
		config.BusinessType, config.ComputationMethod, config.TrafficWeight, config.CompPenaltyWeight,
		config.SuppBonusWeight, config.CostPenaltyWeight, config.RatingBonusWeight, config.FoodDesertBonus, config.GentrificationWeight)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

func main() {
	loadCSVData()

	app := &App{}
	app.InitDB()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}))

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		status := "healthy"
		if app.DB == nil {
			status = "degraded - private DB unreachable"
		}
		w.Write([]byte(fmt.Sprintf("Nourish PT Backend is %s. Loaded %d GM records and %d Tax records.", status, len(gmData), len(taxData))))
	})

	// Core API Endpoints
	r.Get("/api/business-profiles", app.handleBusinessProfiles)
	r.Get("/api/recommend-business", app.handleRecommendBusiness)
	r.Get("/api/opportunity-map", app.handleManualOpportunityMap)
	r.Get("/api/evaluate-location", app.handleEvaluateLocation)
	r.Post("/api/agent/chat", app.handleAgentChat)
	r.Get("/api/explore-db", app.handleExploreDB)

	// Swagger API Docs
	r.Get("/swagger", app.handleSwaggerUI)
	r.Get("/api/swagger.json", app.handleSwaggerJSON)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Starting server on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func parseCSVFiles(files []string, subDir string, processor func([]string, map[string]int)) int {
	bases := []string{"../data/", "../../data/", "./data/", "/data/", "/app/data/"}
	totalLoaded := 0

	for _, file := range files {
		loaded := false
		for _, base := range bases {
			path := filepath.Join(base, subDir, file)
			f, err := os.Open(path)
			if err == nil {
				reader := csv.NewReader(f)
				reader.LazyQuotes = true
				reader.FieldsPerRecord = -1
				records, err := reader.ReadAll()
				f.Close()

				if err == nil && len(records) > 0 {
					headers := records[0]
					idxMap := make(map[string]int)
					for i, h := range headers {
						clean := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\xef\xbb\xbf")))
						idxMap[clean] = i
					}

					for i := 1; i < len(records); i++ {
						processor(records[i], idxMap)
					}
					loaded = true
					totalLoaded += len(records) - 1
					log.Printf("Loaded %d records from %s", len(records)-1, path)
					break
				} else if err != nil {
					log.Printf("Error reading CSV format for %s: %v", path, err)
				}
			}
		}
		if !loaded {
			log.Printf("Warning: Could not find or parse %s in any checked directory", file)
		}
	}
	return totalLoaded
}

func loadCSVData() {
	gmFiles := []string{
		"lajolla.csv", "lajollashores.csv", "miramar.csv", "miramesa.csv",
		"oldtown.csv", "sandiego.csv", "sorrentovalley.csv", "universitycity.csv",
	}

	parseCSVFiles(gmFiles, "gm", func(row []string, idxMap map[string]int) {
		latIdx, okLat := idxMap["latitude"]
		lngIdx, okLng := idxMap["longitude"]
		rcIdx, okRc := idxMap["review_count"]
		rrIdx, okRr := idxMap["review_rating"]
		titleIdx, okT := idxMap["title"]
		catIdx, okC := idxMap["category"]
		priceIdx, okP := idxMap["price_range"]
		resIdx, okRes := idxMap["reservations"]
		popIdx, okPop := idxMap["popular_times"]

		if !okLat || !okLng {
			return
		}

		var lat, lng, rating float64
		var reviewCount int
		var title, category string

		if latIdx < len(row) {
			lat, _ = strconv.ParseFloat(row[latIdx], 64)
		}
		if lngIdx < len(row) {
			lng, _ = strconv.ParseFloat(row[lngIdx], 64)
		}
		if okRc && rcIdx < len(row) {
			reviewCount, _ = strconv.Atoi(row[rcIdx])
		}
		if okRr && rrIdx < len(row) {
			rating, _ = strconv.ParseFloat(row[rrIdx], 64)
		}
		if okT && titleIdx < len(row) {
			title = row[titleIdx]
		}
		if okC && catIdx < len(row) {
			category = row[catIdx]
		}

		var priceRange int
		if okP && priceIdx < len(row) {
			pStr := row[priceIdx]
			if strings.Contains(pStr, "$") {
				priceRange = strings.Count(pStr, "$")
			} else {
				p, err := strconv.Atoi(pStr)
				if err == nil {
					priceRange = p
				}
			}
		}

		reservations := false
		if okRes && resIdx < len(row) {
			if strings.ToLower(row[resIdx]) == "true" {
				reservations = true
			}
		}

		hasPop := false
		if okPop && popIdx < len(row) {
			if len(row[popIdx]) > 5 {
				hasPop = true
			}
		}

		if lat != 0 && lng != 0 {
			gmData = append(gmData, GMRecord{
				Title:        title,
				Lat:          lat,
				Lng:          lng,
				ReviewCount:  reviewCount,
				Rating:       rating,
				Category:     category,
				PriceRange:   priceRange,
				Reservations: reservations,
				HasPopTimes:  hasPop,
			})
		}
	})

	taxFiles := []string{"tr_active1.csv", "tr_active2.csv"}

	parseCSVFiles(taxFiles, "tax_listings", func(row []string, idxMap map[string]int) {
		dbaIdx, okDba := idxMap["dba name"]
		addrIdx, okAddr := idxMap["address"]
		naicsIdx, okNaics := idxMap["naics"]

		var dba, addr, naics string
		if okDba && dbaIdx < len(row) {
			dba = row[dbaIdx]
		}
		if okAddr && addrIdx < len(row) {
			addr = row[addrIdx]
		}
		if okNaics && naicsIdx < len(row) {
			naics = row[naicsIdx]
		}

		if dba != "" {
			taxData = append(taxData, TaxRecord{
				DBAName: dba,
				Address: addr,
				NAICS:   naics,
			})
		}
	})
}

func (a *App) InitDB() {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	db, err := sql.Open("postgres", connStr)
	if err == nil {
		err = db.Ping()
		if err == nil {
			log.Println("Connected to PostgreSQL Database successfully.")
			a.DB = db
			return
		}
	}
	log.Printf("Warning: Could not establish connection to SDSC database: %v", err)
}

func (a *App) getDemographicsHeuristic(lat, lng float64) (bool, float64, float64) {
	// Simple spatial heuristic for food deserts mapping (Southeast SD boundaries proxy)
	isFoodDesert := (lng > -117.15 && lng < -117.05 && lat > 32.65 && lat < 32.75) || (lng > -117.10 && lat > 32.85 && lat < 32.90)
	baseIncome := 65000.0 + (lat-32.0)*15000.0
	gentVal := 2.4 + (lng+117.0)*4.5
	return isFoodDesert, baseIncome, gentVal
}

func (a *App) calculateOpportunityScore(lat, lng float64, config ScoreConfig) (int, float64, int, int, int, float64, int, int, string) {
	compCount := 0
	suppCount := 0
	reviewSum := 0
	priceSum := 0
	priceCount := 0
	ratingSum := 0.0
	resCount := 0
	popCount := 0

	trafficScore := 0.0
	compPenalty := 0.0
	suppBonus := 0.0

	// Computation Method Adjustments
	radiusSq := 0.001 // Approx 2 miles (Standard Local Alloc)
	if config.ComputationMethod == "boutique" {
		radiusSq = 0.01 // Approx 6.5 miles (Boutique additive strategy)
	}

	for _, gm := range gmData {
		dLat := gm.Lat - lat
		dLng := gm.Lng - lng
		distSq := dLat*dLat + dLng*dLng

		if distSq < radiusSq {
			distMeters := math.Sqrt(distSq) * 111000
			weight := 1.0

			if config.ComputationMethod == "boutique" {
				if distMeters > 500 {
					weight = 500.0 / distMeters
				}
			} else {
				if distMeters > 50 {
					weight = 50.0 / distMeters
				}
			}

			catLower := strings.ToLower(gm.Category)
			isFoodRelated := strings.Contains(catLower, "restaurant") || strings.Contains(catLower, "food") || strings.Contains(catLower, "cafe") || strings.Contains(catLower, "grocery")

			isCompetitor := false
			isSupportive := false

			if isFoodRelated {
				if strings.HasPrefix(config.BusinessType, "445") {
					if strings.Contains(catLower, "grocery") || strings.Contains(catLower, "supermarket") {
						isCompetitor = true
					} else if strings.Contains(catLower, "farm") || strings.Contains(catLower, "wholesale") || strings.Contains(catLower, "restaurant") {
						isSupportive = true
					}
				} else if strings.HasPrefix(config.BusinessType, "722") {
					if strings.Contains(catLower, "restaurant") || strings.Contains(catLower, "cafe") {
						isCompetitor = true
					} else if strings.Contains(catLower, "grocery") || strings.Contains(catLower, "bar") {
						isSupportive = true
					}
				} else if strings.HasPrefix(config.BusinessType, "454") {
					if strings.Contains(catLower, "truck") || strings.Contains(catLower, "stand") {
						isCompetitor = true
					} else if strings.Contains(catLower, "park") || strings.Contains(catLower, "brewery") {
						isSupportive = true
					}
				} else {
					if strings.Contains(catLower, "restaurant") {
						isCompetitor = true
					} else {
						isSupportive = true
					}
				}
			}

			if isCompetitor {
				compCount++
				ratingSum += gm.Rating
				if gm.PriceRange > 0 {
					priceSum += gm.PriceRange
					priceCount++
				}
				if gm.Reservations {
					resCount++
				}
				compPenalty += weight * config.CompPenaltyWeight
			} else if isSupportive {
				suppCount++
				suppBonus += weight * config.SuppBonusWeight
			} else {
				reviewSum += gm.ReviewCount
				trafficScore += weight * (float64(gm.ReviewCount) / 100.0) * config.TrafficWeight
				if gm.HasPopTimes {
					popCount++
					trafficScore += weight * 3.0 * config.TrafficWeight
				}
			}
		}
	}

	avgPrice := 0.0
	if priceCount > 0 {
		avgPrice = float64(priceSum) / float64(priceCount)
	}

	avgRating := 0.0
	if compCount > 0 {
		avgRating = ratingSum / float64(compCount)
	}

	baseScore := 45.0
	if trafficScore > 40.0 {
		trafficScore = 40.0
	}
	if compPenalty > 40.0 {
		compPenalty = 40.0
	}
	if suppBonus > 15.0 {
		suppBonus = 15.0
	}

	costPenalty := 0.0
	if avgPrice > 0 {
		costPenalty = avgPrice * config.CostPenaltyWeight
	}

	ratingBonus := 0.0
	if compCount > 0 && avgRating < 4.0 && avgRating > 0 {
		ratingBonus = (4.0 - avgRating) * config.RatingBonusWeight
	}

	isFoodDesert, _, gentVal := a.getDemographicsHeuristic(lat, lng)

	if !config.UseFootTraffic {
		trafficScore = 20.0
	}
	if !config.UseCompetitors {
		compPenalty = 0.0
		ratingBonus = 0.0
	}
	if !config.UseCosts {
		costPenalty = 0.0
	}

	finalScore := baseScore + trafficScore + suppBonus - compPenalty - costPenalty + ratingBonus

	// Apply Demographic adjustments
	if isFoodDesert {
		finalScore += config.FoodDesertBonus
	}
	finalScore += gentVal * config.GentrificationWeight

	if finalScore > 100.0 {
		finalScore = 100.0
	} else if finalScore < 0.0 {
		finalScore = 0.0
	}

	logStr := fmt.Sprintf("Base: %.1f | Traffic Add: +%.1f | Supp Bonus: +%.1f | Comp Pen: -%.1f | Cost Pen: -%.1f | Gap Bonus: +%.1f | FD Bonus: +%.1f | Gentrif Mod: %.1f",
		baseScore, trafficScore, suppBonus, compPenalty, costPenalty, ratingBonus, config.FoodDesertBonus, gentVal*config.GentrificationWeight)

	return int(finalScore), avgPrice, reviewSum, compCount, suppCount, avgRating, popCount, resCount, logStr
}

func parseFloatParam(r *http.Request, key string, defaultVal float64) float64 {
	valStr := r.URL.Query().Get(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultVal
	}
	return val
}

func (a *App) handleBusinessProfiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BusinessProfiles)
}

func (a *App) handleRecommendBusiness(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")

	lat, _ := strconv.ParseFloat(latStr, 64)
	lng, _ := strconv.ParseFloat(lngStr, 64)

	var recommendations []BusinessRecommendation

	for _, profile := range BusinessProfiles {
		config := ScoreConfig{
			UseFootTraffic:       true,
			UseCosts:             true,
			UseCompetitors:       true,
			AllowApproximations:  true,
			BusinessType:         profile.NAICS,
			ComputationMethod:    "standard",
			TrafficWeight:        profile.TrafficWeight,
			CompPenaltyWeight:    profile.CompPenaltyWeight,
			SuppBonusWeight:      profile.SuppBonusWeight,
			CostPenaltyWeight:    profile.CostPenaltyWeight,
			RatingBonusWeight:    profile.RatingBonusWeight,
			FoodDesertBonus:      profile.FoodDesertBonus,
			GentrificationWeight: profile.GentrificationWeight,
		}

		score, _, _, compCount, suppCount, _, _, _, logStr := a.calculateOpportunityScore(lat, lng, config)

		recommendations = append(recommendations, BusinessRecommendation{
			Profile: profile,
			Score:   float64(score),
			Details: fmt.Sprintf("Competitors: %d | Supporters: %d | Log: %s", compCount, suppCount, logStr),
		})
	}

	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recommendations)
}

func (a *App) handleEvaluateLocation(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")

	config := ScoreConfig{
		UseFootTraffic:       r.URL.Query().Get("useFootTraffic") != "false",
		UseCosts:             r.URL.Query().Get("useCosts") != "false",
		UseCompetitors:       r.URL.Query().Get("useCompetitors") != "false",
		AllowApproximations:  r.URL.Query().Get("allowApproximations") != "false",
		BusinessType:         r.URL.Query().Get("naics"),
		ComputationMethod:    r.URL.Query().Get("computationMethod"),
		TrafficWeight:        parseFloatParam(r, "trafficW", 1.0),
		CompPenaltyWeight:    parseFloatParam(r, "compW", 8.0),
		SuppBonusWeight:      parseFloatParam(r, "suppW", 1.5),
		CostPenaltyWeight:    parseFloatParam(r, "costW", 5.0),
		RatingBonusWeight:    parseFloatParam(r, "ratingW", 15.0),
		FoodDesertBonus:      parseFloatParam(r, "foodDesertW", 0.0),
		GentrificationWeight: parseFloatParam(r, "gentrificationW", 0.0),
	}

	lat, _ := strconv.ParseFloat(latStr, 64)
	lng, _ := strconv.ParseFloat(lngStr, 64)

	score, avgPrice, totalReviews, calcCompCount, calcSuppCount, avgRating, popCount, resCount, calcLog := a.calculateOpportunityScore(lat, lng, config)

	resp := LocationEvalResponse{
		Lat:                         lat,
		Lng:                         lng,
		NearbyCompetitors:           calcCompCount,
		SupportiveBusinesses:        calcSuppCount,
		OpportunityScore:            float64(score),
		ReviewCount:                 totalReviews,
		CitywideActiveTaxCompetitor: 0,
		IsApproximated:              false,
		CalcLog:                     calcLog,
	}

	resp.Demographics = Demographics{Source: "System Defaults / No DB Connection"}
	resp.OperatingCosts = DetailedCosts{Source: "SBA Base Guidelines"}

	if a.DB != nil {
		var isFoodDesert, isLILA bool
		var foodInsecRate sql.NullFloat64

		errFood := a.DB.QueryRow(`SELECT is_food_desert_usda, is_low_income_low_access, food_insecurity_rate FROM nourish_cbg_food_environment LIMIT 1`).Scan(&isFoodDesert, &isLILA, &foodInsecRate)
		if errFood == nil {
			resp.Demographics.FoodDesertStatus = isFoodDesert
			resp.Demographics.LowIncomeLowAccess = isLILA
			if foodInsecRate.Valid {
				resp.Demographics.FoodInsecurityRate = &foodInsecRate.Float64
			}
			resp.Demographics.Source = "nourish_cbg_food_environment & esri_variables"
			// Score logic relies on calculateOpportunityScore weights now. No manual static additions here.
		}

		var zoneName string
		errZone := a.DB.QueryRow(`
			SELECT zone_name
			FROM sandag_layer_zoning_base_sd_new
			ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC
			LIMIT 1
		`, lng, lat).Scan(&zoneName)
		if errZone == nil {
			resp.DemographicProfile = fmt.Sprintf("Zone Context: %s", zoneName)
		} else {
			resp.DemographicProfile = "Commercial / Mixed Zone (Interpolated)"
		}

		var laborCost float64
		err := a.DB.QueryRow(`SELECT avg(labor_cost_pct_of_revenue) FROM nourish_ref_bakery_economics`).Scan(&laborCost)
		if err == nil {
			resp.OperatingCosts.LaborCostPct = &laborCost
			resp.OperatingCosts.Source = "SBA Guidelines & Database Ref"
		} else {
			lc := 32.5
			resp.OperatingCosts.LaborCostPct = &lc
		}

		var retailVisits int
		errRetail := a.DB.QueryRow(`SELECT raw_visit_counts FROM pass_by_retail_store_foot_traffic_yelp_category LIMIT 1`).Scan(&retailVisits)
		if errRetail == nil {
			resp.FootTraffic = &retailVisits
			resp.FootTrafficSource = "Direct UCSF Proxy Counts"
		}
	} else {
		lc := 32.5
		resp.OperatingCosts.LaborCostPct = &lc
	}

	_, baseIncome, gentVal := a.getDemographicsHeuristic(lat, lng)
	resp.Demographics.IncomeLevel = &baseIncome
	resp.Demographics.GentrificationIndicator = &gentVal

	popGrow := 3.2
	resp.Demographics.TargetPopulationGrowth = &popGrow

	if resp.FootTraffic == nil {
		if config.AllowApproximations {
			approxTraffic := totalReviews * 12
			resp.FootTraffic = &approxTraffic
			resp.FootTrafficSource = "Approximated via GM Volumetrics"
			resp.IsApproximated = true
		} else {
			resp.FootTrafficSource = "Approximations Disabled (Strict Mode)"
			resp.FootTraffic = nil
		}
	}

	if avgPrice > 2.5 {
		resp.OperatingCosts.Source = "High Land Cost Matrix (SBA)"
		r := 45.0
		resp.OperatingCosts.EstimatedRent = &r
	} else if avgPrice > 0 {
		r := 25.0
		resp.OperatingCosts.EstimatedRent = &r
	} else {
		r := 18.0
		resp.OperatingCosts.EstimatedRent = &r
	}

	if resp.OperatingCosts.EstimatedRent != nil {
		resp.OperatingCosts.EstimatedUtilities = func() *float64 { v := *resp.OperatingCosts.EstimatedRent * 0.15; return &v }()
	}

	statsMsg := fmt.Sprintf("%d Area Non-Comp Reviews | Avg Comp Rating: %.1f | %d Pop. Times Points", totalReviews, avgRating, popCount)
	if resCount > 0 {
		statsMsg += fmt.Sprintf(" | %d Comp take reservations", resCount)
	}
	resp.StatsExtra = statsMsg

	if config.BusinessType != "" {
		for _, t := range taxData {
			if strings.HasPrefix(t.NAICS, config.BusinessType) {
				resp.CitywideActiveTaxCompetitor++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (a *App) getRealOpportunities(latStart, latEnd, lngStart, lngEnd float64, config ScoreConfig) ([]MapPoint, int, int, string) {
	var points []MapPoint
	dbStatus := "Not Connected"
	sqlCount := 0
	csvCount := 0

	if a.DB != nil {
		dbStatus = "Connected"
		query := `
			SELECT ST_Y(ST_Centroid(geom)), ST_X(ST_Centroid(geom)), zone_name
			FROM sandag_layer_zoning_base_sd_new
			WHERE (zone_name ILIKE '%Commercial%' OR zone_name ILIKE '%Mixed%')
			  AND ST_Y(ST_Centroid(geom)) BETWEEN $1 AND $2
			  AND ST_X(ST_Centroid(geom)) BETWEEN $3 AND $4
			LIMIT 300;
		`
		rows, err := a.DB.Query(query, latStart, latEnd, lngStart, lngEnd)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var lat, lng float64
				var name string
				if err := rows.Scan(&lat, &lng, &name); err == nil {
					score, _, _, _, _, _, _, _, _ := a.calculateOpportunityScore(lat, lng, config)
					points = append(points, MapPoint{
						Lat:   lat,
						Lng:   lng,
						Score: score,
						Name:  fmt.Sprintf("Parcel: %s", name),
						Type:  "opportunity",
					})
					sqlCount++
				}
			}
		} else {
			dbStatus = fmt.Sprintf("Query Error: %v", err)
		}
	}

	if len(points) == 0 {
		for _, gm := range gmData {
			if gm.Lat >= latStart && gm.Lat <= latEnd && gm.Lng >= lngStart && gm.Lng <= lngEnd {
				if gm.Rating > 0 && gm.Rating < 4.0 && gm.ReviewCount > 10 {
					score, _, _, _, _, _, _, _, _ := a.calculateOpportunityScore(gm.Lat, gm.Lng, config)
					points = append(points, MapPoint{
						Lat:   gm.Lat,
						Lng:   gm.Lng,
						Score: score,
						Name:  "Acquisition Target: " + gm.Title,
						Type:  "opportunity",
					})
					csvCount++
				}
			}
		}
	}

	return points, sqlCount, csvCount, dbStatus
}

func (a *App) getDynamicCompetitors(latStart, latEnd, lngStart, lngEnd float64, config ScoreConfig) []MapPoint {
	var points []MapPoint
	count := 0
	for _, gm := range gmData {
		if gm.Lat >= latStart && gm.Lat <= latEnd && gm.Lng >= lngStart && gm.Lng <= lngEnd {
			catLower := strings.ToLower(gm.Category)

			isCompetitor := false
			if strings.HasPrefix(config.BusinessType, "445") && (strings.Contains(catLower, "grocery") || strings.Contains(catLower, "supermarket")) {
				isCompetitor = true
			} else if strings.HasPrefix(config.BusinessType, "722") && (strings.Contains(catLower, "restaurant") || strings.Contains(catLower, "cafe")) {
				isCompetitor = true
			} else if strings.HasPrefix(config.BusinessType, "454") && (strings.Contains(catLower, "truck") || strings.Contains(catLower, "stand")) {
				isCompetitor = true
			} else if config.BusinessType == "" && (strings.Contains(catLower, "restaurant") || strings.Contains(catLower, "food")) {
				isCompetitor = true
			}

			if isCompetitor && gm.Rating >= 3.8 {
				points = append(points, MapPoint{
					Lat:   gm.Lat,
					Lng:   gm.Lng,
					Score: 50,
					Name:  gm.Title,
					Type:  "competitor",
				})
				count++
				if count > 200 {
					break
				}
			}
		}
	}
	return points
}

func (a *App) handleManualOpportunityMap(w http.ResponseWriter, r *http.Request) {
	naics := r.URL.Query().Get("naics")
	foodDesert := r.URL.Query().Get("foodDesert")
	rent := r.URL.Query().Get("rent")
	popularity := r.URL.Query().Get("popularity")

	config := ScoreConfig{
		UseFootTraffic:       r.URL.Query().Get("useFootTraffic") != "false",
		UseCosts:             r.URL.Query().Get("useCosts") != "false",
		UseCompetitors:       r.URL.Query().Get("useCompetitors") != "false",
		AllowApproximations:  r.URL.Query().Get("allowApproximations") != "false",
		BusinessType:         naics,
		ComputationMethod:    r.URL.Query().Get("computationMethod"),
		TrafficWeight:        parseFloatParam(r, "trafficW", 1.0),
		CompPenaltyWeight:    parseFloatParam(r, "compW", 8.0),
		SuppBonusWeight:      parseFloatParam(r, "suppW", 1.5),
		CostPenaltyWeight:    parseFloatParam(r, "costW", 5.0),
		RatingBonusWeight:    parseFloatParam(r, "ratingW", 15.0),
		FoodDesertBonus:      parseFloatParam(r, "foodDesertW", 0.0),
		GentrificationWeight: parseFloatParam(r, "gentrificationW", 0.0),
	}

	nStr := r.URL.Query().Get("n")
	sStr := r.URL.Query().Get("s")
	eStr := r.URL.Query().Get("e")
	wStr := r.URL.Query().Get("w")

	nBound, _ := strconv.ParseFloat(nStr, 64)
	sBound, _ := strconv.ParseFloat(sStr, 64)
	eBound, _ := strconv.ParseFloat(eStr, 64)
	wBound, _ := strconv.ParseFloat(wStr, 64)

	if nBound == 0 && sBound == 0 {
		nBound, sBound, eBound, wBound = 32.95, 32.65, -116.95, -117.30
	}

	cacheKey := getCacheKey("map", naics, foodDesert, rent, popularity, sBound, nBound, wBound, eBound, config)
	if cachedResult, ok := calculationCache.Load(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cachedResult.([]byte))
		return
	}

	oppPoints, sqlCount, csvCount, dbStatus := a.getRealOpportunities(sBound, nBound, wBound, eBound, config)
	compPoints := a.getDynamicCompetitors(sBound, nBound, wBound, eBound, config)

	allPoints := append(compPoints, oppPoints...)

	debugInfo := map[string]interface{}{
		"bounds":           map[string]float64{"n": nBound, "s": sBound, "e": eBound, "w": wBound},
		"dbStatus":         dbStatus,
		"sqlPointsFound":   sqlCount,
		"csvFallbackFound": csvCount,
		"competitorsFound": len(compPoints),
		"totalPoints":      len(allPoints),
		"csvLoaded":        map[string]int{"gm": len(gmData), "tax": len(taxData)},
	}

	responsePayload := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"points": allPoints,
			"debug":  debugInfo,
		},
	}

	payloadBytes, _ := json.Marshal(responsePayload)
	calculationCache.Store(cacheKey, payloadBytes)

	w.Header().Set("Content-Type", "application/json")
	w.Write(payloadBytes)
}

func (a *App) callLLM(userMessage, apiKey, model, provider, baseUrl string) string {
	token := apiKey
	if token == "" {
		token = os.Getenv("NRP_API_TOKEN")
	}

	if token == "" || token == "your_nrp_token_here" {
		return "⚠️ **Missing API Token**: Please configure your API Token in the Agent Settings (⚙️ icon)."
	}

	systemPrompt := "You are the Nourish PT Data Agent. You help users analyze the San Diego food map. You process data like foot traffic, food deserts, and competitor radius. Give clear, data-driven advice about food business placements. You can format things in markdown."

	var req *http.Request
	client := &http.Client{}

	if provider == "Gemini" {
		if model == "" {
			model = "gemini-1.5-pro"
		}
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, token)

		geminiReq := GeminiRequest{
			SystemInstruction: &GeminiContent{
				Parts: []GeminiPart{{Text: systemPrompt}},
			},
			Contents: []GeminiContent{
				{Role: "user", Parts: []GeminiPart{{Text: userMessage}}},
			},
		}

		jsonData, _ := json.Marshal(geminiReq)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("Error connecting to Gemini: %v", err)
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		var geminiResp GeminiResponse
		if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
			return "Failed to parse Gemini response format."
		}

		if len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
			return geminiResp.Candidates[0].Content.Parts[0].Text
		}
		return "Gemini returned an empty response."

	} else {
		if model == "" {
			model = "gpt-oss"
		}

		url := baseUrl
		if url == "" {
			if provider == "NRP" {
				url = "https://ellm.nrp-nautilus.io/v1/chat/completions"
			} else {
				url = "https://api.openai.com/v1/chat/completions"
			}
		}

		openAiReq := OpenAIRequest{
			Model: model,
			Messages: []OpenAIMessage{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userMessage},
			},
		}

		jsonData, _ := json.Marshal(openAiReq)
		req, _ = http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Sprintf("Error connecting to LLM endpoint: %v", err)
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Sprintf("API returned status %d. Verify your key and model. Response: %s", resp.StatusCode, string(bodyBytes))
		}

		var openAIResp OpenAIResponse
		if err := json.Unmarshal(bodyBytes, &openAIResp); err != nil {
			return "Failed to parse LLM response format."
		}

		if len(openAIResp.Choices) > 0 {
			return openAIResp.Choices[0].Message.Content
		}
		return "LLM returned an empty response."
	}
}

func (a *App) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	latStart, latEnd := 32.65, 32.95
	lngStart, lngEnd := -117.30, -116.95

	replyText := a.callLLM(req.Message, req.ApiKey, req.Model, req.Provider, req.BaseUrl)
	workspace := "LLM Analyzed View"

	config := ScoreConfig{
		UseFootTraffic:       true,
		UseCosts:             true,
		UseCompetitors:       true,
		AllowApproximations:  true,
		ComputationMethod:    "standard",
		TrafficWeight:        1.0,
		CompPenaltyWeight:    8.0,
		SuppBonusWeight:      1.5,
		CostPenaltyWeight:    5.0,
		RatingBonusWeight:    15.0,
		FoodDesertBonus:      0.0,
		GentrificationWeight: 0.0,
	}
	oppPoints, _, _, _ := a.getRealOpportunities(latStart, latEnd, lngStart, lngEnd, config)
	compPoints := a.getDynamicCompetitors(latStart, latEnd, lngStart, lngEnd, config)
	allPoints := append(compPoints, oppPoints...)

	resp := MapConfigResponse{
		Reply:           replyText,
		ActiveLayers:    []string{fmt.Sprintf("A2A Constraints: %s", workspace), "Competitor Footprint"},
		FocusArea:       "San Diego",
		ZoomLevel:       11,
		MapPoints:       allPoints,
		ActiveWorkspace: workspace,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (a *App) handleExploreDB(w http.ResponseWriter, r *http.Request) {
	tableName := r.URL.Query().Get("table")
	if tableName == "" {
		tableName = "nourish_cbg_food_environment"
	}

	if a.DB == nil {
		http.Error(w, `{"error": "Database not connected"}`, http.StatusInternalServerError)
		return
	}

	query := `SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1`
	rows, err := a.DB.Query(query, tableName)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var columns []map[string]string
	for rows.Next() {
		var colName, dataType string
		if err := rows.Scan(&colName, &dataType); err == nil {
			columns = append(columns, map[string]string{"column": colName, "type": dataType})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"table":   tableName,
		"columns": columns,
	})
}

func (a *App) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Swagger UI - Nourish PT API</title>
  <link rel="stylesheet" type="text/css" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/4.15.5/swagger-ui.css" >
  <style>
    body { margin: 0; padding: 0; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/4.15.5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: "/api/swagger.json",
        dom_id: '#swagger-ui',
      });
    }
  </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (a *App) handleSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	spec := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Nourish PT Opportunity API",
    "description": "API for querying food business opportunities, NAICS modeling, and dynamic map calculations.",
    "version": "1.0.0"
  },
  "paths": {
    "/api/evaluate-location": {
      "get": {
        "summary": "Evaluate a specific location's business viability",
        "parameters":[
          {"name": "lat", "in": "query", "required": true, "schema": {"type": "number"}},
          {"name": "lng", "in": "query", "required": true, "schema": {"type": "number"}},
          {"name": "naics", "in": "query", "schema": {"type": "string"}},
          {"name": "allowApproximations", "in": "query", "schema": {"type": "boolean"}},
          {"name": "computationMethod", "in": "query", "schema": {"type": "string", "enum": ["standard", "boutique"]}}
        ],
        "responses": {
          "200": { "description": "Returns LocationEvalResponse JSON containing operating costs, demographics, and competitive density" }
        }
      }
    },
    "/api/opportunity-map": {
      "get": {
        "summary": "Fetch opportunity score map nodes (Find best locations for a given Business Type)",
        "parameters":[
          {"name": "n", "in": "query", "schema": {"type": "number"}},
          {"name": "s", "in": "query", "schema": {"type": "number"}},
          {"name": "e", "in": "query", "schema": {"type": "number"}},
          {"name": "w", "in": "query", "schema": {"type": "number"}},
          {"name": "naics", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {
          "200": { "description": "Returns points formatted for canvas/WebGL heatmap interpolation" }
        }
      }
    },
    "/api/recommend-business": {
      "get": {
        "summary": "Recommend best business types for a given location",
        "parameters":[
          {"name": "lat", "in": "query", "required": true, "schema": {"type": "number"}},
          {"name": "lng", "in": "query", "required": true, "schema": {"type": "number"}}
        ],
        "responses": {
          "200": { "description": "Returns a sorted array of business recommendations based on the opportunity score framework" }
        }
      }
    },
    "/api/business-profiles": {
      "get": {
        "summary": "List available configured business profiles and their default scoring weights",
        "responses": {
          "200": { "description": "Array of BusinessConfig objects" }
        }
      }
    }
  }
}`
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(spec))
}
