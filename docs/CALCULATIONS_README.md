# NOURISH PT

This document provides a comprehensive, in-depth explanation of the Nourish PT evaluation engine. It details the exact mathematical models currently in production, the rationale behind these specific calculations, how to read the calculation trace logs, and an exhaustive roadmap of everything remaining to achieve the full 55-indicator opportunity ontology.

---

## 1. The Mathematical Foundation

The Opportunity Map engine relies on an advanced **Multiplicative Geometric Fusion Model**. 

The final score is factorized cleanly into two logical components:
**`Sfinal(b, c, e, t) = clip(Sstruct(b, c, t) * ME(e, c))`**

### 1a. Normalization to [0.05, 1.0] via Min-Max Scaling (The Z_v Vector)
Before variables can be geometrically combined, they are converted into a `Z_v` scalar using a bounding box `[min, max]` inside the `normalize()` function in Go. 
* If a variable is **favorable** (Income, Density, Walkability), `Z = (val - min) / (max - min)`.
* If a variable is **unfavorable** (Crime, Rent, Competitors, Closure Rates), it is inverted: `Z = 1.0 - ((val - min) / (max - min))`.

*Note: The normalization function explicitly bounds the lowest value to `0.05` instead of `0.0`. This is to prevent a single absolute zero from turning the geometric mean to `log(0)`, crashing the mathematical trace.*

### 1b. Why Geometric Aggregation for `Sstruct`?
The structural environment (`Sstruct`) represents "how good the world is" for the business, ignoring the entrepreneur entirely. It is computed as a weighted geometric mean of five scopes: Location ($S_L$), Market ($S_M$), Industry ($S_I$), Policy ($S_P$), and Temporal ($S_T$).

**Mathematical Reason:** Arithmetic addition allows a massive surplus in one area (e.g., highly accessible transit) to mathematically hide a critical failure in another (e.g., exorbitant rent or zero foot traffic). Geometric aggregation enforces **bottleneck behavior**—if an area has a near-zero score in a critical variable, the entire scope is heavily depressed. Weak indicators act as a multiplier constraint, making the scoring highly resilient and realistic.

### 1c. Why Multiplicative Separation for `ME`?
The Entrepreneur Feasibility Modifier (`ME`) represents the *readiness* of the founder (Capital, Experience, Certifications). 
**Mathematical Reason:** If we added the founder's experience directly into the structural fusion, the math would imply that "having more experience reduces the crime rate or lowers rent." By applying `ME` as a strict external multiplier (bounded to `[0.75, 1.20]`), the structural facts of the world remain static, while the entrepreneur's *ability to capture that opportunity* is scaled proportionally. Calculated as: `ME = (1 + δ_capital) * (1 + δ_experience) * ...`

---

## 2. Interpreting the Calculation Logs

In the Application UI, expanding the **`>_ Execution & Math Logs`** tab exposes exactly how the engine translates spatial databases into the final score.

A typical intermediate log looks like this:
`MATH [L2 - MHI]: Raw=$82000. Bounds:[30k, 150k]. Formula: (82000 - 30k)/120k = 0.43. Represents local purchasing power.`
*   **Raw:** The exact value pulled from the database query (or estimated proxy).
*   **Bounds:** The specific `[min, max]` constraints used to clip extreme outliers.
*   **Formula:** The mathematical calculation proving how the `Raw` was turned into a normalized `0.43` (meaning it represents 43% of the ideal condition).

Inverted logs (for negative constraints like Competition or Crime) will explicitly state `INVERTED Formula: 1.0 - (...)` to show how high crime results in a low Z-Score.

---

## 3. Implemented Indicators

The backend normalizes every raw value into the `[0.05, 1.0]` vector before mathematically fusing them. The following variables are fully actively implemented:

### Location Scope ($S_L$) - Weight: 30%
| Ind. | Variable | Backend Implementation | Mathematical Reasoning |
|---|---|---|---|
| **L1** | Pop Density | `bgs_sd_imp.totpop_cy / ST_Area(...)` | Captures the absolute volume of potential walk-in customers. Capped at 15,000/sqmi. |
| **L2** | Median HH Income | `bgs_sd_imp.medhinc_cy` | Purchasing power. Capped at $150k as marginal food spending diminishes at extreme wealth. |
| **L3** | Unemployment | `bgs_sd_imp.unemprt_cy` | Proxy for poverty and constrained discretionary spending. Inverted scale: higher unemployment severely lowers the score. |
| **L4** | Commercial Rent | `esri_consumer_spending...avg_rent_cost` | Primary fixed cost constraint. Modeled inversely; rent approaching $100/sqft depresses the metric. |
| **L6** | Walkability | `reviewSum` from local `gmData` points | Proxy for pedestrian traffic. GM reviews provide an immediate local foot-traffic density proxy. |
| **L7** | Transit Access | `sd_transit_stops` mapped via `ST_DWithin(800m)` | Buffer counts stops within an 800m radius. Improves worker/customer mobility. |
| **L9** | Crime Rate | `bgs_sd_imp.crm_cy` | Operating friction and safety risk. Inverted constraint capped at an index of 300. |
| **L12**| Shared Kitchen | `nourish_comm_commissary_ext` `ST_DWithin(8000m)`| Same as P4. Closer facilities lower startup capital requirements. |

