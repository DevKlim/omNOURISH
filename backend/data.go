package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var gmData []GMRecord
var taxData[]TaxRecord
var calculationCache sync.Map
var naicsKeywordsCache sync.Map

var sldCache = make(map[string]float64)
var wacCache = make(map[string]int)
var bdsCache = make(map[string]float64)
var cbpCache = make(map[string]float64)
var stopFreqCache = make(map[string]int)
var gentrificationCache = make(map[string]float64)
var transitStops[]TransitStop
var inspections []InspectionRecord
var permitTimes[]int

type TransitStop struct {
	Lat       float64
	Lng       float64
	Frequency int
}

type InspectionRecord struct {
	Lat            float64
	Lng            float64
	DaysToIssuance int
}

func haversineDistSq(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := lat1 - lat2
	dLon := lon1 - lon2
	return dLat*dLat + dLon*dLon
}

func getCacheKey(prefix, keyword, foodDesert, rent, popularity string, latStart, latEnd, lngStart, lngEnd float64, config ScoreConfig) string {
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%.3f|%.3f|%.3f|%.3f|%v|%v|%v|%v|%s|%s|%s|%s|%.2f|%.2f|%.2f|%.2f|%.2f|%.2f|%.2f|%.2f",
		prefix, keyword, foodDesert, rent, popularity, latStart, latEnd, lngStart, lngEnd,
		config.UseFootTraffic, config.UseCosts, config.UseCompetitors, config.AllowApproximations,
		config.BusinessType, config.Keyword, config.ComputationMethod, config.TargetTime, config.TrafficWeight, config.CompPenaltyWeight,
		config.SuppBonusWeight, config.CostPenaltyWeight, config.RatingBonusWeight, config.FoodDesertBonus, config.GentrificationWeight, config.TransitBonusWeight)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

func (a *App) InitDB() {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, dbname, sslmode)

	db, err := sql.Open("postgres", connStr)
	if err == nil {
		err = db.Ping()
		if err == nil {
			log.Println("Database connection established")
			a.DB = db
			go a.exportSchemaToFile()
			return
		}
	}
	log.Printf("Failed to connect to database: %v", err)
}

func (a *App) exportSchemaToFile() {
	if a.DB == nil {
		return
	}
	filePath := "/data/db_schema_dump.json"
	if _, err := os.Stat(filePath); err == nil {
		return
	}

	rows, err := a.DB.Query(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'`)
	if err != nil {
		return
	}
	defer rows.Close()

	schemaDump := make(map[string][]map[string]string)
	var tables[]string
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
	}
}

func parseCSVFiles(files []string, subDir string, processor func([]string, map[string]int)) int {
	bases :=[]string{"../data/", "../../data/", "./data/", "/data/", "/app/data/"}
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
					break
				}
			}
		}
		if !loaded {
			log.Printf("Failed to load %s", file)
		}
	}
	return totalLoaded
}

