**[TYPE LEGEND]**
`s` = string / text / varchar
`i` = integer / bigint / smallint
`n` = numeric / float / double precision / real
`b` = boolean
`a` = array
`d` = date / time / timestamp
`g` = geometry / spatial / USER-DEFINED
`j` = json / jsonb

**[SCHEMA]**

**NAICS & ArcGIS Reference**
`2022_NAICS_Keywords`: naics_code:i, naics_title:s, naics_keywords:s
`2022_naics_descriptions`: 2007_naics:s, 2022_naics:s, description:s, 2022_Title:s, food_related:b, small_investment:b, small_business_examples:a
`ArcGIS_Schema`: Variable_Category:s, VariableName:s, description:s, LongName:s

**ESRI & Demographics (Highly Repetitive Columns Collapsed)**
`ESRI_SD_County_Tract_Level_Market_Potential_Data`: std_geography_level, source_country, aggregation_method, shape, mp31082...mp31098(text cols):s | std_geography_name, std_geography_id, population_to_polygon_size_rating, apportionment_confidence, mp*(hundreds of numeric cols):n | has_data:i
`ESRI_SD_County_Tract_Level_business_data`: std_geography_level, source_country, aggregation_method, shape:s | std_geography_name, std_geography_id, population_to_polygon_size_rating, apportionment_confidence, s*_bus, n*_bus, s*_emp, n*_emp, s*_sales, n*_sales:n | has_data, column_108:i
`ESRI_SD_County_Tract_Level_consumer_spending`: std_geography_level, source_country, aggregation_method, shape:s | std_geography_name, std_geography_id, population_to_polygon_size_rating, apportionment_confidence, x*(hundreds of cols):n | has_data:i
`esri_19k_variables_2025` / `esri_8k_variables_2025`: esri_variable*, variable_description, variable_heading, ACS*, Census*:s
`esri_business_data` / `esri_market_potential_data`: (Identical structural pattern to the ESRI_SD equivalents above)
`esri_consumer_spending_cols`: std_geography_name/id/level/etc, x*(hundreds of cols):s
`esri_consumer_spending_data_`: c1 thru c369:s
`bgs_sd_imp`: ogc_fid, has_data:i | statefp, countyfp, tractce, blkgrpce, source_cou, aggregatio:s | pop*, apportionm, x[0-9]*_a, x[0-9]*fy_a, male*, fem*, totpop*, crm*, hinc*, di*, nw*, val*, gini*, rat*, shr*, lotrhh*, mdtrhh*, uptrhh*, civlbfr*, emp*, ind*, unemp*, occ*, owner*, renter*, rntgrw*, avgia*, wlthindxcy, sei_cy:n

