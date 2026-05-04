-- Adds an index on created_at for both microbus_flows and microbus_steps to
-- support time-window queries (e.g. "all flows created in the last hour").
-- created_at is append-only and monotonically increasing in practice, so the
-- B-tree maintenance cost is minimal: new rows always land at the rightmost
-- leaf page, avoiding random page splits across the index.

-- DRIVER: mysql
CREATE INDEX idx_microbus_flows_created_at ON microbus_flows (created_at);

-- DRIVER: mysql
CREATE INDEX idx_microbus_steps_created_at ON microbus_steps (created_at);

-- DRIVER: pgx
CREATE INDEX idx_microbus_flows_created_at ON microbus_flows (created_at);

-- DRIVER: pgx
CREATE INDEX idx_microbus_steps_created_at ON microbus_steps (created_at);

-- DRIVER: mssql
CREATE INDEX idx_microbus_flows_created_at ON microbus_flows (created_at);

-- DRIVER: mssql
CREATE INDEX idx_microbus_steps_created_at ON microbus_steps (created_at);

-- DRIVER: sqlite
CREATE INDEX idx_microbus_flows_created_at ON microbus_flows (created_at);

-- DRIVER: sqlite
CREATE INDEX idx_microbus_steps_created_at ON microbus_steps (created_at);
