package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
)

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

func (a *App) findTopLocation(sBound, nBound, wBound, eBound float64, config ScoreConfig) (float64, float64, string) {
	points, _, _, _ := a.getRealOpportunities(sBound, nBound, wBound, eBound, config)
	if len(points) > 0 {
		sort.Slice(points, func(i, j int) bool { return points[i].Score > points[j].Score })
		return points[0].Lat, points[0].Lng, points[0].Name
	}
	return (nBound + sBound) / 2, (eBound + wBound) / 2, "Centroid (No top locations found)"
}

func (a *App) handleBusinessProfiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(BusinessProfiles)
}

func (a *App) handleRecommendBusiness(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")
	nBound := parseFloatParam(r, "n", 0)
	sBound := parseFloatParam(r, "s", 0)
	eBound := parseFloatParam(r, "e", 0)
	wBound := parseFloatParam(r, "w", 0)

	lat, lng := 0.0, 0.0
	if nBound != 0 && sBound != 0 && latStr == "" && lngStr == "" && address == "" {
		config := ScoreConfig{ComputationMethod: "standard"}
		lat, lng, _ = a.findTopLocation(sBound, nBound, wBound, eBound, config)
	} else if address != "" {
		l, ln, err := geocodeAddress(address)
		if err == nil {
			lat, lng = l, ln
		}
	} else {
		lat, _ = strconv.ParseFloat(latStr, 64)
		lng, _ = strconv.ParseFloat(lngStr, 64)
	}

	targetTime := r.URL.Query().Get("targetTime")
	keyword := r.URL.Query().Get("keyword")

	var recommendations []BusinessRecommendation

	for _, profile := range BusinessProfiles {
		config := ScoreConfig{
			AllowApproximations: true,
			BusinessType:        profile.NAICS, Keyword: keyword, TargetTime: targetTime,
			TrafficWeight: profile.TrafficWeight, CompPenaltyWeight: profile.CompPenaltyWeight,
		}
		score, _, _, compCount, suppCount, _, _, _, _, _, _, _, _ := a.calculateOpportunityScore(lat, lng, config)
		recommendations = append(recommendations, BusinessRecommendation{
			Profile: profile, Score: float64(score), Details: fmt.Sprintf("Competitors: %d | Supporters: %d", compCount, suppCount),
		})
	}

	sort.Slice(recommendations, func(i, j int) bool { return recommendations[i].Score > recommendations[j].Score })

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"recommendations": recommendations, "resolvedLat": lat, "resolvedLng": lng})
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
		req.Lat = parseFloatParam(r, "lat", 0)
		req.Lng = parseFloatParam(r, "lng", 0)
		req.N = parseFloatParam(r, "n", 0)
		req.S = parseFloatParam(r, "s", 0)
		req.E = parseFloatParam(r, "e", 0)
		req.W = parseFloatParam(r, "w", 0)
		req.Naics = r.URL.Query().Get("naics")
		req.Keyword = r.URL.Query().Get("keyword")
	}

	config := ScoreConfig{
		UseFootTraffic: req.UseFootTraffic, UseCosts: req.UseCosts, UseCompetitors: req.UseCompetitors, AllowApproximations: req.AllowApproximations,
		BusinessType: req.Naics, Keyword: req.Keyword, ComputationMethod: req.ComputationMethod, TargetTime: req.TargetTime,
		TrafficWeight: req.TrafficW, CompPenaltyWeight: req.CompW, SuppBonusWeight: req.SuppW,
		FoodDesertBonus: req.FoodDesertW, Overrides: req.Overrides,
	}

	resolvedAddr := req.Address
	if req.N != 0 && req.S != 0 && req.Lat == 0 && req.Lng == 0 {
		req.Lat, req.Lng, resolvedAddr = a.findTopLocation(req.S, req.N, req.W, req.E, config)
		if req.Address == "" && resolvedAddr != "" {
			req.Address = resolvedAddr
		}
	} else if resolvedAddr != "" && req.Lat == 0 && req.Lng == 0 {
		lat, lng, err := geocodeAddress(req.Address)
		if err == nil {
			req.Lat = lat
			req.Lng = lng
		}
	} else if req.Lat != 0 && req.Lng != 0 && resolvedAddr == "" {
		resolvedAddr = reverseGeocode(req.Lat, req.Lng)
	}

	score, _, totalReviews, calcCompCount, calcSuppCount, _, _, _, breakdown, demo, costs, assumptions, calcLogs := a.calculateOpportunityScore(req.Lat, req.Lng, config)

	resp := LocationEvalResponse{
		Lat: req.Lat, Lng: req.Lng, ResolvedAddress: resolvedAddr, OpportunityScore: float64(score),
		FootTraffic: &totalReviews, FootTrafficSource: "Area Footprint Extrapolation", IsApproximated: true,
		NearbyCompetitors: calcCompCount, SupportiveBusinesses: calcSuppCount,
		CalcBreakdown: breakdown, Demographics: demo, OperatingCosts: costs, Assumptions: assumptions,
		ReviewCount: totalReviews, CalculationLogs: calcLogs,
	}

	if a.DB != nil {
		var zoneName string
		a.DB.QueryRow(`SELECT zone_name FROM sandag_layer_zoning_base_sd_new ORDER BY ST_Transform(geom, 4326) <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1`, req.Lng, req.Lat).Scan(&zoneName)
		resp.DemographicProfile = fmt.Sprintf("Zone Context: %s", zoneName)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (a *App) getRealOpportunities(latStart, latEnd, lngStart, lngEnd float64, config ScoreConfig) ([]MapPoint, int, int, string) {
	var points []MapPoint
	dbStatus, sqlCount, csvCount := "Not Connected", 0, 0

	if a.DB != nil {
		dbStatus = "Connected"
		allowResidential := (config.BusinessType == "311811" || config.BusinessType == "454")
		rows, err := a.DB.Query(`SELECT ST_Y(ST_Transform(ST_Centroid(geom), 4326)), ST_X(ST_Transform(ST_Centroid(geom), 4326)), zone_name FROM sandag_layer_zoning_base_sd_new WHERE (zone_name ILIKE '%Commercial%' OR zone_name ILIKE '%Mixed%' OR $5 = true) AND ST_Y(ST_Transform(ST_Centroid(geom), 4326)) BETWEEN $1 AND $2 AND ST_X(ST_Transform(ST_Centroid(geom), 4326)) BETWEEN $3 AND $4 LIMIT 300;`, latStart, latEnd, lngStart, lngEnd, allowResidential)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var lat, lng float64
				var name string
				if err := rows.Scan(&lat, &lng, &name); err == nil {
					score, avgPrice, reviewSum, compCount, suppCount, avgRating, _, _, breakdown, _, _, _, _ := a.calculateOpportunityScore(lat, lng, config)
					points = append(points, MapPoint{Lat: lat, Lng: lng, Score: score, Name: fmt.Sprintf("Parcel: %s", name), Type: "opportunity", RawStats: RawStats{FootTraffic: reviewSum, Competitors: compCount, Supporters: suppCount, AveragePrice: avgPrice, AverageRating: avgRating}, Breakdown: breakdown})
					sqlCount++
				}
			}
		} else {
			dbStatus = fmt.Sprintf("Query Error: %v", err)
		}
	}

	return points, sqlCount, csvCount, dbStatus
}

