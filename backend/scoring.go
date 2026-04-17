package main

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
)

func clip(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func normalize(val, min, max float64, positive bool) float64 {
	val = clip(val, min, max)
	if max <= min {
		return 0.5
	}
	z := (val - min) / (max - min)
	if !positive {
		z = 1.0 - z
	}
	return clip(z, 0.05, 1.0)
}

func geomMean(values[]float64, weights[]float64) float64 {
	sumWeights := 0.0
	sumLog := 0.0
	for i, v := range values {
		w := weights[i]
		if w <= 0 {
			w = 1.0
		}
		val := clip(v, 0.01, 1.0)
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

func (a *App) calculateOpportunityScore(lat, lng float64, config ScoreConfig) (int, float64, int, int, int, float64, int, int,[]ScoreComponent, Demographics, DetailedCosts, []string, []string, OpportunityMetrics) {
	var assumptions[]string
	var breakdown []ScoreComponent
	var logs[]string

	logs = append(logs, fmt.Sprintf("INIT: lat=%.5f, lng=%.5f", lat, lng))

	demo := Demographics{Source: "Database", FoodDesertStatus: false}
	costs := DetailedCosts{Source: "Database"}

	distDowntown := math.Sqrt(math.Pow(lat-32.715, 2) + math.Pow(lng - -117.16, 2))

	mhi, popDensity, rentEst, emp, unemp, gentrification, avgIncome, crime, diversityIdx := 82000.0, 4800.0, 32.0, 4000.0, 4.5, 2.8, 55000.0, 100.0, 50.0
	popGrowth := 1.2
	snapRate := 0.08
	closureRate := 0.38

	avgBusinessSize := 5.0
	walkabilityD3B := 80.0
	healthInspectionsCount := 0
	permitDays := 45.0

	povRate, educColl, zeroVeh, rentBurden, avgHhSz, occServ := 10.0, 30.0, 5.0, 35.0, 2.5, 15.0

	if lng < -117.20 {
		mhi, popDensity, rentEst, emp, unemp, gentrification, avgIncome, crime = 115000.0, 5500.0, 55.0, 3500.0, 3.2, 4.1, 85000.0, 60.0
		logs = append(logs, "INFO: Region matched Coastal; applied appropriate baselines.")
	} else if distDowntown < 0.05 {
		mhi, popDensity, rentEst, emp, unemp, gentrification, avgIncome, crime = 85000.0, 14000.0, 45.0, 12000.0, 4.8, 6.5, 65000.0, 140.0
		logs = append(logs, "INFO: Region matched Urban Core; applied appropriate baselines.")
	} else if lng > -117.0 {
		mhi, popDensity, rentEst, emp, unemp, gentrification, avgIncome, crime = 68000.0, 3800.0, 26.0, 1800.0, 6.1, 1.5, 45000.0, 90.0
		logs = append(logs, "INFO: Region matched East County; applied appropriate baselines.")
	} else {
		logs = append(logs, "INFO: Region matched Central; applied appropriate baselines.")
	}

	wealthIndex := 100.0
	transitStopsCount := 0
	advisoryCount := 0
	isFoodDesert := false
	hasSharedKitchen := false
	isCommercialZone := false

	var blockGroupID string

	if a.DB != nil {
		logs = append(logs, "INFO: DB query starting for L1-L3, M2, M4, M6, M9")

		var dbMhi, dbPop, dbEmp, dbWlth, dbUnemp, dbAvgia, dbDi sql.NullFloat64
		var statefp, countyfp, tractce, blkgrpce sql.NullString

		errDemo := a.DB.QueryRow(`
			SELECT b.medhinc_cy, 
			       (b.totpop_cy / NULLIF(ST_Area(ST_Transform(e.geom, 4326)::geography) / 2589988.11, 0)) AS pop_density,
			       b.emp_cy, b.wlthindxcy, b.unemprt_cy, b.avgia_cy, b.di_cy,
			       b.statefp, b.countyfp, b.tractce, b.blkgrpce
			FROM bgs_sd_imp b JOIN entity_blockgroup e ON b.ogc_fid = e.ogc_fid
			ORDER BY ST_Transform(e.geom, 4326) <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1
		`, lng, lat).Scan(&dbMhi, &dbPop, &dbEmp, &dbWlth, &dbUnemp, &dbAvgia, &dbDi, &statefp, &countyfp, &tractce, &blkgrpce)

		if errDemo == nil {
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
			if dbAvgia.Valid {
				avgIncome = dbAvgia.Float64
			}
			if dbDi.Valid {
				diversityIdx = dbDi.Float64
			}
			demo.Source = "PostGIS"

			if statefp.Valid && countyfp.Valid && tractce.Valid && blkgrpce.Valid {
				blockGroupID = statefp.String + countyfp.String + tractce.String + blkgrpce.String

				if val, exists := sldCache[blockGroupID]; exists {
					walkabilityD3B = val
					logs = append(logs, fmt.Sprintf("INFO: EPA SLD Walkability (D3B) for GEOID %s: %.1f", blockGroupID, walkabilityD3B))
				}

				if jobs, exists := wacCache[blockGroupID]; exists {
					emp = float64(jobs)
					logs = append(logs, fmt.Sprintf("INFO: LEHD LODES Daytime Jobs (C000) for GEOID %s: %d", blockGroupID, jobs))
				}

				censusTract := statefp.String + countyfp.String + tractce.String
				
				if val, exists := gentrificationCache[censusTract]; exists {
					gentrification = val * 100.0
					logs = append(logs, fmt.Sprintf("INFO: Gentrification Index for Tract %s: %.2f%%", censusTract, gentrification))
				}

				var tractSnap, pop2010 sql.NullFloat64
				errSnap := a.DB.QueryRow(`SELECT "TractSNAP", "Pop2010" FROM "Food_Access_Research_Atlas_Data_2019" WHERE CAST("CensusTract" AS TEXT) = $1 LIMIT 1`, censusTract).Scan(&tractSnap, &pop2010)
				if errSnap == nil && tractSnap.Valid && pop2010.Valid && pop2010.Float64 > 0 {
					snapRate = (tractSnap.Float64 * 2.5) / pop2010.Float64
					logs = append(logs, fmt.Sprintf("INFO: SNAP data for Tract %s: %.2f%%", censusTract, snapRate*100))
				}
			}
		} else {
			logs = append(logs, fmt.Sprintf("ERROR: DB Demographics Query Failed: %v", errDemo))
			demo.Source = "Spatial Model Approximation"
		}

		var dbEduc, dbZeroVeh, dbRentBurden, dbAvgHhSz, dbOccServ sql.NullFloat64
		errAdvDemo := a.DB.QueryRow(`
			SELECT b.educ_coll_cy, b.zero_vehicle_hh_cy, b.rent_burden_cy, b.avghhsz_cy, b.occ_serv_cy
			FROM bgs_sd_imp b JOIN entity_blockgroup e ON b.ogc_fid = e.ogc_fid
			ORDER BY ST_Transform(e.geom, 4326) <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1
		`, lng, lat).Scan(&dbEduc, &dbZeroVeh, &dbRentBurden, &dbAvgHhSz, &dbOccServ)

		if errAdvDemo == nil {
			if dbEduc.Valid {
				educColl = dbEduc.Float64
			}
			if dbZeroVeh.Valid {
				zeroVeh = dbZeroVeh.Float64
			}
			if dbRentBurden.Valid {
				rentBurden = dbRentBurden.Float64
			}
			if dbAvgHhSz.Valid {
				avgHhSz = dbAvgHhSz.Float64
			}
			if dbOccServ.Valid {
				occServ = dbOccServ.Float64
			}
		}

		naicsPrefix := "722"
		if config.BusinessType != "" && len(config.BusinessType) >= 3 {
			naicsPrefix = config.BusinessType[:3]
		}

		if rate, exists := bdsCache[naicsPrefix]; exists {
			closureRate = rate
		}
		if size, exists := cbpCache[naicsPrefix]; exists {
			avgBusinessSize = size
		}

		transitScoreProxy := 0.0
		for _, stop := range transitStops {
			if haversineDistSq(lat, lng, stop.Lat, stop.Lng) < 0.0001 {
				transitStopsCount++
				transitScoreProxy += float64(stop.Frequency) / 100.0
			}
		}

		for _, insp := range inspections {
			if haversineDistSq(lat, lng, insp.Lat, insp.Lng) < 0.00005 {
				healthInspectionsCount++
			}
		}

		var dbDesert sql.NullBool
		errFE := a.DB.QueryRow(`SELECT is_food_desert_usda FROM nourish_cbg_food_environment ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1`, lng, lat).Scan(&dbDesert)
		if errFE == nil && dbDesert.Valid {
			isFoodDesert = dbDesert.Bool
		}

		if transitStopsCount == 0 {
			a.DB.QueryRow(`SELECT COUNT(*) FROM sd_transit_stops WHERE ST_DWithin(ST_Transform(geom, 4326)::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 800)`, lng, lat).Scan(&transitStopsCount)
		}

		var kitchenCount int
		a.DB.QueryRow(`SELECT COUNT(*) FROM nourish_comm_commissary_ext WHERE ST_DWithin(geom::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 8000)`, lng, lat).Scan(&kitchenCount)
		hasSharedKitchen = kitchenCount > 0

		a.DB.QueryRow(`SELECT COUNT(*) FROM nourish_comm_service_provider WHERE ST_DWithin(geom::geography, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 8000)`, lng, lat).Scan(&advisoryCount)

		var dbRent sql.NullFloat64
		a.DB.QueryRow(`SELECT avg_rent_cost FROM esri_consumer_spending_data_ ORDER BY geom <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1`, lng, lat).Scan(&dbRent)
		if dbRent.Valid && dbRent.Float64 > 10.0 {
			rentEst = dbRent.Float64
		}

		var dbZone sql.NullString
		a.DB.QueryRow(`SELECT zone_name FROM sandag_layer_zoning_base_sd_new ORDER BY ST_Transform(geom, 4326) <-> ST_SetSRID(ST_MakePoint($1, $2), 4326) ASC LIMIT 1`, lng, lat).Scan(&dbZone)
		if dbZone.Valid {
			if strings.Contains(strings.ToLower(dbZone.String), "commercial") || strings.Contains(strings.ToLower(dbZone.String), "mixed") {
				isCommercialZone = true
			}
		}

	} else {
		assumptions = append(assumptions, "Database offline. Utilizing spatial heuristics.")
		logs = append(logs, "ERROR: Database offline.")
	}

	demo.IncomeLevel = &mhi
	demo.PopulationDensity = &popDensity
	demo.DaytimePopulation = &emp

	nightPop := popDensity * 1.1
	demo.NighttimePopulation = &nightPop
	demo.GentrificationIndicator = &gentrification
	demo.TargetPopulationGrowth = &popGrowth
	demo.FoodDesertStatus = isFoodDesert
	demo.TransitStopsWithinWalk = &transitStopsCount
	demo.RetailSpendingPotential = &wealthIndex

	costs.EstimatedRent = &rentEst
	utilEst := rentEst * 15.0
	costs.EstimatedUtilities = &utilEst
	laborEst := 30.0
	costs.LaborCostPct = &laborEst
	mktgEst := 4.0
	costs.MarketingPct = &mktgEst

	if config.Overrides.IncomeLevel != nil {
		mhi = *config.Overrides.IncomeLevel
		demo.IncomeLevel = config.Overrides.IncomeLevel
	}
	if config.Overrides.Rent != nil {
		rentEst = *config.Overrides.Rent
		costs.EstimatedRent = config.Overrides.Rent
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

	compCount, suppCount, reviewSum, popCount, resCount := 0, 0, 0, 0, 0
	avgRating, avgPrice := 0.0, 0.0
	ratingSum, priceSum, priceCount := 0.0, 0, 0

	radiusSq := 0.001
	if config.ComputationMethod == "boutique" {
		radiusSq = 0.01
	}

	keywords :=[]string{}
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
	}

	xL1 := normalize(popDensity, 0, 15000, true)
	xL2 := normalize(mhi, 30000, 150000, true)
	xL3 := normalize(unemp, 2, 15, false)
	xL4 := normalize(rentEst, 20, 100, false)
	xL5 := normalize(educColl, 10, 80, true)
	xL6 := normalize(walkabilityD3B, 10, 250, true)
	xL7 := normalize(float64(transitStopsCount), 0, 15, true)
	xL8 := normalize(zeroVeh, 0, 30, true)
	xL9 := normalize(crime, 50, 300, false)
	xL12_rb := normalize(rentBurden, 20, 60, false)
	xL14 := normalize(avgHhSz, 1.5, 4.5, true)
	xL13_gent := normalize(gentrification, -10.0, 40.0, config.GentrificationWeight >= 0)
	xL12 := 0.2
	if hasSharedKitchen {
		xL12 = 1.0
	}

	xM1 := normalize(wealthIndex, 50, 200, true)
	rspi := avgIncome * (1.0 - (povRate / 100.0)) * avgHhSz
	xM2 := normalize(rspi, 50000, 200000, true)
	xM3 := normalize(snapRate, 0, 0.30, true)
	xM4 := normalize(avgIncome, 20000, 100000, true)
	xM6 := normalize(emp, 500, 10000, true)
	xM9 := normalize(diversityIdx, 0, 100, true)
	xM14 := normalize(0.55, 0, 1, false)
	xM15 := normalize(0.60, 0, 1, true)

	xI1 := normalize(avgBusinessSize, 2, 30, false)
	xI2 := normalize(float64(suppCount), 0, 20, true)
	xI3 := normalize(closureRate, 0.05, 0.25, false)
	xI5 := normalize(float64(compCount), 0, 30, false)
	xI13 := normalize(occServ, 5, 40, true)
	xI15 := normalize(float64(healthInspectionsCount), 0, 50, false)

	xP1 := 0.5
	if isFoodDesert {
		xP1 = 1.0
	}
	xP4 := xL12
	xP5 := normalize(float64(advisoryCount), 0, 5, true)
	xP6 := 0.2
	if isCommercialZone {
		xP6 = 1.0
	}
	xP7 := normalize(permitDays, 15, 120, false)

	xT1 := 1.0
	if compCount == 0 && suppCount == 0 && reviewSum < 50 {
		xT1 = 0.5
	}
	xT3 := normalize(0.70, 0, 1, false)
	xT5 := normalize(0.40, 0, 1, false)

	sL := geomMean([]float64{xL1, xL2, xL3, xL4, xL5, xL6, xL7, xL8, xL9, xL12, xL12_rb, xL14, xL13_gent},[]float64{1.0, 1.0, 1.0, config.CostPenaltyWeight, 0.5, config.TrafficWeight, config.TransitBonusWeight, 0.5, 1.0, 1.0, 0.5, 0.5, math.Abs(config.GentrificationWeight)})
	sM := geomMean([]float64{xM1, xM2, xM3, xM4, xM6, xM9, xM14, xM15},[]float64{1.0, 1.0, 1.0, 1.0, 1.0, 0.5, 0.5, 0.5})
	sI := geomMean([]float64{xI1, xI2, xI3, xI5, xI13, xI15},[]float64{1.0, config.SuppBonusWeight, 1.0, config.CompPenaltyWeight, 0.5, 0.5})
	sP := geomMean([]float64{xP1, xP4, xP5, xP6, xP7},[]float64{config.FoodDesertBonus, 1.0, 0.5, 2.0, 1.0})
	sT := geomMean([]float64{xT1, xT3, xT5},[]float64{1.0, 0.5, 1.0})

	sStruct := geomMean([]float64{sL, sM, sI, sP, sT},[]float64{0.30, 0.25, 0.20, 0.15, 0.10})

	deltaCap, deltaExp := 0.0, 0.0
	if config.Overrides.AvailableCapital != nil {
		if *config.Overrides.AvailableCapital < (startupCost * 0.5) {
			deltaCap = -0.25
			assumptions = append(assumptions, "Significant capital deficit applied to entrepreneur multiplier.")
		} else if *config.Overrides.AvailableCapital >= startupCost {
			deltaCap = +0.10
		}
	}
	if config.Overrides.BusinessExperience != nil && *config.Overrides.BusinessExperience > 3 {
		deltaExp = +0.10
	}

	ME := (1.0 + deltaCap) * (1.0 + deltaExp)
	ME = clip(ME, 0.75, 1.20)

	sFinal := clip(sStruct*ME, 0.0, 1.0)
	finalScore := int(clip(sFinal*100.0*1.2, 10, 98))

	breakdown = append(breakdown, ScoreComponent{
		Name: "Location Scope", RawValue: sL, Weight: 0.30, Contribution: sL * 30,
		Impact: "Neutral", Explanation: "Aggregates Pop Density, Income, Rent, Transit, Walkability, Crime.", Expectation: "Higher values indicate stronger baseline environment.",
	})
	breakdown = append(breakdown, ScoreComponent{
		Name: "Market Scope", RawValue: sM, Weight: 0.25, Contribution: sM * 25,
		Impact: "Neutral", Explanation: "Aggregates Wealth Index, Income, Diversity, Daytime Pop.", Expectation: "Higher values imply greater purchasing power.",
	})
	breakdown = append(breakdown, ScoreComponent{
		Name: "Industry Scope", RawValue: sI, Weight: 0.20, Contribution: sI * 20,
		Impact: "Neutral", Explanation: "Measures market saturation and supportive business clusters.", Expectation: "Low saturation yields a higher score.",
	})
	breakdown = append(breakdown, ScoreComponent{
		Name: "Policy Scope", RawValue: sP, Weight: 0.15, Contribution: sP * 15,
		Impact: "Positive", Explanation: "Offsets for Food Deserts, Advisory access, Commercial Zoning.", Expectation: "Boosts locations with programmatic support.",
	})
	
	if config.GentrificationWeight != 0 {
		impactStr := "Positive"
		if config.GentrificationWeight < 0 {
			impactStr = "Negative"
		}
		breakdown = append(breakdown, ScoreComponent{
			Name: "Gentrification Index Offset", RawValue: gentrification, Weight: math.Abs(config.GentrificationWeight),
			Contribution: xL13_gent * math.Abs(config.GentrificationWeight) * 10,
			Impact: impactStr, Explanation: "Rank-delta of local income and rent increases.",
			Expectation: "Weight defines priority for or against gentrifying areas.",
		})
	}
	
	if ME != 1.0 {
		breakdown = append(breakdown, ScoreComponent{
			Name: "Entrepreneur Feasibility (ME)", RawValue: ME, Weight: 1.0, Contribution: (ME - 1.0) * 100,
			Impact: "Positive", Explanation: "Modifier based on Entrepreneur Capital, Experience, and Network.", Expectation: "Scales the structural opportunity.",
		})
	}

	metrics := OpportunityMetrics{
		L1_PopDensity:          MetricDetail{RawValue: popDensity, ZScore: xL1},
		L2_MedianIncome:        MetricDetail{RawValue: mhi, ZScore: xL2},
		L3_Unemployment:        MetricDetail{RawValue: unemp, ZScore: xL3},
		L4_CommercialRent:      MetricDetail{RawValue: rentEst, ZScore: xL4},
		L5_Education:           MetricDetail{RawValue: educColl, ZScore: xL5},
		L6_Walkability:         MetricDetail{RawValue: walkabilityD3B, ZScore: xL6},
		L7_TransitAccess:       MetricDetail{RawValue: float64(transitStopsCount), ZScore: xL7},
		L8_ZeroCarHH:           MetricDetail{RawValue: zeroVeh, ZScore: xL8},
		L9_CrimeRate:           MetricDetail{RawValue: crime, ZScore: xL9},
		L12_RentBurden:         MetricDetail{RawValue: rentBurden, ZScore: xL12_rb},
		L14_HouseholdSize:      MetricDetail{RawValue: avgHhSz, ZScore: xL14},
		L13_Gentrification:     MetricDetail{RawValue: gentrification, ZScore: xL13_gent},
		L12_EnablingFacilities: MetricDetail{RawValue: boolToFloat(hasSharedKitchen), ZScore: xL12},
		M1_FoodSpendIdx:        MetricDetail{RawValue: wealthIndex, ZScore: xM1},
		M2_RSPI:                MetricDetail{RawValue: rspi, ZScore: xM2},
		M3_SNAPRate:            MetricDetail{RawValue: snapRate * 100, ZScore: xM3},
		M4_PerCapitaIncome:     MetricDetail{RawValue: avgIncome, ZScore: xM4},
		M6_DaytimePop:          MetricDetail{RawValue: emp, ZScore: xM6},
		M9_DiversityIdx:        MetricDetail{RawValue: diversityIdx, ZScore: xM9},
		I1_AvgBusinessSize:     MetricDetail{RawValue: avgBusinessSize, ZScore: xI1},
		I2_SupportiveBiz:       MetricDetail{RawValue: float64(suppCount), ZScore: xI2},
		I3_ClosureRate:         MetricDetail{RawValue: closureRate, ZScore: xI3},
		I5_MarketConcentration: MetricDetail{RawValue: float64(compCount), ZScore: xI5},
		I13_SkillAvailability:  MetricDetail{RawValue: occServ, ZScore: xI13},
		I15_SafetyBurden:       MetricDetail{RawValue: float64(healthInspectionsCount), ZScore: xI15},
		P1_FoodDesert:          MetricDetail{RawValue: boolToFloat(isFoodDesert), ZScore: xP1},
		P5_AdvisoryAccess:      MetricDetail{RawValue: float64(advisoryCount), ZScore: xP5},
		P6_ZoningFlex:          MetricDetail{RawValue: boolToFloat(isCommercialZone), ZScore: xP6},
		P7_PermitBurden:        MetricDetail{RawValue: permitDays, ZScore: xP7},
		T1_SeasonalDemand:      MetricDetail{RawValue: 0, ZScore: xT1},
		T5_WageInflation:       MetricDetail{RawValue: 0.40, ZScore: xT5},

		ScopeLocation:        sL,
		ScopeMarket:          sM,
		ScopeIndustry:        sI,
		ScopePolicy:          sP,
		ScopeTemporal:        sT,
		StructuralScore:      sStruct,
		EntrepreneurModifier: ME,
	}

	return finalScore, avgPrice, reviewSum, compCount, suppCount, avgRating, popCount, resCount, breakdown, demo, costs, assumptions, logs, metrics
}
