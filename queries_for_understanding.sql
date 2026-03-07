-- Use these queries against the SDSC database or via the new 'Database Explorer' frontend UI 
-- to fetch context about the primary tables mentioned in the meeting recap.
-- Paste the results back into the chat so I can write perfectly aligned SQL commands for our maps.

-- 1. Understand the structure of the primary business list
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'ca_business';

-- 2. Understand how NAICS codes are represented in the DB currently
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = '2022_NAICS_Keywords';

-- 3. Understand what demographic block-group data we have access to (SNAP/Income)
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'nourish_cbg_demographics';

-- 4. Understand what foot-traffic/UCSF data looks like
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'nourish_cbg_pedestrian_flow';

-- 5. Check if there's a community-to-block-group crosswalk table available
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'nourish_community_block_group_intersection' 
   OR table_name = 'ref_cbg_community_crosswalk';

-- 6. Sample 1 row from the business table to see what 2022 NAICS columns exist
SELECT * FROM ca_business LIMIT 1;