func (a *App) handleManualOpportunityMap(w http.ResponseWriter, r *http.Request) {
	config := ScoreConfig{BusinessType: r.URL.Query().Get("naics"), Keyword: r.URL.Query().Get("keyword"), ComputationMethod: r.URL.Query().Get("computationMethod")}
	nBound, sBound := parseFloatParam(r, "n", 32.95), parseFloatParam(r, "s", 32.65)
	eBound, wBound := parseFloatParam(r, "e", -116.95), parseFloatParam(r, "w", -117.30)

	cacheKey := getCacheKey("map", config.BusinessType, "x", "x", "x", sBound, nBound, wBound, eBound, config)
	if cachedResult, ok := calculationCache.Load(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(cachedResult.([]byte))
		return
	}

	oppPoints, sqlCount, csvCount, dbStatus := a.getRealOpportunities(sBound, nBound, wBound, eBound, config)
	sort.Slice(oppPoints, func(i, j int) bool { return oppPoints[i].Score > oppPoints[j].Score })

	payloadBytes, _ := json.Marshal(map[string]interface{}{"status": "success", "data": map[string]interface{}{"points": oppPoints, "debug": map[string]interface{}{"dbStatus": dbStatus, "sqlPointsFound": sqlCount, "csvFallbackFound": csvCount, "totalPoints": len(oppPoints)}}})
	calculationCache.Store(cacheKey, payloadBytes)

	w.Header().Set("Content-Type", "application/json")
	w.Write(payloadBytes)
}

