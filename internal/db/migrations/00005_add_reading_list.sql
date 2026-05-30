-- +goose Up
CREATE TABLE reading_list (
    book_id BIGINT PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE reading_list;
