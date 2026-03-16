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
	"net/url"
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
	{NAICS: "311811", Name: "Retail Bakeries / Home Kitchens", TrafficWeight: 2.0, CompPenaltyWeight: 6.0, SuppBonusWeight: 2.0, CostPenaltyWeight: 6.0, RatingBonusWeight: 12.0, FoodDesertBonus: 5.0, GentrificationWeight: 4.0},
	{NAICS: "445110", Name: "Supermarkets (Healthy Grocery)", TrafficWeight: 1.5, CompPenaltyWeight: 12.0, SuppBonusWeight: 2.5, CostPenaltyWeight: 7.0, RatingBonusWeight: 5.0, FoodDesertBonus: 30.0, GentrificationWeight: -5.0},
}

type ScoreOverrides struct {
	FootTraffic  *int     `json:"footTraffic"`
	Rent         *float64 `json:"rent"`
	StartupCosts *float64 `json:"startupCosts"`
	LaborCostPct *float64 `json:"laborCostPct"`
	IncomeLevel  *float64 `json:"incomeLevel"`
	DaytimePop   *float64 `json:"daytimePop"`
	NighttimePop *float64 `json:"nighttimePop"`
	MarketingPct *float64 `json:"marketingPct"`
}

type ScoreConfig struct {
	UseFootTraffic      bool
	UseCosts            bool
	UseCompetitors      bool
	AllowApproximations bool
	BusinessType        string
	ComputationMethod   string
	TargetTime          string

	// Dynamic Weights
	TrafficWeight        float64
	CompPenaltyWeight    float64
	SuppBonusWeight      float64
	CostPenaltyWeight    float64
	RatingBonusWeight    float64
	FoodDesertBonus      float64
	GentrificationWeight float64

	Overrides ScoreOverrides
}

// Structs for Enterprise Planning Engine (EPE) JSON response
type DetailedCosts struct {
	EstimatedRent      *float64 `json:"estimatedRent"`
	EstimatedUtilities *float64 `json:"estimatedUtilities"`
	LaborCostPct       *float64 `json:"laborCostPct"`
	StartupCosts       *float64 `json:"startupCosts"`
	MarketingPct       *float64 `json:"marketingPct"`
	Source             string   `json:"source"`
}

type Demographics struct {
	IncomeLevel             *float64 `json:"incomeLevel"`
	GentrificationIndicator *float64 `json:"gentrificationIndicator"`
	TargetPopulationGrowth  *float64 `json:"targetPopulationGrowth"`
	FoodDesertStatus        bool     `json:"foodDesertStatus"`
	LowIncomeLowAccess      bool     `json:"lowIncomeLowAccess"`
	FoodInsecurityRate      *float64 `json:"foodInsecurityRate"`
	DaytimePopulation       *float64 `json:"daytimePopulation"`
	NighttimePopulation     *float64 `json:"nighttimePopulation"`
	Source                  string   `json:"source"`
}

type ScoreComponent struct {
	Name         string  `json:"name"`
	RawValue     float64 `json:"rawValue"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
	Impact       string  `json:"impact"` // "Positive", "Negative", "Neutral"
}

type LocationEvalResponse struct {
	Lat                         float64          `json:"lat"`
	Lng                         float64          `json:"lng"`
	ResolvedAddress             string           `json:"resolvedAddress"`
	OpportunityScore            float64          `json:"opportunityScore"`
	FootTraffic                 *int             `json:"footTraffic"`
	FootTrafficSource           string           `json:"footTrafficSource"`
	IsApproximated              bool             `json:"isApproximated"`
	NearbyCompetitors           int              `json:"nearbyCompetitors"`
	SupportiveBusinesses        int              `json:"supportiveBusinesses"`
	Demographics                Demographics     `json:"demographics"`
	OperatingCosts              DetailedCosts    `json:"operatingCosts"`
	DemographicProfile          string           `json:"demographicProfile"`
	ReviewCount                 int              `json:"reviewCount"`
	StatsExtra                  string           `json:"statsExtra"`
	CalcBreakdown               []ScoreComponent `json:"calcBreakdown"`
	CitywideActiveTaxCompetitor int              `json:"citywideActiveTaxCompetitor"`
	Assumptions                 []string         `json:"assumptions"`
}

type EvalRequest struct {
	Address             string         `json:"address"`
	Lat                 float64        `json:"lat"`
	Lng                 float64        `json:"lng"`
	UseFootTraffic      bool           `json:"useFootTraffic"`
	UseCosts            bool           `json:"useCosts"`
	UseCompetitors      bool           `json:"useCompetitors"`
	AllowApproximations bool           `json:"allowApproximations"`
	Naics               string         `json:"naics"`
	ComputationMethod   string         `json:"computationMethod"`
	TargetTime          string         `json:"targetTime"`
	TrafficW            float64        `json:"trafficW"`
	CompW               float64        `json:"compW"`
	SuppW               float64        `json:"suppW"`
	CostW               float64        `json:"costW"`
	RatingW             float64        `json:"ratingW"`
	FoodDesertW         float64        `json:"foodDesertW"`
	GentrificationW     float64        `json:"gentrificationW"`
	Overrides           ScoreOverrides `json:"overrides"`
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
	OpenHours    string
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
var naicsKeywordsCache sync.Map

func getCacheKey(prefix, keyword, foodDesert, rent, popularity string, latStart, latEnd, lngStart, lngEnd float64, config ScoreConfig) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%.3f|%.3f|%.3f|%.3f|%v|%v|%v|%v|%s|%s|%s|%.2f|%.2f|%.2f|%.2f|%.2f|%.2f|%.2f",
		prefix, keyword, foodDesert, rent, popularity, latStart, latEnd, lngStart, lngEnd,
		config.UseFootTraffic, config.UseCosts, config.UseCompetitors, config.AllowApproximations,
		config.BusinessType, config.ComputationMethod, config.TargetTime, config.TrafficWeight, config.CompPenaltyWeight,
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
	r.Post("/api/evaluate-location", app.handleEvaluateLocation)

	// New Utility Endpoints
	r.Get("/api/demographics", app.handleGetDemographics)
	r.Get("/api/competitors", app.handleGetCompetitors)

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
		ohIdx, okOh := idxMap["open_hours"]

		if !okLat || !okLng {
			return
		}

		var lat, lng, rating float64
		var reviewCount int
		var title, category, openHours string

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
		if okOh && ohIdx < len(row) {
			openHours = row[ohIdx]
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
				OpenHours:    openHours,
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
			go a.exportSchemaToFile()
			return
		}
	}
	log.Printf("Warning: Could not establish connection to SDSC database: %v", err)
}

func (a *App) exportSchemaToFile() {
	if a.DB == nil {
		return
	}
	filePath := "/data/db_schema_dump.json"
	if _, err := os.Stat(filePath); err == nil {
		return // File exists, do not re-explore
	}

	log.Println("Exporting DB schema to", filePath)

	rows, err := a.DB.Query(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'`)
	if err != nil {
		log.Println("Error fetching tables:", err)
		return
	}
	defer rows.Close()

	schemaDump := make(map[string][]map[string]string)
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			tables = append(tables, t)
		}
	}

	for _, t := range tables {
		colRows, err := a.DB.Query(`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1`, t)
		if err == nil {
			var cols []map[string]string
			for colRows.Next() {
				var c, d string
				if err := colRows.Scan(&c, &d); err == nil {
					cols = append(cols, map[string]string{"column": c, "type": d})
				}
			}
			colRows.Close()
			schemaDump[t] = cols
		}
	}

	bytes, err := json.MarshalIndent(schemaDump, "", "  ")
	if err == nil {
		os.MkdirAll("/data", 0755)
		os.WriteFile(filePath, bytes, 0644)
		log.Println("Successfully exported DB schema to", filePath)
	}
}

