-- +goose Up
ALTER TABLE books ADD COLUMN genre TEXT;

-- +goose Down
ALTER TABLE books DROP COLUMN genre;