**FNDDS & USDA**
`FNDDS Data Dictionary`: TableName, VariableName, VariableDescription:s
`FNDDS Foods and Beverages 2019-2020`: Food code, WWEIA Category number:i | Main food description, Additional food description, WWEIA Category description:s
`FNDDS Ingredient Nutrient Values 2019-2020`: Ingredient/Nutrient code, FDC ID, SR AddMod year, Foundation year acquired:i | Ingredient/Nutrient desc, Nutrient value source, Derivation code:s | Nutrient value:n
`FNDDS Ingredients 2019-2020`: Food/Ingredient code, WWEIA Category number, Seq num, Retention code, Moisture change (%):i | Main food desc, WWEIA Category desc, Ingredient desc:s | Ingredient weight (g):n
`FNDDS Nutrient Values 2019-2020`: Food code, WWEIA Category number, Energy, Cholesterol, Retinol, Vit A, Carotenes, Crypto, Lycopene, Lutein, Folic acid, Folate, Calcium, Phos, Mag, Potas, Sod, Caff, Theo, Alc, FA 18:4:i | Protein, Carb, Sugars, Fiber, Fat, FA*, Thiamin, Ribo, Niacin, Vit B/C/D/E/K, Iron, Zinc, Cop, Sel, Water:n | Main food desc, WWEIA Category desc:s
`FNDDS Portions and Weights 2019-2020`: Food code, WWEIA Category number, Seq num:i | Main food/WWEIA Category/Portion desc:s | Portion weight:n
`USDA_Nutrition`: index, NDB_No, Energ_Kcal, Vit_A_IU, Vit_D_IU:i | Shrt_Desc, GmWt_Desc1, GmWt_Desc2:s | Water, Protein, Lipid, Ash, Carb, Fiber, Sugar, Calcium, Iron, Mag, Phos, Potas, Sod, Zinc, Cop, Mang, Sel, Vit_C/B/E/K, Thiamin, Ribo, Niacin, Panto, Folate*, Choline, Retinol, Carot*, Crypto, Lycopene, Lut, FA_*, Cholestrl, GmWt_1/2, Refuse_Pct:n
`Nutrients_Branded_Foods_2018`: NDB_No, Nutrient_Code:i | Nutrient_name, Derivation_Code, Output_uom:s | Output_value:n
`Products_Branded_Foods_2018`: NDB_Number:i | long_name, data_source, manufacturer, date_modified, date_available, ingredients_english:s | gtin_upc:n
`Serving_size_Branded_Foods_2018`: NDB_No:i | Serving_Size, Household_Serving_Size:n | Serving_Size_UOM, Household_Serving_Size_UOM, Preparation_State:s
`product_ingredients_2018`: NDB_Number:i | long_name, ingredient:s
`usda_2022_branded_food_nutrients`: id, fdc_id, nutrient_id, derivation_id:i | amount:n | data_points, min, max, median, footnote, min_year_acquired:s
`usda_2022_branded_food_product`: fdc_id:i | gtin_upc, serving_size:n | brand_owner, brand_name, subbrand_name, ingredients, not_a_significant_source_of, serving_size_unit, household_serving_fulltext, branded_food_category, data_source, package_weight, modified_date, available_date, market_country, discontinued_date, preparation_state_code, trade_channel, short_description:s
`usda_2022_food_branded_experimental`: fdc_id:i | data_type, description, food_category_id, publication_date:s
`usda_2022_food_calorie_conversion_factor`: food_nutrient_conversion_factor_id:i | protein_value, fat_value, carbohydrate_value:s
`usda_2022_food_portions`: id, fdc_id, seq_num, measure_unit_id:i | amount, gram_weight:n | portion_description, modifier, data_points, footnote, min_year_acquired:s
`usda_2022_food_protein_conversion_factor`: food_nutrient_conversion_factor_id:i | value:n
`usda_2022_nutrient_master`: id:i | name, unit_name, rank:s | nutrient_nbr:n

**Food Access, Env Atlas & Meal Gap**
`Food_Access_Research_Atlas_Data_2019`: CensusTract, Urban, Pop2010, ohu2010, GroupQuartersFlag, numgqtrs, LILA*, HUNVFlag, LowIncomeTracts, MedianFamilyIncome, LA*, Tract*:i | State, County, lapop(10/20)*, lalowi(10/20)*, lakids(10/20)*, laseniors(10/20)*...:s | pctgqtrs, PovertyRate, lapop(0.5/1)*, lalowi(0.5/1)*...:n
`Food_Access_Research_Atlas_Data_2019_Data_Dictionary`: Field, LongName, Description:s
`Food_Environment_Atlas_County_Supplemental`: fips, Value:i | State, County, Variable_Code:s
`Food_Environment_Atlas_Data_Dictionary`: Variable_Name, Category_Name, Category_Code, Subcategory_Name, Variable_Code, Geography, Units:s
`Food_Environment_Atlas_State_County`: fips:i | State, County, Variable_Code:s | Value:n
`Map_the_Meal_Gap_Data_Dictionary`: Attribute, Definition, Scope, Description:s | Years:a

