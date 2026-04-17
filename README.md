# omNOURISH: Live Opportunity Mapper & Evaluation Engine

## Project Alignment & Overview
This project provides a live-updating opportunity map designed to identify the best locations for food businesses in San Diego County. It utilizes an advanced mathematical Opportunity Scoring framework bridging spatial variables (L, M, P, T) and entrepreneur attributes (E).

The Go backend (`/backend`) has been modularized and implements real-time spatial evaluations utilizing the PostgreSQL database tables documented below. 

## Architectural Refactoring
The backend monolith `main.go` has been refactored for clarity and domain organization:
- `main.go`: Application initialization, environment configuration, and HTTP routing mapping.
- `models.go`: Unified application and mathematics struct definitions (including NAICS mapping matrices).
- `data.go`: Database connection state, global cache bindings, and geospatial processing functions.
- `api.go`: Exposed web handler routes servicing map points, recommender systems, and individual location polling.
- `scoring.go`: Core domain mathematics logic processing NAICS scaling algorithms and L, E, M, P, T structural variables.
- `llm.go`: External AI Gateway handlers.

## Opportunity Evaluation Framework (The Math)
The backend evaluates block groups using the following mathematical formulation:

**Core Math**:
`CO = (MF^0.4 * LF^0.4 * PF^0.2)^(1 / 1.0)`
`Sfinal = clip(CO * RF * TF * ME)`

### The Indicators & Active Table Queries
The `calculateOpportunityScore()` function inside `scoring.go` maps indicator constants dynamically via spatial queries:

**Location & Demographics (L & M)**
*   **L1 (Pop Density) / M4 (Pop Size):** Queried against `bgs_sd_imp` (`totpop_cy` relative to spatial geographic area sizing).
*   **L2 (Median Household Income):** Queried directly via `medhinc_cy` on `bgs_sd_imp`.
*   **L4 (Commercial Rent):** Queried via Esri mapping `esri_consumer_spending_data_`.
*   **L7 (Transit Accessibility):** Extracted by running PostGIS `ST_DWithin` buffers over `sd_transit_stops`.
*   **L10 (Zoning Compatibility):** Verified using point geometries over the `sandag_layer_zoning_base_sd_new` table (specifically filtering for Mixed or Commercial zones).
*   **M3 (SNAP / Low Income):** Checked using boolean USDA queries against `nourish_cbg_food_environment`.

**Policy & Temporal (P & T)**
*   **P4 (Shared Kitchen Access):** PostGIS `ST_DWithin` queries against `nourish_comm_commissary_ext` to look for existing localized infrastructure enabling food ventures without brick-and-mortar leases.

**Entrepreneur Capacity (E - What If Scenarios)**
*   Provides support for user-defined overrides through the `ScoreOverrides` API body. If an entrepreneur reports available capital (E1) below baseline requirements, the backend dynamically restricts the output `ME` multiplier.

### NAICS Adjustments
A `NAICSMatrix` enforces strict sensitivity logic over NAICS profiles. For example, Bakery codes (`311812`) suffer elevated rent and energy sensitivities compared to standard groceries (`445110`). The math dynamically reduces the baseline Risk Factor (RF) calculation based on your inputted NAICS matrix and geographic zone multipliers (e.g., Urban Coastal vs Inland).

## Setting up
Open command prompt/terminal in the base folder (`omNOURISH/`) and run:
`docker-compose up --build`
- Proceed to connect to the interface via the stated url (likely `localhost:8082/`)
