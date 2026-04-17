package main

import (
	"database/sql"
)

type App struct {
	DB *sql.DB
}

type ChatRequest struct {
	Message  string  `json:"message"`
	ApiKey   string  `json:"apiKey"`
	Model    string  `json:"model"`
	Provider string  `json:"provider"`
	BaseUrl  string  `json:"baseUrl"`
	N        float64 `json:"n"`
	S        float64 `json:"s"`
	E        float64 `json:"e"`
	W        float64 `json:"w"`
}

type RawStats struct {
	FootTraffic   int     `json:"footTraffic"`
	Competitors   int     `json:"competitors"`
	Supporters    int     `json:"supporters"`
	AveragePrice  float64 `json:"averagePrice"`
	AverageRating float64 `json:"averageRating"`
}

type MapPoint struct {
	Lat       float64          `json:"lat"`
	Lng       float64          `json:"lng"`
	Score     int              `json:"score"`
	Name      string           `json:"name"`
	Type      string           `json:"type"`
	RawStats  RawStats         `json:"rawStats"`
	Breakdown[]ScoreComponent `json:"breakdown"`
}

type MapConfigResponse struct {
	Reply           string     `json:"reply"`
	ActiveLayers[]string   `json:"activeLayers"`
	FocusArea       string     `json:"focusArea"`
	ZoomLevel       int        `json:"zoomLevel"`
	MapPoints[]MapPoint `json:"mapPoints"`
	ActiveWorkspace string     `json:"activeWorkspace"`
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

	AvailableCapital     *float64 `json:"availableCapital"`
	BusinessExperience   *int     `json:"businessExperience"`
	SharedKitchenAccess  bool     `json:"sharedKitchenAccess"`
}

type ScoreConfig struct {
	UseFootTraffic      bool
	UseCosts            bool
	UseCompetitors      bool
	AllowApproximations bool
	BusinessType        string
	Keyword             string
	ComputationMethod   string
	TargetTime          string

	TrafficWeight        float64
	CompPenaltyWeight    float64
	SuppBonusWeight      float64
	CostPenaltyWeight    float64
	RatingBonusWeight    float64
	FoodDesertBonus      float64
	GentrificationWeight float64
	TransitBonusWeight   float64

	Overrides ScoreOverrides
}

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
	PopulationDensity       *float64 `json:"populationDensity"`
	TransitStopsWithinWalk  *int     `json:"transitStopsWithinWalk"`
	RetailSpendingPotential *float64 `json:"retailSpendingPotential"`
	Source                  string   `json:"source"`
}

