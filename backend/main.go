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

type ScoreConfig struct {
	UseFootTraffic bool
	UseCosts       bool
	UseCompetitors bool
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

var gmData []GMRecord
var taxData []TaxRecord
var calculationCache sync.Map

func getCacheKey(prefix, keyword, foodDesert, rent, popularity string, latStart, latEnd, lngStart, lngEnd float64, config ScoreConfig) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%.3f|%.3f|%.3f|%.3f|%v|%v|%v", prefix, keyword, foodDesert, rent, popularity, latStart, latEnd, lngStart, lngEnd, config.UseFootTraffic, config.UseCosts, config.UseCompetitors)
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

	r.Get("/api/opportunity-map", app.handleManualOpportunityMap)
	r.Get("/api/evaluate-location", app.handleEvaluateLocation)
	r.Post("/api/agent/chat", app.handleAgentChat)
	r.Get("/api/explore-db", app.handleExploreDB)

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

func (a *App) calculateOpportunityScore(lat, lng float64, config ScoreConfig) (int, float64, int, int, float64, int, int, string) {
	compCount := 0
	reviewSum := 0
	priceSum := 0
	priceCount := 0
	ratingSum := 0.0
	resCount := 0
	popCount := 0

	trafficScore := 0.0
	compPenalty := 0.0

	for _, gm := range gmData {
		dLat := gm.Lat - lat
		dLng := gm.Lng - lng
		distSq := dLat*dLat + dLng*dLng

		if distSq < 0.001 { // Approx 2 miles
			distMeters := math.Sqrt(distSq) * 111000
			weight := 1.0
			if distMeters > 50 {
				weight = 50.0 / distMeters
			}

			catLower := strings.ToLower(gm.Category)
			isCompetitor := strings.Contains(catLower, "restaurant") || strings.Contains(catLower, "food") || strings.Contains(catLower, "cafe")

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
				compPenalty += weight * 8.0 // Reduced severity
			} else {
				reviewSum += gm.ReviewCount
				trafficScore += weight * (float64(gm.ReviewCount) / 100.0)
				if gm.HasPopTimes {
					popCount++
					trafficScore += weight * 3.0
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

	// Re-balanced Baseline
	baseScore := 45.0

	// Normalize
	if trafficScore > 40.0 {
		trafficScore = 40.0
	}
	if compPenalty > 40.0 {
		compPenalty = 40.0
	}

	costPenalty := 0.0
	if avgPrice > 0 {
		costPenalty = avgPrice * 5.0 // Max penalty around ~20
	}

	ratingBonus := 0.0
	if compCount > 0 && avgRating < 4.0 && avgRating > 0 {
		ratingBonus = (4.0 - avgRating) * 15.0 // Up to +15 for bad competitors
	}

	// Apply Filter Overrides
	if !config.UseFootTraffic {
		trafficScore = 20.0 // assume average baseline
	}
	if !config.UseCompetitors {
		compPenalty = 0.0
		ratingBonus = 0.0
	}
	if !config.UseCosts {
		costPenalty = 0.0
	}

	finalScore := baseScore + trafficScore - compPenalty - costPenalty + ratingBonus

	if finalScore > 100.0 {
		finalScore = 100.0
	} else if finalScore < 0.0 {
		finalScore = 0.0
	}

	logStr := fmt.Sprintf("Base: %.1f | Traffic Add: +%.1f | Comp Pen: -%.1f | Cost Pen: -%.1f | Gap Bonus: +%.1f",
		baseScore, trafficScore, compPenalty, costPenalty, ratingBonus)

	return int(finalScore), avgPrice, reviewSum, compCount, avgRating, popCount, resCount, logStr
}

func (a *App) handleEvaluateLocation(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")
	naicsFilter := r.URL.Query().Get("naics")

	config := ScoreConfig{
		UseFootTraffic: r.URL.Query().Get("useFootTraffic") != "false",
		UseCosts:       r.URL.Query().Get("useCosts") != "false",
		UseCompetitors: r.URL.Query().Get("useCompetitors") != "false",
	}

	lat, _ := strconv.ParseFloat(latStr, 64)
	lng, _ := strconv.ParseFloat(lngStr, 64)

	score, avgPrice, totalReviews, calcCompCount, avgRating, popCount, resCount, calcLog := a.calculateOpportunityScore(lat, lng, config)

	estCost := "Data unavailable"
	demographicProfile := "Standard Block Group"
	pedestrianVolume := "Unknown"

	if a.DB != nil {
		var zoneName string
		errZone := a.DB.QueryRow(`
			SELECT zone_name
			FROM sandag_layer_zoning_base_sd_new
			ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC
			LIMIT 1
		`, lng, lat).Scan(&zoneName)
		if errZone == nil {
			demographicProfile = fmt.Sprintf("Zone Context: %s", zoneName)
		} else {
			demographicProfile = "Commercial / Mixed Zone (Interpolated)"
		}

		var retailVisits int
		errRetail := a.DB.QueryRow(`
			SELECT raw_visit_counts
			FROM pass_by_retail_store_foot_traffic_yelp_category
			LIMIT 1
		`).Scan(&retailVisits)

		if errRetail == nil {
			pedestrianVolume = fmt.Sprintf("Proxy ~%d Yelp Category Visits", retailVisits)
		} else {
			var avgPedFlow float64
			errPed := a.DB.QueryRow(`SELECT avg(avg_daily_pedestrian_count) FROM nourish_cbg_pedestrian_flow`).Scan(&avgPedFlow)
			if errPed == nil {
				pedestrianVolume = fmt.Sprintf("~%d daily (Scaled baseline)", int(avgPedFlow*(1.0+float64(totalReviews)/1000.0)))
			}
		}

		if avgPrice > 2.5 {
			estCost = fmt.Sprintf("High Costs (Area Avg Price: %.1f/4.0)", avgPrice)
		} else if avgPrice > 0 {
			estCost = fmt.Sprintf("Moderate Costs (Area Avg Price: %.1f/4.0)", avgPrice)
		} else {
			var laborCost float64
			err := a.DB.QueryRow(`SELECT avg(labor_cost_pct_of_revenue) FROM nourish_ref_bakery_economics`).Scan(&laborCost)
			if err == nil {
				estCost = fmt.Sprintf("Avg Labor: %.1f%% of Rev (DB Derived)", laborCost)
			}
		}
	} else {
		if totalReviews > 500 {
			pedestrianVolume = "High (Inferred from Maps Density)"
		} else {
			pedestrianVolume = "Low-Medium (Inferred Density)"
		}

		if avgPrice > 2.5 {
			estCost = fmt.Sprintf("High Operation/Land Costs (Area Avg Price: %.1f/4.0)", avgPrice)
		} else if avgPrice > 0 {
			estCost = fmt.Sprintf("Moderate/Low Area Costs (Area Avg Price: %.1f/4.0)", avgPrice)
		}
	}

	statsMsg := fmt.Sprintf("%d Area Non-Comp Reviews | Avg Comp Rating: %.1f | %d Pop. Times Points", totalReviews, avgRating, popCount)
	if resCount > 0 {
		statsMsg += fmt.Sprintf(" | %d Comp take reservations", resCount)
	}

	citywideTaxListings := 0
	if naicsFilter != "" {
		for _, t := range taxData {
			if strings.HasPrefix(t.NAICS, naicsFilter) {
				citywideTaxListings++
			}
		}
	}

	eval := map[string]interface{}{
		"lat":                         lat,
		"lng":                         lng,
		"footTraffic":                 pedestrianVolume,
		"nearbyCompetitors":           calcCompCount,
		"opportunityScore":            score,
		"demographicProfile":          demographicProfile,
		"estCosts":                    estCost,
		"reviewCount":                 totalReviews,
		"statsExtra":                  statsMsg,
		"calcLog":                     calcLog,
		"citywideActiveTaxCompetitor": citywideTaxListings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(eval)
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
					score, _, _, _, _, _, _, _ := a.calculateOpportunityScore(lat, lng, config)
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
					score, _, _, _, _, _, _, _ := a.calculateOpportunityScore(gm.Lat, gm.Lng, config)
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

func (a *App) getDynamicCompetitors(latStart, latEnd, lngStart, lngEnd float64) []MapPoint {
	var points []MapPoint
	count := 0
	for _, gm := range gmData {
		if gm.Lat >= latStart && gm.Lat <= latEnd && gm.Lng >= lngStart && gm.Lng <= lngEnd {
			catLower := strings.ToLower(gm.Category)
			isCompetitor := strings.Contains(catLower, "restaurant") || strings.Contains(catLower, "food") || strings.Contains(catLower, "cafe")
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
		UseFootTraffic: r.URL.Query().Get("useFootTraffic") != "false",
		UseCosts:       r.URL.Query().Get("useCosts") != "false",
		UseCompetitors: r.URL.Query().Get("useCompetitors") != "false",
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
	compPoints := a.getDynamicCompetitors(sBound, nBound, wBound, eBound)

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
		// OpenAI / NRP Gateway Format
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

	config := ScoreConfig{UseFootTraffic: true, UseCosts: true, UseCompetitors: true}
	oppPoints, _, _, _ := a.getRealOpportunities(latStart, latEnd, lngStart, lngEnd, config)
	compPoints := a.getDynamicCompetitors(latStart, latEnd, lngStart, lngEnd)
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