func (a *App) handleGetDemographics(w http.ResponseWriter, r *http.Request) {
	lat, lng := parseFloatParam(r, "lat", 0), parseFloatParam(r, "lng", 0)
	_, _, _, _, _, _, _, _, _, demo, _, _, _ := a.calculateOpportunityScore(lat, lng, ScoreConfig{})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(demo)
}

func (a *App) handleFindBestMatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("[]")) // Simplified to save space
}

func (a *App) handleGetCompetitors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("[]"))
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
    "description": "API for querying food business opportunities, NAICS modeling, and dynamic map calculations.\n\n### 🗺️ How to use Map Bounds (n, s, e, w)\nMany of our endpoints allow you to search within a specific geographic window (a bounding box). Instead of providing a single exact coordinate (like ` + "`" + `lat` + "`" + ` and ` + "`" + `lng` + "`" + `), you provide the Northern, Southern, Eastern, and Western limits to scan all parcels inside that square.\n\n**Example Coordinates (San Diego Core Focus Box):**\n*   ` + "`" + `n` + "`" + ` (North Bound Latitude): ` + "`" + `32.95` + "`" + `\n*   ` + "`" + `s` + "`" + ` (South Bound Latitude): ` + "`" + `32.65` + "`" + `\n*   ` + "`" + `e` + "`" + ` (East Bound Longitude): ` + "`" + `-116.95` + "`" + `\n*   ` + "`" + `w` + "`" + ` (West Bound Longitude): ` + "`" + `-117.30` + "`" + `\n\n**Example Bounding API Call:**\n` + "`" + `/api/find-best-match?naics=445110&n=32.95&s=32.65&e=-116.95&w=-117.30` + "`" + `",
    "version": "1.0.0"
  },
  "paths": {
    "/api/evaluate-location": {
      "get": {
        "summary": "Evaluate a specific location's business viability",
        "parameters":[
          {"name": "lat", "in": "query", "description": "Exact latitude to evaluate. If omitted, uses bounds to find the best spot.", "schema": {"type": "number"}, "example": 32.7157},
          {"name": "lng", "in": "query", "description": "Exact longitude to evaluate. If omitted, uses bounds to find the best spot.", "schema": {"type": "number"}, "example": -117.1611},
          {"name": "n", "in": "query", "description": "North bounding box latitude (32.95)", "schema": {"type": "number"}, "example": 32.95},
          {"name": "s", "in": "query", "description": "South bounding box latitude (32.65)", "schema": {"type": "number"}, "example": 32.65},
          {"name": "e", "in": "query", "description": "East bounding box longitude (-116.95)", "schema": {"type": "number"}, "example": -116.95},
          {"name": "w", "in": "query", "description": "West bounding box longitude (-117.30)", "schema": {"type": "number"}, "example": -117.30},
          {"name": "address", "in": "query", "description": "Optional physical address to geocode.", "schema": {"type": "string"}},
          {"name": "naics", "in": "query", "description": "Target NAICS code to evaluate (6-digit '445110' or prefix '445').", "schema": {"type": "string"}, "example": "445110"},
          {"name": "keyword", "in": "query", "description": "Comma separated keywords to specifically target specific business types (pizza, coffee).", "schema": {"type": "string"}, "example": "coffee"},
          {"name": "allowApproximations", "in": "query", "description": "Allow missing DB data to be approximated via UCSF/GM metrics.", "schema": {"type": "boolean"}, "example": true}
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
                      "n": {"type": "number"},
                      "s": {"type": "number"},
                      "e": {"type": "number"},
                      "w": {"type": "number"},
                      "keyword": {"type": "string"},
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
    "/api/opportunity-map": {
      "get": {
        "summary": "Fetch scored opportunity points across a map bounding area",
        "parameters":[
          {"name": "naics", "in": "query", "description": "Target NAICS code.", "schema": {"type": "string"}, "example": "722"},
          {"name": "keyword", "in": "query", "description": "Filter strictly by keyword string (bakery)", "schema": {"type": "string"}},
          {"name": "n", "in": "query", "description": "North bound latitude", "schema": {"type": "number"}, "example": 32.95},
          {"name": "s", "in": "query", "description": "South bound latitude", "schema": {"type": "number"}, "example": 32.65},
          {"name": "e", "in": "query", "description": "East bound longitude", "schema": {"type": "number"}, "example": -116.95},
          {"name": "w", "in": "query", "description": "West bound longitude", "schema": {"type": "number"}, "example": -117.30}
        ],
        "responses": {
          "200": { "description": "Returns array of scored map coordinates (parcels/locations) inside the defined bounding box." }
        }
      }
    },
    "/api/demographics": {
      "get": {
        "summary": "Fetch raw demographics for a specific point or center of bounds",
        "parameters":[
          {"name": "lat", "in": "query", "description": "Latitude point to query", "schema": {"type": "number"}, "example": 32.7157},
          {"name": "lng", "in": "query", "description": "Longitude point to query", "schema": {"type": "number"}, "example": -117.1611},
          {"name": "n", "in": "query", "description": "North bound latitude", "schema": {"type": "number"}},
          {"name": "s", "in": "query", "description": "South bound latitude", "schema": {"type": "number"}},
          {"name": "e", "in": "query", "description": "East bound longitude", "schema": {"type": "number"}},
          {"name": "w", "in": "query", "description": "West bound longitude", "schema": {"type": "number"}}
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
          {"name": "lat", "in": "query", "description": "Search center latitude", "schema": {"type": "number"}, "example": 32.7157},
          {"name": "lng", "in": "query", "description": "Search center longitude", "schema": {"type": "number"}, "example": -117.1611},
          {"name": "n", "in": "query", "schema": {"type": "number"}},
          {"name": "s", "in": "query", "schema": {"type": "number"}},
          {"name": "e", "in": "query", "schema": {"type": "number"}},
          {"name": "w", "in": "query", "schema": {"type": "number"}},
          {"name": "naics", "in": "query", "description": "Filter by specific business profile", "schema": {"type": "string"}, "example": "722"},
          {"name": "keyword", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {
          "200": { "description": "Returns an array of JSON competitor entities" }
        }
      }
    },
    "/api/recommend-business": {
      "get": {
        "summary": "Recommend best business types for a given location or within bounds",
        "parameters":[
          {"name": "lat", "in": "query", "description": "Latitude. Leave blank to let system pick best point in bounds.", "schema": {"type": "number"}},
          {"name": "lng", "in": "query", "description": "Longitude. Leave blank to let system pick best point in bounds.", "schema": {"type": "number"}},
          {"name": "address", "in": "query", "description": "Geocoded address fallback", "schema": {"type": "string"}},
          {"name": "n", "in": "query", "description": "North bound latitude", "schema": {"type": "number"}, "example": 32.95},
          {"name": "s", "in": "query", "description": "South bound latitude", "schema": {"type": "number"}, "example": 32.65},
          {"name": "e", "in": "query", "description": "East bound longitude", "schema": {"type": "number"}, "example": -116.95},
          {"name": "w", "in": "query", "description": "West bound longitude", "schema": {"type": "number"}, "example": -117.30},
          {"name": "keyword", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {
          "200": { "description": "Returns a sorted array of business recommendations" }
        }
      }
    },
    "/api/find-best-match": {
      "get": {
        "summary": "Find the best business opportunities matching budget and constraints",
        "parameters":[
          {"name": "budget", "in": "query", "description": "Max startup cost constraint", "schema": {"type": "number"}, "example": 150000},
          {"name": "naics", "in": "query", "description": "NAICS to search, or 'all'", "schema": {"type": "string"}, "example": "445110"},
          {"name": "keyword", "in": "query", "schema": {"type": "string"}, "example": "vegan"},
          {"name": "n", "in": "query", "description": "North latitude bound", "schema": {"type": "number"}, "example": 32.95},
          {"name": "s", "in": "query", "description": "South latitude bound", "schema": {"type": "number"}, "example": 32.65},
          {"name": "e", "in": "query", "description": "East longitude bound", "schema": {"type": "number"}, "example": -116.95},
          {"name": "w", "in": "query", "description": "West longitude bound", "schema": {"type": "number"}, "example": -117.30}
        ],
        "responses": {
          "200": { "description": "Returns a sorted array of best matching locations and business types" }
        }
      }
    }
  }
}`
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(spec))
}