**General DBs & APIs**
`FederalCodeRegulations_Title7`: unique_subsection_id:i | url, title/subtitle/chapter/subchapter/part/subpart/subject_group/subsection nums & names, references:s
`FoodKG`: id:i | title, link, source:s | ingredients, directions, ner:a
`Johnnys_Seeds`: url, parent_url, item_type, main_header, description, details, latin_name, scientific_name, days_to_maturity, life_cycle, hybrid_status, culture, for_green_manure, for_ornamental..., grazing, seed_specs, seeding_rate, use, pdf_links:s
`LexMapr_FoodOn_2022USDA_FoodProducts`: fdc_id:i | Product_Desc, Processed_Sample*, Matched_Components, Match_Status:s | Matched:a
`LexMapr_FoodOn_FoodKG_Ingredients`: Recipe_Id:i | Sample_Desc, Processed_Sample*, Matched_Components, Match_Status*:s | Matched:a
`NC_NonProfits` / `ca_nonprofits`: ein, GROUP, subsection, affiliation, classification, ruling, deductibility, foundation, activity, organization, status, tax_period, asset/income/filing_req/pf_filing_req_cd, acct_pd, asset/income/revenue_amt:i (ca_nonprofits adds nonprofit_id:i) | name, ico, street, city, state, zip, ntee_cd, sort_name:s
`ca_business_ai_franchise_full_analysis`: business_id:i | business_name, categories, city, classification, reasoning:s | confidence:n
`ca_business_permits_licenses` / `ca_business_types_permits`: id:i | county, name, description, agency_note, controlling_office*, jurisdiction:s | applies_to:a
`ca_farms_2`: farm_type, company_name, farm_address, phone_number, email_address:s | crops_product, sells_at, practices:a
`ca_laws_and_regulations`: unique_legal_index:i | corpus, url, title_*, subtitle_*, division_*, part_*, subpart_*, chapter_*, subchapter_*, article_*, subject_group_*, subsection_*:s
`ca_legal_definitions`: definition_id:i | corpus, unique_subsection_id, definition_term, definition:s
`ca_nonprofits_NTEE_Codes`: NTEECode, Description, Definition:s
`ca_nonprofits_data_dictionary`: variable_name, variable_code, description, long_name:s
`california_employment_statistics_2014-2024`: Area Type, Area Name, month, date, Industry Title, Seasonally Adjusted:s | year, Series Code, Current Employment, benchmark:i
`farmer_assistance`: farmer_id:i | assistance_category, explanation:s | response_date:d
`farmersmarket_2023-2593956`: listing_id, specialproductionmethods_*, acceptedpayment_*, fnap_*, SNAP_option_*:i | location_*, orgnization_*, diversegroup_*, saleschannel_*, fnap:s | lon, lat:n
`fda_orangebook_product`: c1..c14:s
`food_business_mapping_with_hobbies_passions`: Work Experience, Location of Work, Relevant Skills, Personal Background, Practical Constraints, Viable Business Type, Hobbies and Passions:s
`food_investor_landscape`: aum_usd_million, id:i | investor, category, pct_portfolio_in_usd, description, investment_*, comments, website:s
`food_organizations`: id:i | name, owner_org:s | products:a
`pass_by_retail_store_foot_traffic_yelp_category` / `pass_by_retail_store_visitors`: store_id, name, brand, address, city, state, week/month*, daily/hourly*, visitor_profiles:s | zip_code, total_visits:i | is_restaurant:b
`sba_franchise_directory`: SBA*, brand, FTC*, Addendum*, dates, notes:s | Reference Number:i
`sbdc_first_contact` / `sbdc_intake_table`: id/ClientID/employees/size/hh_size:i | names, contacts, address, demographics, status, desc, naics, center:s | revenue, income:n | dates:d | booleans:b | arrays:a
`snap_retailer_location_data`: record_id, zip_code, objectid:i | store_name, address, city, state, zip4, county, type, program, grantee:s | x, y, latitude, longitude:n
`statisticalatlas` / `statisticalatlas2`: county, state_name:s | original:j
`stop_words_english` / `students`: word/student_name:s | subjects:a
`us_gaz` / `us_lex` / `us_rules`: id, seq, token:i | word, stdword, rule:s | is_custom:b
`uscities`: city, state_*, county_name, source, military, incorporated, timezone:s | county_fips, pop, density, ranking, id:i | lat, lng:n | zipcodes:a
`users`: id, reg_id:i | username, password, type:s

