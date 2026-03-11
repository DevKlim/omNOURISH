# omNOURISH: Live Opportunity Mapper

## Project Alignment & Overview
This project provides a live-updating opportunity map designed to identify the best locations for food businesses in San Diego County. It addresses the goals established in the recent team meetings, emphasizing accurate 2022 NAICS codes standardization, demographic block-group profiling, UCSF foot traffic data integration, and community mapping. 

It heavily focuses on integrating dynamic indicators like Gentrification/Income and Offsetting Food Deserts to locate highly impactful market gaps.

## Architecture & Mapping Stack
- **Backend:** Go (Golang) handling API endpoints, PostgreSQL connections, and the Agent Chat service. The API exposes dual endpoints starting either from a selected location, or a selected NAICS business configuration framework.
- **Frontend:** React + TypeScript via Vite. 
- **Mapping Stack Evolution (Choosing the Right Mapping Library):**
  - *Previous Approach:* Leaflet using thousands of SVG rectangles/hexagons. DOM bloat caused severe lag, and polygons were rendering over unbuildable space like the ocean.
  - *MapLibre Attempt:* WebGL creation failed in the testing environment (`FEATURE_FAILURE_WEBGL_EXHAUSTED_DRIVERS`).
  - *Current Approach:* **Leaflet (Canvas-Accelerated)** via `react-leaflet`. We reverted to Leaflet but injected the `preferCanvas={true}` configuration. This eliminates the DOM-bloat lag entirely by rendering all points on a single `<canvas>` element. We also completely rewrote the backend logic to generate points strictly along viable commercial parcels (bypassing unzoned areas like the ocean). To simulate a WebGL heatmap, we render layered, semi-transparent opportunity bubbles directly via the Leaflet Canvas API.

## Setting up
Open command prompt/terminal in the base folder (`omNOURISH/`) and run:
`docker-compose up --build`
- Proceed to connect to the interface via the stated url (likely `localhost:8082/`)

## API Endpoints (Swagger Supported)
- `GET /api/business-profiles`: Fetches the available configured NAICS business frameworks and their unique baseline scoring weights.
- `GET /api/recommend-business`: Accepts coordinates (`lat`/`lng`) and cross-evaluates all known business types, returning a sorted JSON recommending the best type of business to establish in that specific location based on market gaps and real estate metrics.
- `GET /api/opportunity-map`: Returns map data points scored by criteria (SNAP population, foot traffic, GM ratings, competitor density) representing the optimal locations to host the specified NAICS business type.
- `GET /api/evaluate-location`: Parses lat/lng coordinates to dynamically assign an opportunity score metric.
- `GET /api/explore-db`: Queries Postgres schemas for direct LLM integration.