func geocodeAddress(address string) (float64, float64, error) {
	apiURL := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json&limit=1", url.QueryEscape(address))
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "Nourish-PT-App/1.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var result []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, err
	}

	if len(result) > 0 {
		lat, _ := strconv.ParseFloat(result[0].Lat, 64)
		lng, _ := strconv.ParseFloat(result[0].Lon, 64)
		return lat, lng, nil
	}
	return 0, 0, fmt.Errorf("Address not found")
}

func reverseGeocode(lat, lng float64) string {
	apiURL := fmt.Sprintf("https://nominatim.openstreetmap.org/reverse?lat=%f&lon=%f&format=json", lat, lng)
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Nourish-PT-App/1.0")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var res struct {
		DisplayName string `json:"display_name"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	return res.DisplayName
}

func (a *App) getNAICSKeywords(code string) []string {
	if code == "" {
		return nil
	}
	if val, ok := naicsKeywordsCache.Load(code); ok {
		return val.([]string)
	}
	if a.DB == nil {
		return nil
	}

	codeInt, err := strconv.Atoi(code)
	if err != nil {
		return nil
	}

	var keywordsStr string
	err = a.DB.QueryRow(`SELECT naics_keywords FROM "2022_NAICS_Keywords" WHERE naics_code = $1 LIMIT 1`, codeInt).Scan(&keywordsStr)
	var keywords []string
	if err == nil && keywordsStr != "" {
		parts := strings.Split(keywordsStr, ",")
		for _, p := range parts {
			clean := strings.ToLower(strings.TrimSpace(p))
			if clean != "" {
				keywords = append(keywords, clean)
			}
		}
	}
	naicsKeywordsCache.Store(code, keywords)
	return keywords
}

func isLikelyOpen(openHours string, targetTime string) bool {
	if targetTime == "Any / All Day" || targetTime == "" || openHours == "" {
		return true
	}

	ohLower := strings.ToLower(openHours)
	if strings.Contains(ohLower, "24 hours") || strings.Contains(ohLower, "open 24") {
		return true
	}

	ohNorm := strings.ReplaceAll(ohLower, ":00", "")
	ohNorm = strings.ReplaceAll(ohNorm, ":30", "")
	ohNorm = strings.ReplaceAll(ohNorm, "\u202f", " ")
	ohNorm = strings.ReplaceAll(ohNorm, "\u2013", "-")
	ohNorm = strings.ReplaceAll(ohNorm, "a.m.", "am")
	ohNorm = strings.ReplaceAll(ohNorm, "p.m.", "pm")

	isMorning := strings.Contains(targetTime, "Early Morning")
	isEvening := strings.Contains(targetTime, "Evening") || strings.Contains(targetTime, "Night")

	if isMorning {
		return strings.Contains(ohNorm, "4 am") || strings.Contains(ohNorm, "5 am") || strings.Contains(ohNorm, "6 am") || strings.Contains(ohNorm, "7 am") || strings.Contains(ohNorm, "8 am")
	}

	if isEvening {
		return strings.Contains(ohNorm, "6 pm") || strings.Contains(ohNorm, "7 pm") || strings.Contains(ohNorm, "8 pm") || strings.Contains(ohNorm, "9 pm") || strings.Contains(ohNorm, "10 pm") || strings.Contains(ohNorm, "11 pm") || strings.Contains(ohNorm, "12 am") || strings.Contains(ohNorm, "1 am") || strings.Contains(ohNorm, "2 am")
	}

	return true
}

func (a *App) fetchDemographics(lat, lng float64, avgPrice float64, config ScoreConfig) (Demographics, DetailedCosts, []string) {
	var assumptions []string

	demo := Demographics{
		FoodDesertStatus: false,
		Source:           "Database",
	}
	costs := DetailedCosts{
		Source: "Database",
	}

	if a.DB == nil {
		assumptions = append(assumptions, "Database disconnected. All demographic and cost metrics are defaulting to NULL or baseline approximations.")
	}

	// 1. Food Environment
	if a.DB != nil {
		var dbFoodDesert, dbLILA sql.NullBool
		var dbFoodInsec sql.NullFloat64
		err := a.DB.QueryRow(`
			SELECT is_food_desert_usda, is_low_income_low_access, food_insecurity_rate 
			FROM nourish_cbg_food_environment 
			ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC
			LIMIT 1
		`, lng, lat).Scan(&dbFoodDesert, &dbLILA, &dbFoodInsec)
		if err == nil {
			if dbFoodDesert.Valid {
				demo.FoodDesertStatus = dbFoodDesert.Bool
			}
			if dbLILA.Valid {
				demo.LowIncomeLowAccess = dbLILA.Bool
			}
			if dbFoodInsec.Valid {
				demo.FoodInsecurityRate = &dbFoodInsec.Float64
			}
		} else {
			assumptions = append(assumptions, "Missing USDA Food Environment data for this coordinate. Assuming Not a Food Desert.")
			demo.FoodDesertStatus = false
		}
	} else {
		assumptions = append(assumptions, "Missing USDA Food Environment data for this coordinate. Assuming Not a Food Desert.")
	}

	// 2. Demographics
	if config.Overrides.IncomeLevel != nil {
		demo.IncomeLevel = config.Overrides.IncomeLevel
	} else if a.DB != nil {
		var inc, gent, pop sql.NullFloat64
		err := a.DB.QueryRow(`
			SELECT median_income, gentrification_index, population_growth 
			FROM nourish_cbg_demographics 
			ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC
			LIMIT 1
		`, lng, lat).Scan(&inc, &gent, &pop)
		if err == nil {
			if inc.Valid {
				demo.IncomeLevel = &inc.Float64
			}
			if gent.Valid {
				demo.GentrificationIndicator = &gent.Float64
			}
			if pop.Valid {
				demo.TargetPopulationGrowth = &pop.Float64
			}
		} else {
			assumptions = append(assumptions, "Missing Demographics (Income, Gentrification, Pop Growth). Left as NULL.")
		}
	} else {
		assumptions = append(assumptions, "Missing Demographics (Income, Gentrification, Pop Growth). Left as NULL.")
	}

	// 3. Population Time
	if config.Overrides.DaytimePop != nil || config.Overrides.NighttimePop != nil {
		demo.DaytimePopulation = config.Overrides.DaytimePop
		demo.NighttimePopulation = config.Overrides.NighttimePop
	} else if a.DB != nil {
		var dp, np sql.NullFloat64
		errPopTime := a.DB.QueryRow(`
			SELECT metrics, counts 
			FROM nourish_cbg_population_time 
			ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC
			LIMIT 1
		`, lng, lat).Scan(&dp, &np)
		if errPopTime == nil {
			if dp.Valid {
				demo.DaytimePopulation = &dp.Float64
			}
			if np.Valid {
				demo.NighttimePopulation = &np.Float64
			}
		} else {
			assumptions = append(assumptions, "Missing Daytime/Nighttime Population data. Left as NULL.")
		}
	} else {
		assumptions = append(assumptions, "Missing Daytime/Nighttime Population data. Left as NULL.")
	}

	// 4. Rent Override check
	if config.Overrides.Rent != nil {
		costs.EstimatedRent = config.Overrides.Rent
		utilsDB := *config.Overrides.Rent * 0.15
		costs.EstimatedUtilities = &utilsDB
	} else if a.DB != nil {
		var rentDB sql.NullFloat64
		errRent := a.DB.QueryRow(`
			SELECT avg_rent_cost
			FROM esri_consumer_spending_data_
			ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC
			LIMIT 1
		`, lng, lat).Scan(&rentDB)
		if errRent == nil && rentDB.Valid {
			costs.EstimatedRent = &rentDB.Float64
			utilsDB := rentDB.Float64 * 0.15
			costs.EstimatedUtilities = &utilsDB
		} else {
			fallbackRent := 25.0
			if avgPrice > 2.5 {
				fallbackRent = 45.0
			}
			costs.EstimatedRent = &fallbackRent
			assumptions = append(assumptions, "Missing local rent spatial data. Used SBA proxy mapping based on competitor price tiers.")
		}
	} else {
		fallbackRent := 25.0
		costs.EstimatedRent = &fallbackRent
		assumptions = append(assumptions, "Missing local rent spatial data. Used SBA proxy mapping based on competitor price tiers.")
	}

	// 5. Labor Override check
	if config.Overrides.LaborCostPct != nil {
		costs.LaborCostPct = config.Overrides.LaborCostPct
	} else if a.DB != nil {
		var lc sql.NullFloat64
		errLab := a.DB.QueryRow(`SELECT avg(labor_cost_pct_of_revenue) FROM nourish_ref_bakery_economics`).Scan(&lc)
		if errLab == nil && lc.Valid {
			costs.LaborCostPct = &lc.Float64
		} else {
			fallbackLab := 32.5
			costs.LaborCostPct = &fallbackLab
			assumptions = append(assumptions, "Missing reference economics. Applied SBA labor benchmark (32.5%).")
		}
	} else {
		fallbackLab := 32.5
		costs.LaborCostPct = &fallbackLab
		assumptions = append(assumptions, "Missing reference economics. Applied SBA labor benchmark (32.5%).")
	}

	// 6. Startup/Marketing Overrides
	if config.Overrides.StartupCosts != nil {
		costs.StartupCosts = config.Overrides.StartupCosts
	} else {
		fallbackStartup := 150000.0
		costs.StartupCosts = &fallbackStartup
		assumptions = append(assumptions, "Placeholder applied: Startup Costs = $150,000")
	}

	if config.Overrides.MarketingPct != nil {
		costs.MarketingPct = config.Overrides.MarketingPct
	} else {
		fallbackMktg := 5.0
		costs.MarketingPct = &fallbackMktg
		assumptions = append(assumptions, "Placeholder applied: Marketing Budget = 5%")
	}

	return demo, costs, assumptions
}

func (a *App) calculateOpportunityScore(lat, lng float64, config ScoreConfig) (int, float64, int, int, int, float64, int, int, []ScoreComponent, Demographics, DetailedCosts, []string) {
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

	reducedCompCount := 0
	reducedSuppCount := 0

	radiusSq := 0.001
	if config.ComputationMethod == "boutique" {
		radiusSq = 0.01
	}

	keywords := a.getNAICSKeywords(config.BusinessType)

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

			if len(keywords) > 0 {
				for _, kw := range keywords {
					if strings.Contains(catLower, kw) {
						isCompetitor = true
						break
					}
				}
			}

			if isFoodRelated && !isCompetitor {
				if strings.HasPrefix(config.BusinessType, "445") {
					if strings.Contains(catLower, "grocery") || strings.Contains(catLower, "supermarket") {
						isCompetitor = true
					} else if strings.Contains(catLower, "farm") || strings.Contains(catLower, "wholesale") || strings.Contains(catLower, "restaurant") {
						isSupportive = true
					}
				} else if strings.HasPrefix(config.BusinessType, "722") {
					if strings.Contains(catLower, "restaurant") || strings.Contains(catLower, "cafe") {
						isCompetitor = true
					} else if strings.Contains(catLower, "grocery") || strings.Contains(catLower, "bar") || strings.Contains(catLower, "supplier") {
						isSupportive = true
					}
				} else if strings.HasPrefix(config.BusinessType, "454") || strings.HasPrefix(config.BusinessType, "311811") {
					if strings.Contains(catLower, "truck") || strings.Contains(catLower, "stand") || strings.Contains(catLower, "bakery") {
						isCompetitor = true
					} else if strings.Contains(catLower, "park") || strings.Contains(catLower, "brewery") || strings.Contains(catLower, "event") {
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

			openDuringTarget := isLikelyOpen(gm.OpenHours, config.TargetTime)

			if isCompetitor {
				if !openDuringTarget {
					weight = weight * 0.1
					reducedCompCount++
				}
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
				if !openDuringTarget {
					weight = weight * 0.5
					reducedSuppCount++
				}
				suppCount++
				suppBonus += weight * config.SuppBonusWeight
			} else {
				if !openDuringTarget {
					weight = weight * 0.5
				}
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

	demo, costs, assumptions := a.fetchDemographics(lat, lng, avgPrice, config)
	isFoodDesert := demo.FoodDesertStatus
	gentVal := 0.0
	if demo.GentrificationIndicator != nil {
		gentVal = *demo.GentrificationIndicator
	}

	distToRoadMeters := 500.0
	if a.DB != nil {
		var dist sql.NullFloat64
		err := a.DB.QueryRow(`
			SELECT ST_Distance(geom::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography)
			FROM sandag_layer_roads
			ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1
		`, lng, lat).Scan(&dist)
		if err == nil && dist.Valid {
			distToRoadMeters = dist.Float64
		} else {
			assumptions = append(assumptions, "Failed to query road proximity. Assuming 500m distance for traffic adjustments.")
		}
	} else {
		assumptions = append(assumptions, "Failed to query road proximity. Assuming 500m distance for traffic adjustments.")
	}

	isMorningCommute := strings.Contains(config.TargetTime, "Early Morning")
	isEveningCommute := strings.Contains(config.TargetTime, "Afternoon")
	isNightTime := strings.Contains(config.TargetTime, "Evening/Night")

	if distToRoadMeters < 100 {
		boost := 15.0
		if isMorningCommute || isEveningCommute {
			boost = 25.0
			assumptions = append(assumptions, "Applied high commuter traffic bonus due to major road proximity during peak hours.")
		}
		trafficScore += boost * config.TrafficWeight
	} else if distToRoadMeters < 300 {
		trafficScore += 7.5 * config.TrafficWeight
	} else if distToRoadMeters > 1000 {
		assumptions = append(assumptions, "Location is far from major roads. Applied traffic reduction penalty.")
		trafficScore -= 5.0 * config.TrafficWeight
	}

	// Apply Proxy Bootstrap Estimation (if enabled)
	if config.AllowApproximations && config.Overrides.FootTraffic == nil {
		var activePop *float64
		popLabel := "Daytime"
		if isNightTime {
			activePop = demo.NighttimePopulation
			popLabel = "Nighttime"
		} else {
			activePop = demo.DaytimePopulation
		}

		if reviewSum < 50 {
			if activePop != nil && *activePop > 0 {
				bootstrapTraffic := (*activePop / 500.0) * config.TrafficWeight
				trafficScore += bootstrapTraffic
				assumptions = append(assumptions, fmt.Sprintf("Low review count detected. Bootstrapped traffic score using local %s population based on Target Time.", popLabel))
			} else {
				assumptions = append(assumptions, fmt.Sprintf("Low review count and missing %s population. Traffic score may be artificially low.", strings.ToLower(popLabel)))
			}
		}
	} else if config.Overrides.FootTraffic != nil {
		trafficScore += float64(*config.Overrides.FootTraffic) / 100.0 * config.TrafficWeight
	}

	baseScore := 0.0
	if trafficScore > 100.0 {
		trafficScore = 100.0
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

	if isFoodDesert {
		finalScore += config.FoodDesertBonus
	}
	finalScore += gentVal * config.GentrificationWeight

	if reducedCompCount > 0 {
		assumptions = append(assumptions, fmt.Sprintf("Reduced competitive penalty for %d businesses that are confirmed closed during %s.", reducedCompCount, config.TargetTime))
	}
	if reducedSuppCount > 0 {
		assumptions = append(assumptions, fmt.Sprintf("Reduced supportive business bonus for %d locations that are closed during %s.", reducedSuppCount, config.TargetTime))
	}

	// Construct Calc Breakdown Array
	var breakdown []ScoreComponent
	breakdown = append(breakdown, ScoreComponent{"Base Score", baseScore, 1.0, baseScore, "Neutral"})

	if config.UseFootTraffic {
		rawTraffic := float64(reviewSum)
		if config.Overrides.FootTraffic != nil {
			rawTraffic = float64(*config.Overrides.FootTraffic)
		}
		breakdown = append(breakdown, ScoreComponent{"Foot Traffic", rawTraffic, config.TrafficWeight, trafficScore, "Positive"})
	}

	if config.UseCompetitors {
		breakdown = append(breakdown, ScoreComponent{"Supportive Businesses", float64(suppCount), config.SuppBonusWeight, suppBonus, "Positive"})
		breakdown = append(breakdown, ScoreComponent{"Competitor Penalty", float64(compCount), config.CompPenaltyWeight, -compPenalty, "Negative"})
		if ratingBonus > 0 {
			breakdown = append(breakdown, ScoreComponent{"Competitor Quality Gap", avgRating, config.RatingBonusWeight, ratingBonus, "Positive"})
		}
	}

	if config.UseCosts {
		breakdown = append(breakdown, ScoreComponent{"Cost / Rent Penalty", avgPrice, config.CostPenaltyWeight, -costPenalty, "Negative"})
	}

	if isFoodDesert {
		breakdown = append(breakdown, ScoreComponent{"Food Desert Bonus", 1.0, config.FoodDesertBonus, config.FoodDesertBonus, "Positive"})
	}

	if gentVal != 0 {
		impact := "Neutral"
		if gentVal*config.GentrificationWeight > 0 {
			impact = "Positive"
		} else if gentVal*config.GentrificationWeight < 0 {
			impact = "Negative"
		}
		breakdown = append(breakdown, ScoreComponent{"Gentrification Mod", gentVal, config.GentrificationWeight, gentVal * config.GentrificationWeight, impact})
	}

	return int(finalScore), avgPrice, reviewSum, compCount, suppCount, avgRating, popCount, resCount, breakdown, demo, costs, assumptions
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
	address := r.URL.Query().Get("address")
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")

	lat, lng := 0.0, 0.0
	if address != "" {
		l, ln, err := geocodeAddress(address)
		if err == nil {
			lat, lng = l, ln
		}
	} else {
		lat, _ = strconv.ParseFloat(latStr, 64)
		lng, _ = strconv.ParseFloat(lngStr, 64)
	}

	useFootTraffic := r.URL.Query().Get("useFootTraffic") != "false"
	useCosts := r.URL.Query().Get("useCosts") != "false"
	useCompetitors := r.URL.Query().Get("useCompetitors") != "false"
	allowApproximations := r.URL.Query().Get("allowApproximations") != "false"
	targetTime := r.URL.Query().Get("targetTime")

	var recommendations []BusinessRecommendation

	for _, profile := range BusinessProfiles {
		config := ScoreConfig{
			UseFootTraffic:       useFootTraffic,
			UseCosts:             useCosts,
			UseCompetitors:       useCompetitors,
			AllowApproximations:  allowApproximations,
			BusinessType:         profile.NAICS,
			ComputationMethod:    "standard",
			TargetTime:           targetTime,
			TrafficWeight:        profile.TrafficWeight,
			CompPenaltyWeight:    profile.CompPenaltyWeight,
			SuppBonusWeight:      profile.SuppBonusWeight,
			CostPenaltyWeight:    profile.CostPenaltyWeight,
			RatingBonusWeight:    profile.RatingBonusWeight,
			FoodDesertBonus:      profile.FoodDesertBonus,
			GentrificationWeight: profile.GentrificationWeight,
		}

		score, _, _, compCount, suppCount, _, _, _, _, _, _, _ := a.calculateOpportunityScore(lat, lng, config)

		recommendations = append(recommendations, BusinessRecommendation{
			Profile: profile,
			Score:   float64(score),
			Details: fmt.Sprintf("Competitors: %d | Supporters: %d", compCount, suppCount),
		})
	}

	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"recommendations": recommendations,
		"resolvedLat":     lat,
		"resolvedLng":     lng,
	})
}

func (a *App) handleEvaluateLocation(w http.ResponseWriter, r *http.Request) {
	var req EvalRequest
	if r.Method == "POST" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		req.Address = r.URL.Query().Get("address")
		req.Lat, _ = strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
		req.Lng, _ = strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
		req.UseFootTraffic = r.URL.Query().Get("useFootTraffic") != "false"
		req.UseCosts = r.URL.Query().Get("useCosts") != "false"
		req.UseCompetitors = r.URL.Query().Get("useCompetitors") != "false"
		req.AllowApproximations = r.URL.Query().Get("allowApproximations") != "false"
		req.Naics = r.URL.Query().Get("naics")
		req.ComputationMethod = r.URL.Query().Get("computationMethod")
		req.TargetTime = r.URL.Query().Get("targetTime")
		req.TrafficW = parseFloatParam(r, "trafficW", 1.0)
		req.CompW = parseFloatParam(r, "compW", 8.0)
		req.SuppW = parseFloatParam(r, "suppW", 1.5)
		req.CostW = parseFloatParam(r, "costW", 5.0)
		req.RatingW = parseFloatParam(r, "ratingW", 15.0)
		req.FoodDesertW = parseFloatParam(r, "foodDesertW", 0.0)
		req.GentrificationW = parseFloatParam(r, "gentrificationW", 0.0)
	}

	resolvedAddr := req.Address
	if resolvedAddr != "" && req.Lat == 0 && req.Lng == 0 {
		lat, lng, err := geocodeAddress(req.Address)
		if err == nil {
			req.Lat = lat
			req.Lng = lng
		}
	} else if req.Lat != 0 && req.Lng != 0 && resolvedAddr == "" {
		resolvedAddr = reverseGeocode(req.Lat, req.Lng)
	}

	config := ScoreConfig{
		UseFootTraffic:       req.UseFootTraffic,
		UseCosts:             req.UseCosts,
		UseCompetitors:       req.UseCompetitors,
		AllowApproximations:  req.AllowApproximations,
		BusinessType:         req.Naics,
		ComputationMethod:    req.ComputationMethod,
		TargetTime:           req.TargetTime,
		TrafficWeight:        req.TrafficW,
		CompPenaltyWeight:    req.CompW,
		SuppBonusWeight:      req.SuppW,
		CostPenaltyWeight:    req.CostW,
		RatingBonusWeight:    req.RatingW,
		FoodDesertBonus:      req.FoodDesertW,
		GentrificationWeight: req.GentrificationW,
		Overrides:            req.Overrides,
	}

	score, _, totalReviews, calcCompCount, calcSuppCount, avgRating, popCount, resCount, breakdown, demo, costs, assumptions := a.calculateOpportunityScore(req.Lat, req.Lng, config)

	resp := LocationEvalResponse{
		Lat:                         req.Lat,
		Lng:                         req.Lng,
		ResolvedAddress:             resolvedAddr,
		NearbyCompetitors:           calcCompCount,
		SupportiveBusinesses:        calcSuppCount,
		OpportunityScore:            float64(score),
		ReviewCount:                 totalReviews,
		CitywideActiveTaxCompetitor: 0,
		IsApproximated:              false,
		CalcBreakdown:               breakdown,
		Demographics:                demo,
		OperatingCosts:              costs,
	}

	if a.DB != nil {
		var zoneName string
		errZone := a.DB.QueryRow(`
			SELECT zone_name
			FROM sandag_layer_zoning_base_sd_new
			ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC
			LIMIT 1
		`, req.Lng, req.Lat).Scan(&zoneName)
		if errZone == nil {
			resp.DemographicProfile = fmt.Sprintf("Zone Context: %s", zoneName)
			if strings.Contains(strings.ToLower(zoneName), "residential") {
				if config.BusinessType == "311811" || config.BusinessType == "454" {
					resp.DemographicProfile += " (Valid for Home Kitchen / Mobile Setup)"
				} else {
					resp.DemographicProfile += " (Warning: Primarily Residential Zoning for Core Commercial)"
				}
			}
		} else {
			resp.DemographicProfile = "Commercial / Mixed Zone (Interpolated)"
		}

		var retailVisits int
		errRetail := a.DB.QueryRow(`
            SELECT raw_visit_counts 
            FROM pass_by_retail_store_foot_traffic_yelp_category 
            ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC
            LIMIT 1`).Scan(&retailVisits)
		if errRetail == nil {
			resp.FootTraffic = &retailVisits
			resp.FootTrafficSource = "Direct UCSF Proxy Counts"
		}
	}

	if config.Overrides.FootTraffic != nil {
		resp.FootTraffic = config.Overrides.FootTraffic
		resp.FootTrafficSource = "User Override Applied"
		resp.IsApproximated = false
	} else if resp.FootTraffic == nil {
		if config.AllowApproximations {
			approxTraffic := totalReviews * 12
			resp.FootTraffic = &approxTraffic
			resp.FootTrafficSource = "Approximated via GM Volumetrics"
			resp.IsApproximated = true
			assumptions = append(assumptions, "Used Google Maps review volumetrics to approximate UCSF foot traffic values.")
		} else {
			resp.FootTrafficSource = "Approximations Disabled (Strict Mode)"
			resp.FootTraffic = nil
		}
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

	resp.Assumptions = assumptions

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
			WHERE (zone_name ILIKE '%Commercial%' OR zone_name ILIKE '%Mixed%' OR $5 = true)
			  AND ST_Y(ST_Centroid(geom)) BETWEEN $1 AND $2
			  AND ST_X(ST_Centroid(geom)) BETWEEN $3 AND $4
			LIMIT 300;
		`
		allowResidential := false
		if config.BusinessType == "311811" || config.BusinessType == "454" {
			allowResidential = true
		}
		rows, err := a.DB.Query(query, latStart, latEnd, lngStart, lngEnd, allowResidential)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var lat, lng float64
				var name string
				if err := rows.Scan(&lat, &lng, &name); err == nil {
					score, _, _, _, _, _, _, _, _, _, _, _ := a.calculateOpportunityScore(lat, lng, config)
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
					score, _, _, _, _, _, _, _, _, _, _, _ := a.calculateOpportunityScore(gm.Lat, gm.Lng, config)
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
	keywords := a.getNAICSKeywords(config.BusinessType)

	for _, gm := range gmData {
		if gm.Lat >= latStart && gm.Lat <= latEnd && gm.Lng >= lngStart && gm.Lng <= lngEnd {
			catLower := strings.ToLower(gm.Category)

			isCompetitor := false
			if len(keywords) > 0 {
				for _, kw := range keywords {
					if strings.Contains(catLower, kw) {
						isCompetitor = true
						break
					}
				}
			}

			if !isCompetitor {
				if strings.HasPrefix(config.BusinessType, "445") && (strings.Contains(catLower, "grocery") || strings.Contains(catLower, "supermarket")) {
					isCompetitor = true
				} else if strings.HasPrefix(config.BusinessType, "722") && (strings.Contains(catLower, "restaurant") || strings.Contains(catLower, "cafe")) {
					isCompetitor = true
				} else if (strings.HasPrefix(config.BusinessType, "454") || strings.HasPrefix(config.BusinessType, "311811")) && (strings.Contains(catLower, "truck") || strings.Contains(catLower, "stand") || strings.Contains(catLower, "bakery")) {
					isCompetitor = true
				} else if config.BusinessType == "" && (strings.Contains(catLower, "restaurant") || strings.Contains(catLower, "food")) {
					isCompetitor = true
				}
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

	config := ScoreConfig{
		UseFootTraffic:       r.URL.Query().Get("useFootTraffic") != "false",
		UseCosts:             r.URL.Query().Get("useCosts") != "false",
		UseCompetitors:       r.URL.Query().Get("useCompetitors") != "false",
		AllowApproximations:  r.URL.Query().Get("allowApproximations") != "false",
		BusinessType:         naics,
		ComputationMethod:    r.URL.Query().Get("computationMethod"),
		TargetTime:           r.URL.Query().Get("targetTime"),
		TrafficWeight:        parseFloatParam(r, "trafficW", 1.0),
		CompPenaltyWeight:    parseFloatParam(r, "compW", 8.0),
		SuppBonusWeight:      parseFloatParam(r, "suppW", 1.5),
		CostPenaltyWeight:    parseFloatParam(r, "costW", 5.0),
		RatingBonusWeight:    parseFloatParam(r, "ratingW", 15.0),
		FoodDesertBonus:      parseFloatParam(r, "foodDesertW", 0.0),
		GentrificationWeight: parseFloatParam(r, "gentrificationW", 0.0),
	}

	nBound, _ := strconv.ParseFloat(r.URL.Query().Get("n"), 64)
	sBound, _ := strconv.ParseFloat(r.URL.Query().Get("s"), 64)
	eBound, _ := strconv.ParseFloat(r.URL.Query().Get("e"), 64)
	wBound, _ := strconv.ParseFloat(r.URL.Query().Get("w"), 64)

	if nBound == 0 && sBound == 0 {
		nBound, sBound, eBound, wBound = 32.95, 32.65, -116.95, -117.30
	}

	cacheKey := getCacheKey("map", naics, "x", "x", "x", sBound, nBound, wBound, eBound, config)
	if cachedResult, ok := calculationCache.Load(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cachedResult.([]byte))
		return
	}

	oppPoints, sqlCount, csvCount, dbStatus := a.getRealOpportunities(sBound, nBound, wBound, eBound, config)

	sort.Slice(oppPoints, func(i, j int) bool {
		return oppPoints[i].Score > oppPoints[j].Score
	})

	var allocatedPoints []MapPoint
	minDistanceSq := 0.0001
	for _, p := range oppPoints {
		tooClose := false
		for _, ap := range allocatedPoints {
			dSq := (p.Lat-ap.Lat)*(p.Lat-ap.Lat) + (p.Lng-ap.Lng)*(p.Lng-ap.Lng)
			if dSq < minDistanceSq {
				tooClose = true
				break
			}
		}
		if !tooClose {
			p.Name = "[Top Allocated Parcel] " + p.Name
			allocatedPoints = append(allocatedPoints, p)
		}
		if len(allocatedPoints) >= 5 {
			break
		}
	}

	var finalPoints []MapPoint
	for _, ap := range allocatedPoints {
		finalPoints = append(finalPoints, ap)
	}
	for _, p := range oppPoints {
		isAlloc := false
		for _, ap := range allocatedPoints {
			if p.Lat == ap.Lat && p.Lng == ap.Lng {
				isAlloc = true
				break
			}
		}
		if !isAlloc {
			finalPoints = append(finalPoints, p)
		}
	}

	compPoints := a.getDynamicCompetitors(sBound, nBound, wBound, eBound, config)
	allPoints := append(compPoints, finalPoints...)

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

func (a *App) handleGetDemographics(w http.ResponseWriter, r *http.Request) {
	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, _ := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)

	demo, _, _ := a.fetchDemographics(lat, lng, 0, ScoreConfig{})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(demo)
}

func (a *App) handleGetCompetitors(w http.ResponseWriter, r *http.Request) {
	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, _ := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	naics := r.URL.Query().Get("naics")

	// Default radius rough approx for 1 mile
	config := ScoreConfig{BusinessType: naics}
	points := a.getDynamicCompetitors(lat-0.015, lat+0.015, lng-0.015, lng+0.015, config)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}

func (a *App) callLLM(userMessage, apiKey, model, provider, baseUrl string) string {
	token := apiKey
	if token == "" {
		token = os.Getenv("NRP_API_TOKEN")
	}

	if token == "" || token == "your_nrp_token_here" {
		return "⚠️ **Missing API Token**: Please configure your API Token in the Agent Settings (⚙️ icon)."
	}

	systemPrompt := `You are the Nourish PT Data Agent. You help users analyze the San Diego food map. Give clear, data-driven advice about food business placements. You can format things in markdown.[SYSTEM KNOWLEDGE: DATABASE SCHEMA SUMMARY]
- NAICS Reference (2022_NAICS_Keywords, 2022_naics_descriptions): Maps standard industry codes to descriptions, keywords, and investment requirements.
- Demographics (ESRI_SD_County_Tract_Level_Market_Potential_Data, ESRI_SD_County_Tract_Level_consumer_spending, bgs_sd_imp): Comprehensive Census/Tract/Block Group level metrics covering population, income, race, wealth index, and detailed consumer spending behaviors.
- FNDDS & USDA (FNDDS Foods and Beverages, usda_2022_branded_food_product): Detailed nutritional profiles, ingredients, portion weights, and branded food category cross-references.
- Food Access (Food_Access_Research_Atlas_Data_2019, Food_Environment_Atlas_State_County): USDA metrics on Food Deserts, LILA (Low Income Low Access) tracts, and food insecurity rates.
- Businesses & Locations (ca_business, SD_City_Business_Directory, sba_franchise_directory): Active CA businesses, SD city tax listings, coordinates, AI-assessed franchise reasoning, and SBA directory.
- Nourish Core Metrics (nourish_cbg_pedestrian_flow, nourish_cbg_demographics, nourish_cbg_food_environment, nourish_cbg_population_time): Curated Nourish tables containing foot traffic flows, day/night population changes, community sentiment, and specific demographic boundaries.
- Economics & Types (nourish_ref_bakery_economics, nourish_ref_mobile_vendor_economics, nourish_comm_commissary_ext): Baseline operating costs, labor requirements, expected rent, and startup modeling for specific food business formats.
- Geospatial/Zoning (sandag_layer_zoning_base_sd_new, sandag_layer_roads, entity_blockgroup): Map layers defining commercial/mixed/residential zoning polygons, road proximity (highways/primary), and geometry.

Use this knowledge to answer data availability queries and recommend variables for opportunity scoring.`

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
          {"name": "lat", "in": "query", "schema": {"type": "number"}},
          {"name": "lng", "in": "query", "schema": {"type": "number"}},
          {"name": "address", "in": "query", "schema": {"type": "string"}},
          {"name": "naics", "in": "query", "schema": {"type": "string"}},
          {"name": "allowApproximations", "in": "query", "schema": {"type": "boolean"}}
        ],
        "responses": {
          "200": { "description": "Returns LocationEvalResponse JSON containing operating costs, demographics, and competitive density" }
        }
      },
      "post": {
        "summary": "Evaluate a specific location with custom data overrides",
        "requestBody": {
          "content": {
             "application/json": {
                "schema": {
                   "type": "object",
                   "properties": {
                      "address": {"type": "string"},
                      "lat": {"type": "number"},
                      "lng": {"type": "number"},
                      "overrides": {"type": "object"}
                   }
                }
             }
          }
        },
        "responses": {
          "200": { "description": "Evaluation with manual overrides applied" }
        }
      }
    },
    "/api/demographics": {
      "get": {
        "summary": "Fetch raw demographics for specific point",
        "parameters":[
          {"name": "lat", "in": "query", "required": true, "schema": {"type": "number"}},
          {"name": "lng", "in": "query", "required": true, "schema": {"type": "number"}}
        ],
        "responses": {
          "200": { "description": "Returns JSON containing income, daytime/nighttime pop, etc." }
        }
      }
    },
    "/api/competitors": {
      "get": {
        "summary": "List direct competitors within approx 1-mile bounds of lat/lng",
        "parameters":[
          {"name": "lat", "in": "query", "required": true, "schema": {"type": "number"}},
          {"name": "lng", "in": "query", "required": true, "schema": {"type": "number"}},
          {"name": "naics", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {
          "200": { "description": "Returns an array of JSON competitor entities" }
        }
      }
    },
    "/api/recommend-business": {
      "get": {
        "summary": "Recommend best business types for a given location",
        "parameters":[
          {"name": "address", "in": "query", "schema": {"type": "string"}},
          {"name": "lat", "in": "query", "schema": {"type": "number"}},
          {"name": "lng", "in": "query", "schema": {"type": "number"}}
        ],
        "responses": {
          "200": { "description": "Returns a sorted array of business recommendations" }
        }
      }
    }
  }
}`
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(spec))
}