### Market Scope ($S_M$) - Weight: 25%
| Ind. | Variable | Backend Implementation | Mathematical Reasoning |
|---|---|---|---|
| **M1/M2**| Food/Retail Spend | `bgs_sd_imp.wlthindxcy` | Captures readiness to spend on retail vs housing/savings. |
| **M3** | SNAP Participation | `Food_Access_Research_Atlas_Data_2019` | Joined via `CensusTract`. Indicates demand for discount grocery and affordability constraints. |
| **M4** | Per-Capita Income | `bgs_sd_imp.avgia_cy` | Secondary income metric normalizing MHI by headcount to determine individual pricing flexibility. |
| **M6** | Daytime Pop Ratio| `bgs_sd_imp.emp_cy` | Total employment in the block group. Identifies commercial/commuter hubs essential for lunch-hour viability. |
| **M9** | Consumer Diversity | `bgs_sd_imp.di_cy` | Diversity Index mapping indicates higher likelihood of cultural food demand and niche market viability. |

### Industry Scope ($S_I$) - Weight: 20%
| Ind. | Variable | Backend Implementation | Mathematical Reasoning |
|---|---|---|---|
| **I1** | Competition | `compCount` via spatial radial scan | Direct market saturation. Heavy penalty as density approaches 30 nearby competitors. |
| **I2** | Supportive Biz | `suppCount` via spatial radial scan | Agglomeration economics. Nearby non-competing food businesses actually increase destination foot traffic. |
| **I3** | Closure Rate | `sd_active_businesses` local status scan | Fraction of businesses with status INACTIVE vs ACTIVE within a 2km radius. Captures real historical failure friction. |

### Policy Scope ($S_P$) - Weight: 15%
| Ind. | Variable | Backend Implementation | Mathematical Reasoning |
|---|---|---|---|
| **P1** | Food Desert | `nourish_cbg_food_environment.is_food_desert_usda` | Policy incentive. If true, score is boosted to prioritize locations eligible for food equity grants. |
| **P4** | Enabling Facilities| `nourish_comm_commissary_ext` `ST_DWithin` | Proximity to commissary kitchens drastically lowers the CAPEX barrier for food trucks/bakers. |
| **P5** | Advisory Access | `nourish_comm_service_provider` `ST_DWithin` | Proximity to CBOs/SBDCs provides vital support scaffolding to early entrepreneurs. |
| **P6** | Zoning Flex. | `sandag_layer_zoning_base_sd_new` | Pure legal feasibility. Parcels outside Commercial/Mixed-Use severely depress the policy scope. |

### Entrepreneur Feasibility ($ME$)
| Ind. | Variable | Backend Implementation | Mathematical Reasoning |
|---|---|---|---|
| **E1** | Available Capital | `Overrides.AvailableCapital` | If capital is < 50% of the calculated `startupCost`, `δ = -0.25`. If a surplus exists, `δ = +0.10`. |
| **E5** | Experience | `Overrides.BusinessExperience` | >3 years experience triggers a `δ = +0.10` risk mitigation bump. |

---

## 4. Mapped variables based on scope.

The final objective is to map all 55 variables from the theoretical ontology directly into the SDSC database. Review the tables below to identify the open datasets required for data-engineering progress. 

### Location Scope: 20 Variables
| Code | Name | Primary Free Data Source | Mapping Target |
|---|---|---|---|
| L1 | Population density | ACS 5-Year B01003 + TIGER/Line | `bgs_sd_imp` (`totpop_cy` ÷ area) |
| L2 | Median household income | ACS B19013 | `bgs_sd_imp.medhinc_cy` |
| L3 | Poverty rate | ACS B17021 | `bgs_sd_imp.povrate_cy` |
| L4 | Unemployment rate | ACS B23025 | `bgs_sd_imp.unemprt_cy` |
| L5 | Educational attainment | ACS B15003 | `bgs_sd_imp.educ_coll_cy` |
| L6 | Walkability proxy | EPA Smart Location Database | Requires EPA SLD Import |
| L7 | Transit accessibility | GTFS (local transit agencies) | Requires GTFS routing matrix |
| L8 | Vehicle access (zero-car) | ACS B08201 | `bgs_sd_imp.zero_vehicle_hh_cy` |
| L9 | Crime rate index | City Police Open Data | Tracked locally by centroid |
| L10 | Zoning compatibility | City Zoning Shapefiles | `sandag_layer_zoning_base_sd_new` |
| L11 | Commercial vacancy rate | HUD USPS Vacancy Dataset | Proxy via `vacant_housing_units_cy` |
| L12 | Housing cost burden | ACS B25070 | `bgs_sd_imp.rent_burden_cy` |
| L13 | Housing stability index | ACS mobility tables B07001 | `bgs_sd_imp.moved_recently_cy` |
| L14 | Household size | ACS B25010 | `bgs_sd_imp.avghhsz_cy` |
| L15 | Language diversity | ACS B16001 | `bgs_sd_imp.lang_home_cy` |
| L16 | Foreign-born share | ACS B05012 | `bgs_sd_imp.forborn_cy` |
| L17 | Age distribution | ACS B01001 | `bgs_sd_imp.age_cy` |
| L18 | Retail leakage proxy | Derived ACS income + CEX model | Requires modeling script |
| L19 | Broadband access | ACS S2801 | `bgs_sd_imp.broadbnd_have_cy` |
| L20 | Pedestrian safety | City Vision Zero Incident Data | Spatial join required |

