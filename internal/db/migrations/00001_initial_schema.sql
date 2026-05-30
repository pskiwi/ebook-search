-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE books (
    id        BIGSERIAL PRIMARY KEY,
    title     TEXT NOT NULL,
    author    TEXT,
    file_path TEXT NOT NULL UNIQUE,
    file_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chunks (
    id        BIGSERIAL PRIMARY KEY,
    book_id   BIGINT NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    content   TEXT NOT NULL,
    tsv       tsvector GENERATED ALWAYS AS (to_tsvector('german', content)) STORED,
    embedding vector(768),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX chunks_tsv_idx       ON chunks USING GIN(tsv);
CREATE INDEX chunks_embedding_idx ON chunks USING hnsw(embedding vector_cosine_ops);

-- +goose Down
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS books;
DROP EXTENSION IF EXISTS vector;
