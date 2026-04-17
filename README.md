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

### NOURISH PT Setup and Integration Guide

Overview
-> How to set up the repository...

Repository Setup
Clone the repository to your local machine and navigate into the root directory. 
You must have Docker installed. Run the command docker-compose up --build to initialize the system. 
This command starts the backend service on port 8081 and the frontend web interface on port 8082. 

Configuration
You need to configure your environment variables before starting the application. 
Copy the provided example environment file to a new file named .env in the root folder. 
Open this file and fill in your database credentials. 
This includes the database host, port, username, password, and database name. 

Data Downloads
You will receive a set of data files separately. 
Place all provided comma separated value files and datasets into the data directory located in the project root. 
The backend requires these files to calculate baseline foot traffic, commercial rent, and demographic information when the database is unreachable.
Place `data/` in the root folder alongside this readme file.
Download here: https://drive.google.com/file/d/1plN_iCzEzyYGbQvjdDaxNP6La0YSbFdq/view?usp=sharing

Connecting
The communicate preference is over HTTP. 
You need to point the main platform to your local backend address. 
You can test the connection by opening your web browser and navigating to localhost:8081/api/health. 
A healthy status message indicates the application programming interface is ready to receive requests from the core NOURISH platform.

Accessing the Application Programming Interface
You can explore the full documentation by visiting localhost:8081/swagger in your browser. 
This interactive interface allows you to test endpoints and view the expected request and response structures.

Endpoints and Parameters
The evaluate location endpoint is located at /api/evaluate-location and is used to score a specific site. 
It accepts a latitude parameter named lat and a longitude parameter named lng. 
You can alternatively provide a physical address string. 
You may also pass a business category code using the naics parameter or specific terms using the keyword parameter such as bakery or coffee.

The opportunity map endpoint is located at /api/opportunity-map and returns a grid of scored locations within a designated area. 
This endpoint requires bounding box coordinates. You must provide the north latitude limit as n, the south latitude limit as s, 
the east longitude limit as e, and the west longitude limit as w.

The find best match endpoint is located at /api/find-best-match and searches the designated bounding box for the most mathematically viable business opportunities. 
It accepts the same bounding box parameters as the opportunity map. It also accepts a budget parameter to filter out opportunities that exceed a specified maximum startup cost.