func loadCSVData() {
	gmFiles :=[]string{
		"lajolla.csv", "lajollashores.csv", "miramar.csv", "miramesa.csv",
		"oldtown.csv", "sandiego.csv", "sorrentovalley.csv", "universitycity.csv",
	}

	parseCSVFiles(gmFiles, "gm", func(row[]string, idxMap map[string]int) {
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
				Title: title, Lat: lat, Lng: lng, ReviewCount: reviewCount, Rating: rating,
				Category: category, PriceRange: priceRange, Reservations: reservations,
				HasPopTimes: hasPop, OpenHours: openHours,
			})
		}
	})

	taxFiles :=[]string{"tr_active1.csv", "tr_active2.csv"}
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
			taxData = append(taxData, TaxRecord{DBAName: dba, Address: addr, NAICS: naics})
		}
	})

	parseCSVFiles([]string{"SmartLocationDb.csv"}, "epa_sld", func(row []string, idxMap map[string]int) {
		geoIdx, okGeo := idxMap["geoid10"]
		d3bIdx, okD3b := idxMap["d3b"]
		if okGeo && okD3b && geoIdx < len(row) && d3bIdx < len(row) {
			d3b, err := strconv.ParseFloat(row[d3bIdx], 64)
			if err == nil {
				sldCache[row[geoIdx]] = d3b
			}
		}
	})

	parseCSVFiles([]string{"ca_wac.csv"}, "lehd", func(row[]string, idxMap map[string]int) {
		geoIdx, okGeo := idxMap["w_geocode"]
		c000Idx, okC000 := idxMap["c000"]
		if okGeo && okC000 && geoIdx < len(row) && c000Idx < len(row) {
			blockId := row[geoIdx]
			if len(blockId) >= 12 {
				bgId := blockId[:12]
				jobs, _ := strconv.Atoi(row[c000Idx])
				wacCache[bgId] += jobs
			}
		}
	})

	parseCSVFiles([]string{"bds_msa.csv"}, "census_business", func(row[]string, idxMap map[string]int) {
		naicsIdx, okN := idxMap["naics"]
		exitIdx, okE := idxMap["estabs_exit_rate"]
		if okN && okE && naicsIdx < len(row) && exitIdx < len(row) {
			rate, err := strconv.ParseFloat(row[exitIdx], 64)
			if err == nil {
				bdsCache[row[naicsIdx]] = rate
			}
		}
	})

	parseCSVFiles([]string{"cbp_county.csv"}, "census_business", func(row[]string, idxMap map[string]int) {
		naicsIdx, okN := idxMap["naics2017"]
		empIdx, okEmp := idxMap["emp"]
		estabIdx, okEstab := idxMap["estab"]
		if okN && okEmp && okEstab && estabIdx < len(row) {
			emp, _ := strconv.ParseFloat(row[empIdx], 64)
			estab, _ := strconv.ParseFloat(row[estabIdx], 64)
			if estab > 0 {
				cbpCache[row[naicsIdx]] = emp / estab
			}
		}
	})

	parseCSVFiles([]string{"stop_times.txt"}, "gtfs", func(row []string, idxMap map[string]int) {
		stopIdx, okS := idxMap["stop_id"]
		if okS && stopIdx < len(row) {
			stopFreqCache[row[stopIdx]]++
		}
	})

	parseCSVFiles([]string{"stops.txt"}, "gtfs", func(row []string, idxMap map[string]int) {
		idIdx, okI := idxMap["stop_id"]
		latIdx, okLat := idxMap["stop_lat"]
		lngIdx, okLng := idxMap["stop_lon"]
		if okI && okLat && okLng && latIdx < len(row) {
			lat, _ := strconv.ParseFloat(row[latIdx], 64)
			lng, _ := strconv.ParseFloat(row[lngIdx], 64)
			freq := stopFreqCache[row[idIdx]]
			transitStops = append(transitStops, TransitStop{Lat: lat, Lng: lng, Frequency: freq})
		}
	})

	parseCSVFiles([]string{"sd_food_inspections.csv"}, "open_data", func(row []string, idxMap map[string]int) {
		latIdx, okLat := idxMap["lat"]
		lngIdx, okLng := idxMap["lon"]
		if okLat && okLng && latIdx < len(row) {
			lat, _ := strconv.ParseFloat(row[latIdx], 64)
			lng, _ := strconv.ParseFloat(row[lngIdx], 64)
			inspections = append(inspections, InspectionRecord{Lat: lat, Lng: lng, DaysToIssuance: 45})
		}
	})

	parseCSVFiles([]string{"census_gentrification.csv"}, "gentrification", func(row[]string, idxMap map[string]int) {
		geoIdx, okGeo := idxMap["geoid"]
		genIdx, okGen := idxMap["gentrification"]
		if okGeo && okGen && geoIdx < len(row) && genIdx < len(row) {
			val, err := strconv.ParseFloat(row[genIdx], 64)
			if err == nil {
				gentrificationCache[row[geoIdx]] = val
			}
		}
	})

	log.Printf("Data loaded: SLD=%d, LODES=%d, BDS=%d, GTFS=%d, Gentrification=%d", len(sldCache), len(wacCache), len(bdsCache), len(transitStops), len(gentrificationCache))
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

	var result[]struct {
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

func (a *App) getNAICSKeywords(code string)[]string {
	if code == "" {
		return nil
	}
	if val, ok := naicsKeywordsCache.Load(code); ok {
		return val.([]string)
	}
	if a.DB == nil {
		return nil
	}

	var keywords[]string
	rows, err := a.DB.Query(`SELECT naics_keywords FROM "2022_NAICS_Keywords" WHERE CAST(naics_code AS TEXT) LIKE $1 || '%'`, code)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var kws sql.NullString
			if err := rows.Scan(&kws); err == nil && kws.Valid {
				parts := strings.Split(kws.String, ",")
				for _, p := range parts {
					clean := strings.ToLower(strings.TrimSpace(p))
					if clean != "" {
						keywords = append(keywords, clean)
					}
				}
			}
		}
	}

	uniqueMap := make(map[string]bool)
	var uniqueKeywords []string
	for _, k := range keywords {
		if !uniqueMap[k] {
			uniqueMap[k] = true
			uniqueKeywords = append(uniqueKeywords, k)
		}
	}

	naicsKeywordsCache.Store(code, uniqueKeywords)
	return uniqueKeywords
}
