# Nourish PT: Live Opportunity Mapper

## Project Alignment & Overview
This project provides a live-updating opportunity map designed to identify the best locations for food businesses in San Diego County. It addresses the goals established in the recent team meetings, emphasizing accurate 2022 NAICS codes standardization, demographic block-group profiling, UCSF foot traffic data integration, and community mapping. 

The application utilizes an Agentic Chat interface (integrating with Google Agent ADK / A2A patterns) to help users dial in on specific market gaps (e.g., Healthy Grocery, Healthy Prepared Foods, Bodegas) in low-access neighborhoods.

## Architecture & Mapping Stack
- **Backend:** Go (Golang) handling API endpoints, PostgreSQL connections, and the Agent Chat service. Calculations are live-hashed and cached in-memory for instant retrieval.
- **Frontend:** React + TypeScript via Vite. 
- **Mapping Stack Evolution (Choosing the Right Mapping Library):**
  - *Previous Approach:* Leaflet using thousands of SVG rectangles/hexagons. DOM bloat caused severe lag, and polygons were rendering over unbuildable space like the ocean.
  - *MapLibre Attempt:* WebGL creation failed in the testing environment (`FEATURE_FAILURE_WEBGL_EXHAUSTED_DRIVERS`).
  - *Current Approach:* **Leaflet (Canvas-Accelerated)** via `react-leaflet`. We reverted to Leaflet but injected the `preferCanvas={true}` configuration. This eliminates the DOM-bloat lag entirely by rendering all points on a single `<canvas>` element. We also completely rewrote the backend logic to generate points strictly along viable commercial parcels (bypassing unzoned areas like the ocean). To simulate a WebGL heatmap, we render layered, semi-transparent opportunity bubbles directly via the Leaflet Canvas API.

## Data Dictionary & Discoveries

### Local CSV File Index
During our dataset alignment, several local raw sources were cataloged:
* `data/gm/lajolla.csv` (and related Google Maps exports):
  - `input_id`, `link`, `title`, `category`, `address`, `open_hours`, `popular_times`, `website`, `phone`, `plus_code`, `review_count`, `review_rating`, `reviews_per_rating`, `latitude`, `longitude`, `cid`, `status`, `descriptions`, `reviews_link`, `thumbnail`, `timezone`, `price_range`, `data_id`, `place_id`, `images`, `reservations`, `order_online`, `menu`, `owner`, `complete_address`, `about`, `user_reviews`, `user_reviews_extended`, `emails`.
* `data/tax_listings/tr_active1.csv`:
  - `BUSINESS ACCT#`, `DBA NAME`, `OWNERSHIP TYPE`, `ADDRESS`, `CITY`, `ZIP`, `STATE`, `BUSINESS PHONE`, `OWNER NAME`, `CREATION DT`, `START DT`, `EXP DT`, `NAICS`, `ACTIVITY DESC`.

### Relational Database Discoveries (Key Tables Added)
Core tables used to build our opportunity heatmap:
1. **`nourish_cbg_food_environment`**: Contains `is_food_desert_usda`, `food_insecurity_rate`, `supermarket_count_1mi`. Core for isolating underserved neighborhoods.
2. **`nourish_cbg_pedestrian_flow`**: Contains daily pedestrian counts and peak vehicle hours.
3. **`san_diego_areawise_foot_traffic`**: Contains daily pedestrian time-interval breakdowns across SD business areas.
4. **`ca_laws_and_regulations`**: Complete legal corpus for food vendor rules, legal constraints, and subchapters.
5. **`nourish_cbg_demographics`**: Detailed census blocks providing `total_population`, `median_age`, race, and language demographic breakdowns.
6. **`esri_consumer_spending_data_`**: 369 columns mapping specific purchasing powers across categories.
7. **`sandag_layer_zoning_base_sd_new`**: We strictly limit map points to viable commercial developments, enforcing buildable areas (`geom` boundaries).
8. **Business Unit Economics Tables**:
   - `nourish_comm_commissary_ext` (Wastewater capacity, ice supply, parking)
   - `nourish_ref_mobile_vendor_economics` (Fuel cost, commissary fees, downtime loss)
   - `nourish_ref_bakery_economics` (Labor hours, energy costs, yield loss burn percentage)

## API Endpoints
- `GET /api/opportunity-map`: Returns map data points scored by criteria (SNAP population, foot traffic, GM ratings, competitor density) ready for MapLibre heatmap interpolation.
- `GET /api/evaluate-location`: Parses lat/lng coordinates to dynamically assign an opportunity score metric.
- `GET /api/explore-db`: Queries `information_schema.columns`.
- `POST /api/agent/chat`: Parses complex natural language constraints into functional mapping results.
