-- +goose Up
-- Deliberately thin. Token payloads live in JSONB because the canonical model
-- will move for months; promote columns out of it once it stops moving.
CREATE TABLE users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email      text        NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE themes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_json jsonb       NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX themes_user_id_idx ON themes (user_id);

-- +goose Down
DROP TABLE themes;
DROP TABLE users;
