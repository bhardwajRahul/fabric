-- Adds lineage tracking columns to microbus_steps for the explicit fan-in model.
-- See workflow/CLAUDE.md "Fan-in is explicit when SetFanIn is declared" for the
-- design. Default 0 marks rows created before this migration and rows in graphs
-- that do not declare SetFanIn (those continue to use the depth-based fan-in
-- evaluator and never read these columns).

-- DRIVER: mysql
ALTER TABLE microbus_steps ADD COLUMN lineage_id BIGINT NOT NULL DEFAULT 0;

-- DRIVER: mysql
ALTER TABLE microbus_steps ADD COLUMN cohort_size INT NOT NULL DEFAULT 0;

-- DRIVER: mysql
ALTER TABLE microbus_steps ADD COLUMN cohort_arrivals INT NOT NULL DEFAULT 0;

-- DRIVER: pgx
ALTER TABLE microbus_steps ADD COLUMN lineage_id BIGINT NOT NULL DEFAULT 0;

-- DRIVER: pgx
ALTER TABLE microbus_steps ADD COLUMN cohort_size INT NOT NULL DEFAULT 0;

-- DRIVER: pgx
ALTER TABLE microbus_steps ADD COLUMN cohort_arrivals INT NOT NULL DEFAULT 0;

-- DRIVER: mssql
ALTER TABLE microbus_steps ADD lineage_id BIGINT NOT NULL DEFAULT 0;

-- DRIVER: mssql
ALTER TABLE microbus_steps ADD cohort_size INT NOT NULL DEFAULT 0;

-- DRIVER: mssql
ALTER TABLE microbus_steps ADD cohort_arrivals INT NOT NULL DEFAULT 0;

-- DRIVER: sqlite
ALTER TABLE microbus_steps ADD COLUMN lineage_id INTEGER NOT NULL DEFAULT 0;

-- DRIVER: sqlite
ALTER TABLE microbus_steps ADD COLUMN cohort_size INTEGER NOT NULL DEFAULT 0;

-- DRIVER: sqlite
ALTER TABLE microbus_steps ADD COLUMN cohort_arrivals INTEGER NOT NULL DEFAULT 0;