**Entities & Neighborhoods**
`ca_business`, `ca_businesses_sd_imperial*`, `ca_businesses_with_ai_franchise`, `entity_business`: id, zip:i | name, url, address, city, blockgroup, franchise, reasoning:s | lat, lon, avg_rating, confidence:n | categories:a | geom:g (depending on view)
`city_neighborhoods`, `community_neighborhoods`, `county_neighborhoods`, `state_neighborhoods`, `locality_neighborhood`, `my_neighborhoods`: id:i | state_name, county, city, metro_area, community, summary_description, business_prospects:s | is_unincorporated_place, is_census_designated_place:b | zipcodes, neighboring_*, nearby_*, unincorporated_places:a
`community-facing_organizations`: id:i | organization_type, organization_name, address, contact_number:s
`community_naics_distribution_2`: naics_code:i | community, naics_description, business_count:s
`demographic_column_map`: long_name, column_alias:s
`entity_blockgroup`: ctblockgroup, ogc_fid:i | countyfp, tractce, blkgrpce, statefp, aggregatio, source_cou:s | n_*(various demographics/spend metrics), fem*, male*, pop*, apportionm:n | geom:g
`entity_city`, `entity_community`, `entity_county`, `entity_state`: id:i | code, name:s
`entity_relationships`, `entity_relationships1`, `entity_relationships2`: entity1, entitytype1, predicate, entity2, entitytype2:s
`entity_table`: entity1, entitytype1, predicate, entity2, entitytype2:s | geom1, geom2:g
`entity_zipcode`: zipcode:s | perimeter, area:n | geom:g

**San Diego GIS & specific Layers**
`SD_City_Business_Directory` / `sd_active_businesses`: BUSINESS ACCT#/account_key, naics*:i | DBA NAME, OWNERSHIP TYPE, address*, city, state, zip*, BUSINESS PHONE, OWNER NAME, dates, status, ACTIVITY DESC:s | lat, lng, bid, council:n
`SD_permits`: unique_permit_index, corpus, url, parent_url:s
`sd_bids`, `sd_bus_*`, `sd_fbn_*`, `sd_food_*`, `sd_mobility_hubs`, `sd_roads`, `sd_transit_stops`, `sd_zipcodes`: objectid, ids:i | names, desc, times, zips:s | lat, lon, lengths, coords:n | geom/geometry:g
`sandag_layer_*` (business_sites, census_block_groups, indian_reservation, law_beats, municipal_boundaries, roads, zipcode, zoning): ogc_fid, objectid, ids:i | name, type, address, code, descriptions:s | shape_length, shape_area, lat, lon, xcoord, ycoord:n | geom, geometry:g

**Nourish Application Schema (Grouped)**
`nourish_cbg_*` AND `nourish_comm_*`
*Shared context for these ~28 twin tables:* All use `id:i`, either `cbg_geoid:s` or `community_id:s`, `data_source:s`, `vintage_year:i`, and `created_at, updated_at:d`.
*Tables included in this namespace:* anchor_procurement, catchment_geometry, coffee_demand_signal, commissary_ext, community_sentiment, comparable_operator_observation, competitor, competitor_attribute, competitor_product_observation, competitor_traffic_observation, cultural_anchor, delivery_viability, demand_signal, demographics, digital_presence, ebt_wic_context, ethnic_distributor_proximity, ethnic_grocery_demand, event_vendor_detail, food_environment, funding_program, gtm_partnership, home_kitchen_regime_applicability, income_affordability, institutional_anchor, labor_pool, local_event, market_concentration, mobile_vendor_permit_gate, nuisance_constraint, pedestrian_flow, permit_requirement, pilot_product_result, pilot_result, pop_up_host_venue, population_time, preference_signal, purchasing_constraint, risk_profile, safety_signal, seasonality_signal, service_provider, shared_kitchen, site_buildout, site_candidate, site_lease, site_zoning, supplier_proximity, training_resource, transit_access, vending_event_opportunity, vending_route_candidate, vending_zone. *(Specific attributes mix metrics:n, counts:i, names/notes:s, flags:b, arrays:a, geoms:g)*

`nourish_cbo_*` (businesses, client_documents, client_type_changes, client_visits, clients, milestones, service_requests, service_types):
id/client_id/business_id:i | names, descriptions, addresses, naics, types:s | timestamps/dates:d | emails, phones:a | employees:i | flags:b

`nourish_ref_*` (allergen, bakery, business_type, channel_fee, coffee, eatery, ethnic_grocery, home_kitchen, mobile_vendor, pop_up, etc):
id/business_type_id/jurisdiction_id:i | category/mode/type/names/notes:s | rates/costs/percentages/sqft:n | is_active/requires_X:b | dates:d | arrays:a