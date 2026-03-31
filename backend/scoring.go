package main

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
)

// Helper for constraining values
func clip(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// Normalizes an indicator to a 0.05 - 1.0 scale using min-max scaling (Z_v).
// If positive is true, higher raw values yield higher scores. If false, it's inverted.
func normalize(val, min, max float64, positive bool) float64 {
	val = clip(val, min, max)
	if max <= min {
		return 0.5
	}
	z := (val - min) / (max - min)
	if !positive {
		z = 1.0 - z
	}
	// Avoid returning absolute 0 to prevent zeroing out the geometric mean
	return clip(z, 0.05, 1.0)
}

// Geometric Mean Helper for fusing indicators and scopes
func geomMean(values []float64, weights []float64) float64 {
	sumWeights := 0.0
	sumLog := 0.0
	for i, v := range values {
		w := weights[i]
		if w <= 0 {
			w = 1.0 // Prevent zeroing out scope if UI weight is dropped
		}
		val := clip(v, 0.01, 1.0) // prevent log(0)
		sumLog += w * math.Log(val)
		sumWeights += w
	}
	if sumWeights == 0 {
		return 0
	}
	return math.Exp(sumLog / sumWeights)
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

func (a *App) calculateOpportunityScore(lat, lng float64, config ScoreConfig) (int, float64, int, int, int, float64, int, int, []ScoreComponent, Demographics, DetailedCosts, []string, []string) {
	var assumptions []string
	var breakdown []ScoreComponent
	var logs []string

	logs = append(logs, fmt.Sprintf("==== INIT EVALUATION: Lat %.5f, Lng %.5f ====", lat, lng))

	demo := Demographics{Source: "Database", FoodDesertStatus: false}
	costs := DetailedCosts{Source: "Database"}

	// 1. Establish San Diego Spatial Heuristic Baselines (Fixes Missing Columns)
	distDowntown := math.Sqrt(math.Pow(lat-32.715, 2) + math.Pow(lng - -117.16, 2))

	mhi, popDensity, rentEst, emp, unemp, gentrification, avgIncome, crime := 82000.0, 4800.0, 32.0, 4000.0, 4.5, 2.8, 55000.0, 100.0
	popGrowth := 1.2
	snapRate := 0.08
	closureRate := 0.38

	if lng < -117.20 {
		mhi, popDensity, rentEst, emp, unemp, gentrification, avgIncome, crime = 115000.0, 5500.0, 55.0, 3500.0, 3.2, 4.1, 85000.0, 60.0
		logs = append(logs, "MODEL: Identified as SD Coastal. Injected High-Income/Rent Baselines.")
	} else if distDowntown < 0.05 {
		mhi, popDensity, rentEst, emp, unemp, gentrification, avgIncome, crime = 85000.0, 14000.0, 45.0, 12000.0, 4.8, 6.5, 65000.0, 140.0
		logs = append(logs, "MODEL: Identified as SD Urban Core. Injected High-Density Baselines.")
	} else if lng > -117.0 {
		mhi, popDensity, rentEst, emp, unemp, gentrification, avgIncome, crime = 68000.0, 3800.0, 26.0, 1800.0, 6.1, 1.5, 45000.0, 90.0
		logs = append(logs, "MODEL: Identified as SD East County. Injected Inland Mod-Income Baselines.")
	} else {
		logs = append(logs, "MODEL: Identified as SD Central. Injected Standard Baseline Metrics.")
	}

	wealthIndex := 100.0
	transitStops := 0
	isFoodDesert := false
	hasSharedKitchen := false
	isCommercialZone := false

	if a.DB != nil {
		logs = append(logs, "CMD: Executing PostGIS spatial JOIN on bgs_sd_imp for Demographics (L1-L3, M2, M4, M6)...")

		var dbMhi, dbPop, dbEmp, dbWlth, dbUnemp, dbCrm, dbAvgia sql.NullFloat64
		var statefp, countyfp, tractce sql.NullString

		// NOTE: Updated unemp_cy to unemprt_cy based on verified DB schema
		errDemo := a.DB.QueryRow(`
			SELECT b.medhinc_cy, 
			       (b.totpop_cy / NULLIF(ST_Area(ST_Transform(e.geom, 4326)::geography) / 2589988.11, 0)) AS pop_density,
			       b.emp_cy, b.wlthindxcy, b.unemprt_cy, b.crm_cy, b.avgia_cy,
			       b.statefp, b.countyfp, b.tractce
			FROM bgs_sd_imp b JOIN entity_blockgroup e ON b.ogc_fid = e.ogc_fid
			ORDER BY ST_Transform(e.geom, 4326) <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1
		`, lng, lat).Scan(&dbMhi, &dbPop, &dbEmp, &dbWlth, &dbUnemp, &dbCrm, &dbAvgia, &statefp, &countyfp, &tractce)

		if errDemo == nil {
			logs = append(logs, "RESULT: Successfully extracted core DB block-group demographics.")
			if dbMhi.Valid {
				mhi = dbMhi.Float64
			}
			if dbPop.Valid {
				popDensity = dbPop.Float64
			}
			if dbEmp.Valid {
				emp = dbEmp.Float64
			}
			if dbWlth.Valid {
				wealthIndex = dbWlth.Float64
			}
			if dbUnemp.Valid {
				unemp = dbUnemp.Float64
			}
			if dbCrm.Valid {
				crime = dbCrm.Float64
			}
			if dbAvgia.Valid {
				avgIncome = dbAvgia.Float64
			}
			demo.Source = "PostGIS DB Extracted"

			// M3 (SNAP Participation) - Cross referencing Food Access Research Atlas
			if statefp.Valid && countyfp.Valid && tractce.Valid {
				censusTract := statefp.String + countyfp.String + tractce.String
				var tractSnap, pop2010 sql.NullFloat64
				errSnap := a.DB.QueryRow(`SELECT "TractSNAP", "Pop2010" FROM "Food_Access_Research_Atlas_Data_2019" WHERE CAST("CensusTract" AS TEXT) = $1 LIMIT 1`, censusTract).Scan(&tractSnap, &pop2010)
				if errSnap == nil && tractSnap.Valid && pop2010.Valid && pop2010.Float64 > 0 {
					// TractSNAP represents HHs. Approximation relative to population size
					snapRate = (tractSnap.Float64 * 2.5) / pop2010.Float64
					logs = append(logs, fmt.Sprintf("RESULT: Extracted SNAP data for Tract %s. Rate: %.2f%%", censusTract, snapRate*100))
				} else {
					logs = append(logs, fmt.Sprintf("WARN: Could not link Census Tract %s to FARA dataset for SNAP rate.", censusTract))
				}
			}

		} else {
			logs = append(logs, fmt.Sprintf("WARN: DB Demographics Query Failed (%v). Core values relying on SD Spatial Model.", errDemo))
			demo.Source = "SD Spatial Model Approximation"
		}

		// I3 (Closure Rate) - Querying local sd_active_businesses
		logs = append(logs, "CMD: Querying sd_active_businesses to compute localized I3 Closure Rate...")
		var activeBiz, inactiveBiz int
		rows, errClosure := a.DB.Query(`
			SELECT status FROM sd_active_businesses 
			WHERE lat IS NOT NULL AND lng IS NOT NULL 
			AND ST_DWithin(ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 2000)
		`, lng, lat)
		if errClosure == nil {
			defer rows.Close()
			for rows.Next() {
				var status string
				if err := rows.Scan(&status); err == nil {
					if strings.Contains(strings.ToUpper(status), "INACTIVE") || strings.Contains(strings.ToUpper(status), "CLOSED") {
						inactiveBiz++
					} else {
						activeBiz++
					}
				}
			}
			if (activeBiz + inactiveBiz) > 10 {
				closureRate = float64(inactiveBiz) / float64(activeBiz+inactiveBiz)
				logs = append(logs, fmt.Sprintf("RESULT: Evaluated %d local businesses. Inactive ratio (I3 Closure Rate) = %.2f", (activeBiz+inactiveBiz), closureRate))
			} else {
				logs = append(logs, "WARN: Not enough local businesses to establish a statistically significant closure rate. Using regional mock (0.38).")
			}
		}

		// P1 (Food Desert)
		var dbDesert sql.NullBool
		errFE := a.DB.QueryRow(`SELECT is_food_desert_usda FROM nourish_cbg_food_environment ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1`, lng, lat).Scan(&dbDesert)
		if errFE == nil && dbDesert.Valid {
			isFoodDesert = dbDesert.Bool
		}

		// L7 (Transit)
		a.DB.QueryRow(`SELECT COUNT(*) FROM sd_transit_stops WHERE ST_DWithin(ST_Transform(geom, 4326)::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 800)`, lng, lat).Scan(&transitStops)

		// P4/L12 (Shared Kitchens)
		var kitchenCount int
		a.DB.QueryRow(`SELECT COUNT(*) FROM nourish_comm_commissary_ext WHERE ST_DWithin(geom::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 8000)`, lng, lat).Scan(&kitchenCount)
		hasSharedKitchen = kitchenCount > 0

		// L4 (Commercial Rent)
		var dbRent sql.NullFloat64
		a.DB.QueryRow(`SELECT avg_rent_cost FROM esri_consumer_spending_data_ ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1`, lng, lat).Scan(&dbRent)
		if dbRent.Valid && dbRent.Float64 > 10.0 {
			rentEst = dbRent.Float64
		}

		// P6/L10 (Zoning)
		var dbZone sql.NullString
		a.DB.QueryRow(`SELECT zone_name FROM sandag_layer_zoning_base_sd_new ORDER BY ST_Transform(geom, 4326) <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1`, lng, lat).Scan(&dbZone)
		if dbZone.Valid {
			if strings.Contains(strings.ToLower(dbZone.String), "commercial") || strings.Contains(strings.ToLower(dbZone.String), "mixed") {
				isCommercialZone = true
			}
		}

	} else {
		assumptions = append(assumptions, "Database offline. Utilizing mathematical SD spatial heuristics.")
		logs = append(logs, "WARN: Database entirely offline. Injecting absolute base metrics.")
	}

	// FORCE POPULATE DEMOGRAPHICS (Fixes "N/A" in Frontend)
	demo.IncomeLevel = &mhi
	demo.PopulationDensity = &popDensity
	demo.DaytimePopulation = &emp

	nightPop := popDensity * 1.1 // Proxy
	demo.NighttimePopulation = &nightPop
	demo.GentrificationIndicator = &gentrification
	demo.TargetPopulationGrowth = &popGrowth
	demo.FoodDesertStatus = isFoodDesert
	demo.TransitStopsWithinWalk = &transitStops
	demo.RetailSpendingPotential = &wealthIndex

	costs.EstimatedRent = &rentEst
	utilEst := rentEst * 15.0 // Proxy monthly utilities
	costs.EstimatedUtilities = &utilEst
	laborEst := 30.0
	costs.LaborCostPct = &laborEst
	mktgEst := 4.0
	costs.MarketingPct = &mktgEst

	// 2. What-If Overrides
	if config.Overrides.IncomeLevel != nil {
		mhi = *config.Overrides.IncomeLevel
		demo.IncomeLevel = config.Overrides.IncomeLevel
		logs = append(logs, fmt.Sprintf("OVERRIDE: Applied custom Income Level: $%.2f", mhi))
	}
	if config.Overrides.Rent != nil {
		rentEst = *config.Overrides.Rent
		costs.EstimatedRent = config.Overrides.Rent
		logs = append(logs, fmt.Sprintf("OVERRIDE: Applied custom Commercial Rent: $%.2f/sqft", rentEst))
	}
	if config.Overrides.SharedKitchenAccess {
		hasSharedKitchen = true
	}

	startupCost := 150000.0
	if config.Overrides.StartupCosts != nil {
		startupCost = *config.Overrides.StartupCosts
		costs.StartupCosts = config.Overrides.StartupCosts
	} else {
		costs.StartupCosts = &startupCost
	}

	// 3. Dynamic Local Points (Foot Traffic Walkability L6, Competitors M9)
	logs = append(logs, "CMD: Executing radial scan of local datasets for Competitors & Traffic...")
	compCount, suppCount, reviewSum, popCount, resCount := 0, 0, 0, 0, 0
	avgRating, avgPrice := 0.0, 0.0
	ratingSum, priceSum, priceCount := 0.0, 0, 0

	radiusSq := 0.001
	if config.ComputationMethod == "boutique" {
		radiusSq = 0.01
	}

	keywords := []string{}
	if config.Keyword != "" {
		keywords = append(keywords, strings.Split(strings.ToLower(config.Keyword), ",")...)
	}

	for _, gm := range gmData {
		dLat, dLng := gm.Lat-lat, gm.Lng-lng
		distSq := dLat*dLat + dLng*dLng

		if distSq < radiusSq {
			catLower := strings.ToLower(gm.Category)
			isCompetitor := false
			isSupportive := false

			for _, kw := range keywords {
				if strings.Contains(catLower, strings.TrimSpace(kw)) {
					isCompetitor = true
					break
				}
			}

			if strings.Contains(catLower, "restaurant") || strings.Contains(catLower, "food") {
				if !isCompetitor {
					isSupportive = true
				}
			}

			openDuringTarget := isLikelyOpen(gm.OpenHours, config.TargetTime)

			if isCompetitor && openDuringTarget {
				compCount++
				ratingSum += gm.Rating
				if gm.PriceRange > 0 {
					priceSum += gm.PriceRange
					priceCount++
				}
				if gm.Reservations {
					resCount++
				}
			} else if isSupportive && openDuringTarget {
				suppCount++
			} else if openDuringTarget {
				reviewSum += gm.ReviewCount
				if gm.HasPopTimes {
					popCount++
				}
			}
		}
	}

	if compCount > 0 {
		avgRating = ratingSum / float64(compCount)
	}
	if priceCount > 0 {
		avgPrice = float64(priceSum) / float64(priceCount)
	}
	if config.Overrides.FootTraffic != nil {
		reviewSum = *config.Overrides.FootTraffic
		logs = append(logs, fmt.Sprintf("OVERRIDE: Applied custom Foot Traffic: %d", reviewSum))
	}

	logs = append(logs, fmt.Sprintf("RESULT: Found %d Competitors, %d Supporters, %d Traffic Index", compCount, suppCount, reviewSum))

	// 4. Compute Normalized Scope Variables (0.05 to 1.0 bounds)

	// Location (L)
	xL1 := normalize(popDensity, 0, 15000, true)
	logs = append(logs, fmt.Sprintf("MATH (L1 - Pop Density): Raw = %.1f/sqmi. Scaled [0-15k] -> Z-Score = %.2f. Favorable for walk-in demand.", popDensity, xL1))

	xL2 := normalize(mhi, 30000, 150000, true)
	logs = append(logs, fmt.Sprintf("MATH (L2 - MHI): Raw = $%.1f. Scaled [30k-150k] -> Z-Score = %.2f. Represents local purchasing power.", mhi, xL2))

	xL3 := normalize(unemp, 2, 15, false) // Inverse Poverty/Unemp
	logs = append(logs, fmt.Sprintf("MATH (L3 - Unemployment): Raw = %.1f%%. Inverted scale [2%%-15%%] -> Z-Score = %.2f. Represents constrained discretionary spending.", unemp, xL3))

	xL4 := normalize(rentEst, 20, 100, false) // Inverse Rent
	logs = append(logs, fmt.Sprintf("MATH (L4 - Rent): Raw = $%.2f/sqft. Inverted scale [20-100] -> Z-Score = %.2f. Represents primary fixed cost constraint.", rentEst, xL4))

	xL6 := normalize(float64(reviewSum), 0, 5000, true)
	logs = append(logs, fmt.Sprintf("MATH (L6 - Walkability): Raw Proxy = %d. Scaled [0-5000] -> Z-Score = %.2f. Approximates local pedestrian activity.", reviewSum, xL6))

	xL7 := normalize(float64(transitStops), 0, 15, true)
	logs = append(logs, fmt.Sprintf("MATH (L7 - Transit): Raw Stops = %d. Scaled [0-15] -> Z-Score = %.2f. Favorable for worker/customer mobility.", transitStops, xL7))

	xL9 := normalize(crime, 50, 300, false) // Inverse Crime
	logs = append(logs, fmt.Sprintf("MATH (L9 - Crime Rate): Raw Index = %.1f. Inverted scale [50-300] -> Z-Score = %.2f. Accounts for operational friction.", crime, xL9))

	xL12 := 0.2
	if hasSharedKitchen {
		xL12 = 1.0
	}
	logs = append(logs, fmt.Sprintf("MATH (L12 - Enabling Facilities): Kitchen Access = %v -> Z-Score = %.2f. Reduces CAPEX barriers.", hasSharedKitchen, xL12))

	// Market (M)
	xM1 := normalize(wealthIndex, 50, 200, true)
	logs = append(logs, fmt.Sprintf("MATH (M1 - Food Spend Idx): Raw = %.1f. Scaled [50-200] -> Z-Score = %.2f. Propensity to spend on food services.", wealthIndex, xM1))

	xM3 := normalize(snapRate, 0, 0.30, true) // SNAP Participation
	logs = append(logs, fmt.Sprintf("MATH (M3 - SNAP Rate): Raw = %.1f%%. Scaled [0-30%%] -> Z-Score = %.2f. Evaluates alignment for discount/grocery models.", snapRate*100, xM3))

	xM4 := normalize(avgIncome, 20000, 100000, true)
	logs = append(logs, fmt.Sprintf("MATH (M4 - Per-Capita Income): Raw = $%.1f. Scaled [20k-100k] -> Z-Score = %.2f. Determines individual pricing flexibility.", avgIncome, xM4))

	xM6 := normalize(emp, 500, 10000, true)
	logs = append(logs, fmt.Sprintf("MATH (M6 - Daytime Pop): Raw = %.1f. Scaled [500-10k] -> Z-Score = %.2f. Indicates commercial hubs for lunch/commuter volume.", emp, xM6))

	xM14 := normalize(0.55, 0, 1, false) // Mock Supply Chain
	xM15 := normalize(0.60, 0, 1, true)  // Mock Trend

	// Industry (I)
	xI1 := normalize(float64(compCount), 0, 30, false)
	logs = append(logs, fmt.Sprintf("MATH (I1 - Competition Density): Local Count = %d. Inverted scale [0-30] -> Z-Score = %.2f. Heavy penalty for market saturation.", compCount, xI1))

	xI2 := normalize(float64(suppCount), 0, 20, true)
	logs = append(logs, fmt.Sprintf("MATH (I2 - Supportive Biz): Local Count = %d. Scaled [0-20] -> Z-Score = %.2f. Represents positive agglomeration economics.", suppCount, xI2))

	xI3 := normalize(closureRate, 0.10, 0.60, false)
	logs = append(logs, fmt.Sprintf("MATH (I3 - Closure Rate): Local Ratio = %.2f. Inverted scale [10%%-60%%] -> Z-Score = %.2f. Historical failure friction.", closureRate, xI3))

	xI5 := normalize(0.50, 0, 1, false) // Mock Volatility

	// Policy (P)
	xP1 := 0.5
	if isFoodDesert {
		xP1 = 1.0
	}
	logs = append(logs, fmt.Sprintf("MATH (P1 - Food Desert): Status = %v -> Z-Score = %.2f. Boosts locations eligible for USDA/food equity grants.", isFoodDesert, xP1))

	xP4 := xL12
	xP6 := 0.2
	if isCommercialZone {
		xP6 = 1.0
	}
	logs = append(logs, fmt.Sprintf("MATH (P6 - Zoning Flex): Commercial Status = %v -> Z-Score = %.2f. Pure legal feasibility constraint.", isCommercialZone, xP6))

	xP7 := normalize(0.45, 0, 1, false) // Mock Permit Burden
	logs = append(logs, fmt.Sprintf("MATH (P7 - Permit Burden): Proxied value 0.45. Inverted scale -> Z-Score = %.2f.", xP7))

	// Temporal (T)
	xT1 := 1.0
	if compCount == 0 && suppCount == 0 && reviewSum < 50 {
		xT1 = 0.5
	}
	logs = append(logs, fmt.Sprintf("MATH (T1 - Seasonal Demand): Evaluated dead-zone overlap -> Z-Score = %.2f.", xT1))

	xT3 := normalize(0.70, 0, 1, false)
	xT5 := normalize(0.40, 0, 1, false) // Wage Inflation proxy
	logs = append(logs, fmt.Sprintf("MATH (T5 - Wage Inflation): Proxied value 0.40. Inverted scale -> Z-Score = %.2f.", xT5))

	// 5. Multiplicative Geometric Fusion of Scopes
	sL := geomMean([]float64{xL1, xL2, xL3, xL4, xL6, xL7, xL9, xL12},
		[]float64{1.0, 1.0, 1.0, config.CostPenaltyWeight, config.TrafficWeight, config.TransitBonusWeight, 1.0, 1.0})

	sM := geomMean([]float64{xM1, xM3, xM4, xM6, xM14, xM15}, []float64{1.0, 1.0, 1.0, 1.0, 0.5, 0.5})
	sI := geomMean([]float64{xI1, xI2, xI3, xI5}, []float64{config.CompPenaltyWeight, config.SuppBonusWeight, 1.0, 1.0})
	sP := geomMean([]float64{xP1, xP4, xP6, xP7}, []float64{config.FoodDesertBonus, 1.0, 2.0, 1.0})
	sT := geomMean([]float64{xT1, xT3, xT5}, []float64{1.0, 0.5, 1.0})

	logs = append(logs, fmt.Sprintf("FUSION: Computed Scope Means: SL=%.3f, SM=%.3f, SI=%.3f, SP=%.3f, ST=%.3f", sL, sM, sI, sP, sT))

	sStruct := geomMean([]float64{sL, sM, sI, sP, sT}, []float64{0.30, 0.25, 0.20, 0.15, 0.10})
	logs = append(logs, fmt.Sprintf("FUSION: Structural Sstruct = geomMean(SL, SM, SI, SP, ST) = %.3f", sStruct))

	// 6. Entrepreneur Feasibility Modifier (ME)
	deltaCap, deltaExp := 0.0, 0.0
	if config.Overrides.AvailableCapital != nil {
		if *config.Overrides.AvailableCapital < (startupCost * 0.5) {
			deltaCap = -0.25
			assumptions = append(assumptions, "What-If Engine: Significant capital deficit applied to entrepreneur multiplier (ME).")
		} else if *config.Overrides.AvailableCapital >= startupCost {
			deltaCap = +0.10
		}
	}
	if config.Overrides.BusinessExperience != nil && *config.Overrides.BusinessExperience > 3 {
		deltaExp = +0.10
	}

	ME := (1.0 + deltaCap) * (1.0 + deltaExp)
	ME = clip(ME, 0.75, 1.20) // Bounded personal influence
	logs = append(logs, fmt.Sprintf("MATH (ME): deltaCapital=%.2f, deltaExperience=%.2f -> ME=%.3f. Acts as a strict post-fusion multiplier.", deltaCap, deltaExp, ME))

	// 7. Final Output Score
	sFinal := clip(sStruct*ME, 0.0, 1.0)

	// Tuned Multiplier (x120 instead of x150) so 0.6 averages output realistically around ~70-80
	finalScore := int(clip(sFinal*100.0*1.2, 10, 98))
	logs = append(logs, fmt.Sprintf("FINAL: Opportunity = clip(Sstruct * ME) * 120 = %d", finalScore))

	// Breakdown Components for the UI Trace
	breakdown = append(breakdown, ScoreComponent{
		Name: "Location Scope (SL)", RawValue: sL, Weight: 0.30, Contribution: sL * 30,
		Impact: "Neutral", Explanation: "Aggregates Population Density, Income, Rent, Transit, Walkability, and Crime.", Expectation: "Higher values indicate a stronger baseline spatial environment.",
	})
	breakdown = append(breakdown, ScoreComponent{
		Name: "Market Scope (SM)", RawValue: sM, Weight: 0.25, Contribution: sM * 25,
		Impact: "Neutral", Explanation: "Aggregates Wealth Index, Per-Capita Income, and Daytime Population.", Expectation: "Higher values imply greater local purchasing power and demand.",
	})
	breakdown = append(breakdown, ScoreComponent{
		Name: "Industry Scope (SI)", RawValue: sI, Weight: 0.20, Contribution: sI * 20,
		Impact: "Neutral", Explanation: "Measures local market saturation, competitor penalization, and supportive business clustering.", Expectation: "Low saturation yields a higher industry score.",
	})
	breakdown = append(breakdown, ScoreComponent{
		Name: "Policy Scope (SP)", RawValue: sP, Weight: 0.15, Contribution: sP * 15,
		Impact: "Positive", Explanation: "Calculates offsets for Food Deserts, Shared Kitchen proximity, and Commercial Zoning allowances.", Expectation: "Boosts locations where public programs/zoning support the use.",
	})
	if ME != 1.0 {
		breakdown = append(breakdown, ScoreComponent{
			Name: "Entrepreneur Feasibility (ME)", RawValue: ME, Weight: 1.0, Contribution: (ME - 1.0) * 100,
			Impact: "Positive", Explanation: "Multiplicative modifier based on Entrepreneur Capital, Experience, and Network.", Expectation: "Acts as a scaling factor on the structural opportunity.",
		})
	}

	return finalScore, avgPrice, reviewSum, compCount, suppCount, avgRating, popCount, resCount, breakdown, demo, costs, assumptions, logs
}
