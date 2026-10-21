CREATE TABLE IF NOT EXISTS products (
    id varchar(36) PRIMARY KEY,
    name text NOT NULL,
    description text,
    price double precision NOT NULL,
    stock bigint NOT NULL DEFAULT 0,
    created_at timestamptz,
    updated_at timestamptz
);
