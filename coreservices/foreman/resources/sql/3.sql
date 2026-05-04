-- Adds surgraph_step_id to microbus_flows so a subgraph flow can identify its
-- surgraph step by primary key, instead of searching by (flow_id, step_depth, status)
-- which is ambiguous when multiple parked surgraph steps coexist at the same depth
-- (static fan-out to multiple subgraph siblings, or concurrent dynamic subgraphs).
-- Default 0 marks rows created before this migration; the lookup falls back to the
-- legacy lease-threshold search for those.

-- DRIVER: mysql
ALTER TABLE microbus_flows ADD COLUMN surgraph_step_id BIGINT NOT NULL DEFAULT 0;

-- DRIVER: pgx
ALTER TABLE microbus_flows ADD COLUMN surgraph_step_id BIGINT NOT NULL DEFAULT 0;

-- DRIVER: mssql
ALTER TABLE microbus_flows ADD surgraph_step_id BIGINT NOT NULL DEFAULT 0;

-- DRIVER: sqlite
ALTER TABLE microbus_flows ADD COLUMN surgraph_step_id INTEGER NOT NULL DEFAULT 0;
