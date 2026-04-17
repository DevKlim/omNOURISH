# Data Acquisition & System Advocacy Report

## 1. External Data Acquisition Roadmap
To achieve the full 55-variable Opportunity Ontology, several external datasets must be acquired and joined to our PostgreSQL/PostGIS database. While we have exhausted the current DB capacity (implementing `POVRATE`, `ZERO_VEHICLE`, `EDUC_COLL`, `RENT_BURDEN`, etc.), the following open datasets remain critical.

### A. EPA Smart Location Database (SLD)
* **Variables Supported:** L6 (Walkability)
* **Acquisition:** Free download from the EPA's website as a national geodatabase or shapefile.
* **Integration Strategy:** Join the `D3B` (Street intersection density) and `D4A` (Distance to transit) fields to our `entity_blockgroup` via spatial join or FIPS code matching. *Why? Relying on GM review counts as a walkability proxy heavily biases areas that already have successful businesses, ignoring potentially excellent, underdeveloped walkable streets.*

### B. LEHD LODES (Longitudinal Employer-Household Dynamics)
* **Variables Supported:** M6 (Daytime Pop Ratio), M7 (Worker Inflow/Outflow), I9 (Supply Chain Complexity proxy)
* **Acquisition:** Download from Census Bureau's OnTheMap / LODES raw files (`wac` and `od` tables).
* **Integration Strategy:** Cross-reference the Census Block codes. This is crucial because currently, we estimate daytime population using basic `emp_cy` metrics, which misses the flow of commuters. Without LODES, we do not know if workers are *arriving* to a block group or *leaving* it.

### C. Census Business Dynamics Statistics (BDS) & County Business Patterns (CBP)
* **Variables Supported:** I1 (Avg Business Size), I2 (New Firm Entry), I3 (Survival Rate), I4 (Closure Rate), I5 (Market Concentration)
* **Acquisition:** Census API or bulk CSV downloads.
* **Integration Strategy:** These are generally available at the County or MSA level. We must map NAICS codes against these tables to inject true historical risk profiles.

### D. GTFS (General Transit Feed Specification)
* **Variables Supported:** L7 (Transit Accessibility), T10 (Transit Service Changes)
* **Acquisition:** San Diego MTS open data portal.
* **Integration Strategy:** Currently, we count raw stops (`sd_transit_stops`). This is flawed. A stop with 1 bus a day is mathematically treated the same as a major transit hub. We must parse `stop_times.txt` to calculate *frequency-weighted* transit access.

### E. City Open Data (Permits & Health Inspections)
* **Variables Supported:** I14 (Licensing Complexity), I15 (Food Safety Burden), P7 (Permitting Burden)
* **Acquisition:** San Diego Open Data Portal (via API or CSV).
* **Integration Strategy:** Time-delta calculations between `Permit Applied` and `Permit Issued` dates, aggregated by Zip Code or Community Planning Area.

---

## 2. Advocate / Critical Assessment of Current Decisions
As an advocate for the system's robustness, several decisions embedded in the current logic matrix should be challenged:

### The Geometric Mean "Harshness"
* **The Decision:** We use Geometric Aggregation for the `Sstruct` calculation to enforce bottleneck behavior (i.e. high income cannot hide bad zoning). We bound the minimum normalized Z-score to `0.05` to prevent `log(0)`.
* **The Critique:** While geometric means prevent high scores in one area from hiding fatal flaws in another, setting the absolute minimum bound to `0.05` is arbitrary. If rent is exorbitant (yielding a 0.05 score), it drags the entire Location Scope ($S_L$) down massively, even if foot traffic is world-class.
* **Recommendation:** We should consider moving to a *Constant Elasticity of Substitution (CES)* function. This allows us to mathematically calibrate exactly *how substitutable* variables are, rather than defaulting to the strict multiplicative penalty of the geometric mean.

### Radial Distance vs Network Distance
* **The Decision:** We use `ST_DWithin` to find transit stops and shared kitchens within an 800m radial buffer.
* **The Critique:** People and delivery trucks do not travel through walls, canyons, or across freeways. An 800m radial circle in San Diego often captures amenities that require a 3-mile drive to actually reach due to canyons or highway divisions.
* **Recommendation:** Integrate `pgRouting` into the PostGIS database to calculate true walking and driving network distances (isochrones).

### The Zero-Vehicle Households (L8) Assumption
* **The Decision:** We mapped `ZERO_VEHICLE_HH_CY` to a positive Z-score, assuming that areas with fewer cars have higher necessity-driven local walk-in traffic.
* **The Critique:** In underserved areas, zero-vehicle households often rely heavily on transit to travel to cheaper big-box stores outside their block group. A lack of cars does not automatically guarantee that residents will spend their limited income at a local business. This assumption is dangerous without correlating it against the RSPI (Retail Spending Potential).

### Closure Rate (I3) "Survivorship Bias"
* **The Decision:** Our current SQL query checks active vs. inactive businesses in a 2km radius using `sd_active_businesses`.
* **The Critique:** This creates a survivorship bias loop. A thriving commercial block with rapid turnover will show many "inactive" licenses, giving it a bad survival score. Conversely, a dead residential street where nothing ever opens will show zero "inactive" licenses, incorrectly giving it a better survival score than the thriving commercial street.
* **Recommendation:** We must switch to NAICS-specific Census BDS data, abandoning the 2km radius proxy for survival entirely.