### Market Scope: 15 Variables
| Code | Name | Primary Free Data Source | Mapping Target |
|---|---|---|---|
| M1 | Food spending index | BLS CEX + ACS income model | Derived from `wlthindxcy` |
| M2 | Retail spending potential| Derived ACS income + HH comp | Derived modeling required |
| M3 | Dining-out propensity | BLS CEX regional tables | Derived demographic proxy |
| M4 | Per-capita income | ACS B19301 | `bgs_sd_imp.per_cap_inc_cy` |
| M5 | Household food budget share | BLS CEX + ACS poverty bins | Derived model |
| M6 | Daytime population ratio | LEHD LODES Workplace data | Requires LODES ingest |
| M7 | Worker inflow/outflow | LEHD LODES OD data | Requires LODES ingest |
| M8 | Price elasticity proxy | USDA ERS elasticity tables | Requires USDA mapping |
| M9 | Consumer diversity index | ACS race/ethnicity tables | `bgs_sd_imp.di_cy` |
| M10 | Store accessibility | OSM POI (grocery, markets) | Local DB scanning |
| M11 | Local food hardship | USDA Food Environment Atlas | External join (Tract level) |
| M12 | SNAP participation rate | ACS DP03 | Proxy using FARA DB |
| M13 | Dietary health index | CDC PLACES | Census tract level mapping |
| M14 | Cultural food preference | ACS ancestry | `bgs_sd_imp` Ancestry tables |
| M15 | Product trend momentum | Google Trends | MSA/County wide index |

### Industry Scope: 15 Variables
| Code | Name | Primary Free Data Source | Mapping Target |
|---|---|---|---|
| I1 | Average business size | Census County Business Patterns | Requires CBP ingest |
| I2 | New firm entry rate | Census BDS Metro/County | BDS External ingest |
| I3 | Survival rate (5-year) | Census BDS Metro/County | BDS External ingest |
| I4 | Closure rate | Census BDS Metro/County | BDS External ingest |
| I5 | Market concentration (HHI) | CBP + OSM business counts | Derived computation |
| I6 | Input cost index | USDA ERS commodity cost tables | USDA ERS mapping |
| I7 | Market growth rate | BEA Regional Accounts | BEA Metro ingest |
| I8 | Sector volatility | BLS QCEW | QCEW County variances |
| I9 | Supply chain complexity | OSM freight/industrial nodes | Network path derivation |
| I10 | Logistics cost index | FHWA National Freight Data | FHWA mapping |
| I11 | Wholesale price volatility | USDA ERS + NOAA energy data | Combined derivation |
| I12 | Labor availability | ACS commuting | `bgs_sd_imp.occ_base_cy` |
| I13 | Local skill availability | ACS occupation tables B24010 | `occ_serv_cy` |
| I14 | Licensing complexity | City Open Data: permit timelines | Requires parsing municipal feeds |
| I15 | Food safety burden index | County Health Dept. | Requires scanning health infractions |

### Policy & Temporal Scopes
| Code | Name | Primary Free Data Source | Mapping Target |
|---|---|---|---|
| P1 | OZ eligibility | Treasury CDFI Fund Shapefiles | Join GEOID to shapefile |
| P2 | Grant availability | City/County ED grant lists | Requires manual curation |
| P3 | Loan program availability | SBA 7a/504 public datasets | Requires SBA datasets |
| P4 | Shared-kitchen proximity | County Environmental Health | `nourish_comm_commissary_ext` |
| P5 | Advisory access (CBO) | Public directory | `nourish_comm_service_provider` |
| P7 | Permitting burden index | City Open Data permit times | Requires open data parser |
| P8 | Zoning allowances | City zoning shapefiles | Uses `sandag_layer_zoning` |
| T1 | Seasonal demand multiplier | Google Trends | NAICS temporal wave mapping |
| T3 | Climate stress index | FEMA National Risk Index | FEMA NRI block group join |
| T4 | Energy price index | EIA regional retail price data | State/city mapping |
| T5 | Weather anomaly frequency | NOAA NCEI Climate Normals | Tract/County joining |
| T6 | Food price volatility | USDA ERS monthly indexes | Metro/State |
| T7 | Labor market tightness | BLS LAUS + ACS commuting | Derived BLS crosswalk |
