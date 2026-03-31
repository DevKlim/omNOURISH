# NOURISH PT: Evaluation Engine & Calculation Ontology

This document provides a comprehensive, in-depth explanation of the Nourish PT evaluation engine. It details the exact mathematical models currently in production, the rationale behind these specific calculations, the heuristic fallbacks used for missing data, and an exhaustive roadmap of everything remaining to achieve the full 55-indicator opportunity ontology.

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

## 2. Implemented Indicators (In-Depth Active Set)

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
| **M3** | SNAP Participation | `Food_Access_Research_Atlas_Data_2019` | Joined via `CensusTract`. Indicates demand for discount grocery and affordability constraints; scaled heavily for grocery/market profiles. |
| **M4** | Per-Capita Income | `bgs_sd_imp.avgia_cy` | Secondary income metric normalizing MHI by headcount to determine individual pricing flexibility. |
| **M6** | Daytime Pop Ratio| `bgs_sd_imp.emp_cy` | Total employment in the block group. Identifies commercial/commuter hubs essential for lunch-hour viability. |

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
| **P6** | Zoning Flex. | `sandag_layer_zoning_base_sd_new` | Pure legal feasibility. Parcels outside Commercial/Mixed-Use severely depress the policy scope. |

### Entrepreneur Feasibility ($ME$)
| Ind. | Variable | Backend Implementation | Mathematical Reasoning |
|---|---|---|---|
| **E1** | Available Capital | `Overrides.AvailableCapital` | If capital is < 50% of the calculated `startupCost`, `δ = -0.25`. If a surplus exists, `δ = +0.10`. |
| **E5** | Experience | `Overrides.BusinessExperience` | >3 years experience triggers a `δ = +0.10` risk mitigation bump. |

---

## 3. How to Progress: Remaining Variables Roadmap

The mathematical foundation and the core 20 variables are highly stable. The final objective is to map the remaining variables from the 55-indicator ontology directly to the SDSC database.

### Step 1: Implement Missing Policy & Temporal Logic
*   **P7 (Permit Burden):** Currently proxied at `0.45`. We need to query `ca_business_permits_licenses` to map the specific `jurisdiction` of the requested `(lat, lng)`, and calculate average approval times to map true regulatory friction.
*   **T5 (Wage Inflation):** Currently proxied at `0.40`. Tap `california_employment_statistics_2014-2024` to dynamically inject the current year's regional unemployment tightness, actively modifying the expected `LaborCostPct`.
*   **T12 (Macro-Economic Shock):** Pull data from national/regional feeds to impose a unified penalty multiplier on premium-priced business models when general consumer spending drops.

### Step 2: Refining Local Footprint Data
*   **M8 (Tourist/Visitor Density):** Connect `pass_by_retail_store_foot_traffic` and `pass_by_retail_store_visitors` tables. Query this via an `ST_DWithin` spatial match to completely replace the Google Map `reviewCount` proxy. This provides true UCSF cell-phone tracking volume for pedestrian demand (`xL6 / xM8`).
*   **L11 (Proximity to Suppliers):** Calculate network distance from the block-group centroid to entities in `ca_farms_2` or wholesale NAICS codes in `ca_business`.

### Step 3: Connecting the Entrepreneur Intake Form
Right now, `ME` relies exclusively on manual overrides via the `ScoreOverrides` API payload. We need to implement a data-bridge to `sbdc_intake_table`. By accepting a `farmer_id` or `client_id` in the API, the backend should automatically query the user's documented certifications (E6-E9), and pre-fill their available startup capital to generate the `δ_capital` modifier automatically.
