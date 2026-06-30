-- DRIVER: mysql
ALTER TABLE person
	ADD first_name VARCHAR(64) NOT NULL DEFAULT '',
	ADD last_name VARCHAR(64) NOT NULL DEFAULT '',
	ADD email VARCHAR(256) NOT NULL DEFAULT '',
	ADD birthday DATETIME NULL;

-- DRIVER: mysql
CREATE UNIQUE INDEX person_idx_email ON person (tenant_id, email);

-- DRIVER: pgx
ALTER TABLE person
	ADD COLUMN first_name VARCHAR(64) NOT NULL DEFAULT '',
	ADD COLUMN last_name VARCHAR(64) NOT NULL DEFAULT '',
	ADD COLUMN email VARCHAR(256) NOT NULL DEFAULT '',
	ADD COLUMN birthday TIMESTAMP WITH TIME ZONE NULL;

-- DRIVER: pgx
CREATE UNIQUE INDEX person_idx_email ON person (tenant_id, email);

-- DRIVER: mssql
ALTER TABLE person ADD
	first_name NVARCHAR(64) NOT NULL DEFAULT '',
	last_name NVARCHAR(64) NOT NULL DEFAULT '',
	email NVARCHAR(256) NOT NULL DEFAULT '',
	birthday DATETIME2 NULL;

-- DRIVER: mssql
CREATE UNIQUE INDEX person_idx_email ON person (tenant_id, email);

-- DRIVER: sqlite
ALTER TABLE person ADD COLUMN first_name TEXT NOT NULL DEFAULT '';
-- DRIVER: sqlite
ALTER TABLE person ADD COLUMN last_name TEXT NOT NULL DEFAULT '';
-- DRIVER: sqlite
ALTER TABLE person ADD COLUMN email TEXT NOT NULL DEFAULT '';
-- DRIVER: sqlite
ALTER TABLE person ADD COLUMN birthday DATETIME;

-- DRIVER: sqlite
CREATE UNIQUE INDEX person_idx_email ON person (tenant_id, email);
