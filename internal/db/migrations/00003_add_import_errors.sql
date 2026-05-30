-- +goose Up
CREATE TABLE import_errors (
    id         BIGSERIAL PRIMARY KEY,
    file_path  TEXT NOT NULL,
    error      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (file_path)
);

-- +goose Down
DROP TABLE import_errors;