type ScoreComponent struct {
	Name         string  `json:"name"`
	RawValue     float64 `json:"rawValue"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
	Impact       string  `json:"impact"`
	Explanation  string  `json:"explanation"`
	Expectation  string  `json:"expectation"`
}

type MetricDetail struct {
	RawValue float64 `json:"rawValue"`
	ZScore   float64 `json:"zScore"`
}

type OpportunityMetrics struct {
	L1_PopDensity          MetricDetail `json:"l1_popDensity"`
	L2_MedianIncome        MetricDetail `json:"l2_medianIncome"`
	L3_Unemployment        MetricDetail `json:"l3_unemployment"`
	L4_CommercialRent      MetricDetail `json:"l4_commercialRent"`
	L5_Education           MetricDetail `json:"l5_education"`
	L6_Walkability         MetricDetail `json:"l6_walkability"`
	L7_TransitAccess       MetricDetail `json:"l7_transitAccess"`
	L8_ZeroCarHH           MetricDetail `json:"l8_zeroCarHh"`
	L9_CrimeRate           MetricDetail `json:"l9_crimeRate"`
	L12_RentBurden         MetricDetail `json:"l12_rentBurden"`
	L14_HouseholdSize      MetricDetail `json:"l14_householdSize"`
	L13_Gentrification     MetricDetail `json:"l13_gentrification"`
	L12_EnablingFacilities MetricDetail `json:"l12_enablingFacilities"`
	M1_FoodSpendIdx        MetricDetail `json:"m1_foodSpendIdx"`
	M2_RSPI                MetricDetail `json:"m2_rspi"`
	M3_SNAPRate            MetricDetail `json:"m3_snapRate"`
	M4_PerCapitaIncome     MetricDetail `json:"m4_perCapitaIncome"`
	M6_DaytimePop          MetricDetail `json:"m6_daytimePop"`
	M9_DiversityIdx        MetricDetail `json:"m9_diversityIdx"`
	I1_AvgBusinessSize     MetricDetail `json:"i1_avgBusinessSize"`
	I2_SupportiveBiz       MetricDetail `json:"i2_supportiveBiz"`
	I3_ClosureRate         MetricDetail `json:"i3_closureRate"`
	I5_MarketConcentration MetricDetail `json:"i5_marketConcentration"`
	I13_SkillAvailability  MetricDetail `json:"i13_skillAvailability"`
	I15_SafetyBurden       MetricDetail `json:"i15_safetyBurden"`
	P1_FoodDesert          MetricDetail `json:"p1_foodDesert"`
	P5_AdvisoryAccess      MetricDetail `json:"p5_advisoryAccess"`
	P6_ZoningFlex          MetricDetail `json:"p6_zoningFlex"`
	P7_PermitBurden        MetricDetail `json:"p7_permitBurden"`
	T1_SeasonalDemand      MetricDetail `json:"t1_seasonalDemand"`
	T5_WageInflation       MetricDetail `json:"t5_wageInflation"`

	ScopeLocation        float64 `json:"scopeLocation"`
	ScopeMarket          float64 `json:"scopeMarket"`
	ScopeIndustry        float64 `json:"scopeIndustry"`
	ScopePolicy          float64 `json:"scopePolicy"`
	ScopeTemporal        float64 `json:"scopeTemporal"`
	StructuralScore      float64 `json:"structuralScore"`
	EntrepreneurModifier float64 `json:"entrepreneurModifier"`
}

type LocationEvalResponse struct {
	Lat                         float64             `json:"lat"`
	Lng                         float64             `json:"lng"`
	ResolvedAddress             string              `json:"resolvedAddress"`
	OpportunityScore            float64             `json:"opportunityScore"`
	FootTraffic                 *int                `json:"footTraffic"`
	FootTrafficSource           string              `json:"footTrafficSource"`
	IsApproximated              bool                `json:"isApproximated"`
	NearbyCompetitors           int                 `json:"nearbyCompetitors"`
	SupportiveBusinesses        int                 `json:"supportiveBusinesses"`
	Demographics                Demographics        `json:"demographics"`
	OperatingCosts              DetailedCosts       `json:"operatingCosts"`
	DemographicProfile          string              `json:"demographicProfile"`
	ReviewCount                 int                 `json:"reviewCount"`
	StatsExtra                  string              `json:"statsExtra"`
	CalcBreakdown[]ScoreComponent    `json:"calcBreakdown"`
	CitywideActiveTaxCompetitor int                 `json:"citywideActiveTaxCompetitor"`
	Assumptions                 []string            `json:"assumptions"`
	CalculationLogs[]string            `json:"calculationLogs"`
	Metrics                     OpportunityMetrics  `json:"metrics"`
}

type EvalRequest struct {
	Address             string         `json:"address"`
	Lat                 float64        `json:"lat"`
	Lng                 float64        `json:"lng"`
	N                   float64        `json:"n"`
	S                   float64        `json:"s"`
	E                   float64        `json:"e"`
	W                   float64        `json:"w"`
	UseFootTraffic      bool           `json:"useFootTraffic"`
	UseCosts            bool           `json:"useCosts"`
	UseCompetitors      bool           `json:"useCompetitors"`
	AllowApproximations bool           `json:"allowApproximations"`
	Naics               string         `json:"naics"`
	Keyword             string         `json:"keyword"`
	ComputationMethod   string         `json:"computationMethod"`
	TargetTime          string         `json:"targetTime"`
	TrafficW            float64        `json:"trafficW"`
	CompW               float64        `json:"compW"`
	SuppW               float64        `json:"suppW"`
	CostW               float64        `json:"costW"`
	RatingW             float64        `json:"ratingW"`
	FoodDesertW         float64        `json:"foodDesertW"`
	GentrificationW     float64        `json:"gentrificationW"`
	TransitW            float64        `json:"transitW"`
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
	TransitBonusWeight   float64 `json:"transitBonusWeight"`
}

type BestMatchResult struct {
	Lat             float64             `json:"lat"`
	Lng             float64             `json:"lng"`
	Name            string              `json:"name"`
	BusinessType    string              `json:"businessType"`
	BusinessName    string              `json:"businessName"`
	Score           int                 `json:"score"`
	StartupCosts    float64             `json:"startupCosts"`
	Rent            float64             `json:"rent"`
	RawStats        RawStats            `json:"rawStats"`
	Breakdown[]ScoreComponent    `json:"breakdown"`
	Demographics    Demographics        `json:"demographics"`
	OperatingCosts  DetailedCosts       `json:"operatingCosts"`
	CalculationLogs[]string            `json:"calculationLogs"`
	Assumptions[]string            `json:"assumptions"`
	Metrics         OpportunityMetrics  `json:"metrics"`
}

type BusinessRecommendation struct {
	Profile BusinessConfig `json:"profile"`
	Score   float64        `json:"score"`
	Details string         `json:"details"`
}

var BusinessProfiles =[]BusinessConfig{
	{NAICS: "445", Name: "Food and Beverage Stores (Grocery)", TrafficWeight: 1.5, CompPenaltyWeight: 10.0, SuppBonusWeight: 2.0, CostPenaltyWeight: 5.0, RatingBonusWeight: 0.0, FoodDesertBonus: 25.0, GentrificationWeight: -2.0, TransitBonusWeight: 3.0},
	{NAICS: "722", Name: "Food Services and Drinking Places", TrafficWeight: 2.0, CompPenaltyWeight: 8.0, SuppBonusWeight: 1.5, CostPenaltyWeight: 8.0, RatingBonusWeight: 15.0, FoodDesertBonus: 0.0, GentrificationWeight: 5.0, TransitBonusWeight: 4.0},
	{NAICS: "454", Name: "Nonstore Retailers (Food Trucks/Stands)", TrafficWeight: 3.0, CompPenaltyWeight: 5.0, SuppBonusWeight: 3.0, CostPenaltyWeight: 2.0, RatingBonusWeight: 10.0, FoodDesertBonus: 10.0, GentrificationWeight: 2.0, TransitBonusWeight: 5.0},
	{NAICS: "311811", Name: "Retail Bakeries / Home Kitchens", TrafficWeight: 2.0, CompPenaltyWeight: 6.0, SuppBonusWeight: 2.0, CostPenaltyWeight: 6.0, RatingBonusWeight: 12.0, FoodDesertBonus: 5.0, GentrificationWeight: 4.0, TransitBonusWeight: 2.0},
	{NAICS: "445110", Name: "Supermarkets (Healthy Grocery)", TrafficWeight: 1.5, CompPenaltyWeight: 12.0, SuppBonusWeight: 2.5, CostPenaltyWeight: 7.0, RatingBonusWeight: 5.0, FoodDesertBonus: 30.0, GentrificationWeight: -5.0, TransitBonusWeight: 3.0},
}

type NAICSMatrix struct {
	fR, fE, fL, fP, fReg, fT, fScale, fC, fS, fH, fW, fMech, fClim, fI float64
}

var IndustryMatrices = map[string]NAICSMatrix{
	"722110": {fR: 1.25, fL: 1.45, fE: 1.25, fP: 1.40, fReg: 1.35, fT: 1.20, fScale: 1.30},
	"722211": {fR: 1.20, fL: 1.35, fE: 1.20, fP: 1.25, fReg: 1.25, fT: 1.30, fScale: 1.20},
	"722330": {fR: 0.60, fL: 1.20, fE: 1.10, fP: 1.20, fReg: 1.25, fT: 1.35, fScale: 0.80},
	"445110": {fR: 1.20, fL: 1.30, fP: 1.35, fE: 1.25, fReg: 1.30, fC: 1.10, fScale: 1.40},
	"445291": {fR: 1.15, fL: 1.25, fP: 1.25, fE: 1.20, fReg: 1.20, fC: 1.10, fScale: 1.10},
	"311812": {fR: 1.10, fE: 1.25, fL: 1.20, fP: 1.20, fReg: 1.25, fS: 1.10, fScale: 1.10},
}

var GenericMatrixFallback = NAICSMatrix{fR: 1.0, fE: 1.0, fL: 1.0, fP: 1.0, fReg: 1.0, fT: 1.0, fScale: 1.0, fC: 1.0, fS: 1.0}
