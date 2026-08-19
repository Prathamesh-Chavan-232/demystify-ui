-- Runs once, on first initialisation of the data volume.
-- Schema itself is goose's job (services/migrations); this is only for
-- database-level setup that must exist before the first migration runs.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
