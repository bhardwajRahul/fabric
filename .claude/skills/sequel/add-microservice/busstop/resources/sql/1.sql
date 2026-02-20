-- DRIVER: mysql
CREATE TABLE bus_stop (
	tenant_id INT NOT NULL,
	id BIGINT NOT NULL AUTO_INCREMENT,
	revision BIGINT NOT NULL DEFAULT 0,
	example VARCHAR(256) NULL,
	created_at DATETIME(3) NOT NULL,
	updated_at DATETIME(3) NOT NULL,
	reserved_before DATETIME(3) NOT NULL,

	CONSTRAINT bus_stop_pk PRIMARY KEY (tenant_id, id),
	UNIQUE INDEX bus_stop_idx_id (id),
	INDEX bus_stop_idx_created_at (tenant_id, created_at)
);

-- DRIVER: pgx
CREATE TABLE bus_stop (
	tenant_id INT NOT NULL,
	id BIGSERIAL,
	revision BIGINT NOT NULL DEFAULT 0,
	example VARCHAR(256) NULL,
	created_at TIMESTAMP(3) NOT NULL,
	updated_at TIMESTAMP(3) NOT NULL,
	reserved_before TIMESTAMP(3) NOT NULL,

	CONSTRAINT bus_stop_pk PRIMARY KEY (tenant_id, id)
);
-- DRIVER: pgx
CREATE UNIQUE INDEX bus_stop_idx_id ON bus_stop USING btree (id);
-- DRIVER: pgx
CREATE INDEX bus_stop_idx_created_at ON bus_stop USING btree (tenant_id, created_at);

-- DRIVER: mssql
CREATE TABLE bus_stop (
	tenant_id INT NOT NULL,
	id BIGINT IDENTITY(1, 1),
	revision BIGINT NOT NULL DEFAULT 0,
	example NVARCHAR(256) NULL,
	created_at DATETIME2(3) NOT NULL,
	updated_at DATETIME2(3) NOT NULL,
	reserved_before DATETIME2(3) NOT NULL,

	CONSTRAINT bus_stop_pk PRIMARY KEY NONCLUSTERED (id),
	CONSTRAINT bus_stop_idx_id UNIQUE CLUSTERED (tenant_id, id),
	INDEX bus_stop_idx_created_at (tenant_id, created_at)
);
