-- +goose Up
ALTER TABLE books ADD COLUMN description TEXT;

-- +goose Down
ALTER TABLE books DROP COLUMN description;
