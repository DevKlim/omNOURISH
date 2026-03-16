# Nourish PT: Codebase Context & Development Guide

## Overview
omNOURISH is a real-time opportunity mapper for food businesses in San Diego County. The stack consists of a React/Vite/Leaflet frontend and a Go backend connected to a PostgreSQL/PostGIS database (AwesomeDB).

## Data Architecture Strategy
*   **Memory/CSV (Local `data/` folder):** Used purely for static, lightweight point-based data (e.g., Google Maps scrapes, generic Tax Listings).
*   **AwesomeDB (PostgreSQL + PostGIS):** Used for complex geometries, polygons, and massive demographic datasets.
    *   *Rule of Thumb:* If a dataset involves block groups, zoning polygons, or zip-code boundaries, it must be queried dynamically from Postgres using PostGIS functions (`ST_Intersects`, `<->` nearest neighbor, etc.). Do not export spatial polygons to the local CSV folder.

## Active Placeholders to Replace
The following metrics are currently generated using mathematical heuristics in `backend/main.go` and need to be replaced with real AwesomeDB queries:

1.  **Income Level & Gentrification Index:** 
    *   *Current State:* Calculated using lat/lng coordinates in `getDemographicsHeuristic()`.
    *   *Next Step:* Replace with a `ST_Intersects` query against `bg_sd_imp_values` or `nourish_cbg_demographics`.
2.  **Population Growth:** 
    *   *Current State:* Hardcoded to `3.2`.
    *   *Next Step:* Pull from Census/ACS datasets in the DB.
3.  **Rent / Utilities Cost:**
    *   *Current State:* Inferred from Google Maps `$` symbols of nearby competitors.
    *   *Next Step:* Pull from `esri_consumer_spending_data_` or local real estate data.

## Future Development Questions & Checkpoints
When resuming development, consider asking/tackling the following:
*   **Database Sync:** Are the schemas for `bg_sd_imp_values` matching the expected columns? Use the UI's "Database Explorer" tab to pull live schemas.
*   **Performance:** The map uses `preferCanvas={true}` for rendering point nodes, but spatial queries for scoring can take time. Should we pre-calculate "Opportunity Base Scores" for all block groups and cache them in Postgres?
*   **Heuristic Cleanup:** How can we restructure the `LocationEvalResponse` to gracefully fall back to approximations *only* when the DB query returns a NULL for a specific parcel?

## AI Developer Instructions
When working in this repository:
1.  **Check DB Dependencies:** Before modifying opportunity math, verify which SQL tables are available.
2.  **Preserve the Canvas:** Leaflet is currently highly optimized using Canvas rendering. Do not revert to SVG layers or standard markers, as 10,000+ zoning points will crash the browser.
3.  **PostGIS Syntax:** Standardize spatial lookups to use SRID 4326 (`ST_SetSRID(ST_MakePoint(lng, lat), 4326)`).