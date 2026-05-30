# ebook-search

Persönliche Ebook-Bibliothek mit Volltextsuche, Genre-Filter und Merkliste. Läuft als Docker-Container auf einem Synology DXP2800 NAS.

## Features

- Suche nach Titel und Autor (live, ohne Seitenreload)
- Genre-Filter
- Merkliste (dauerhaft in der Datenbank)
- Detailansicht mit Beschreibung
- EPUB- und PDF-Unterstützung

## Stack

- **Go** (net/http) — kein Framework
- **HTMX** — dynamische UI ohne JS-Framework
- **PostgreSQL + pgvector** — Volltextsuche
- **Ollama** (`nomic-embed-text`) — Embeddings für die Klassifizierung
- **goose** — DB-Migrationen

## Starten

```bash
docker compose up --build
```

Die App läuft danach auf [http://localhost:8080](http://localhost:8080).

## Ebooks importieren

Läuft lokal (nicht im Container), liest Calibre-Bibliotheksstruktur:

```bash
DATABASE_URL=postgres://ebook:ebook@localhost:5432/ebook_search?sslmode=disable \
  go run ./cmd/import
```

## Genre-Klassifizierung

Lokal mit nativem Ollama (Metal/GPU):

```bash
DATABASE_URL=postgres://ebook:ebook@localhost:5432/ebook_search?sslmode=disable \
  go run ./cmd/classify
```

Auf dem NAS (Ollama läuft in Docker):

```bash
docker compose --profile classify run --rm classify
```

## Migrationen

```bash
go run github.com/pressly/goose/v3/cmd/goose \
  -dir internal/db/migrations postgres \
  "postgres://ebook:ebook@localhost:5432/ebook_search?sslmode=disable" up
```

## Konfiguration

| Variable | Beschreibung |
|---|---|
| `DATABASE_URL` | PostgreSQL-Verbindungs-URL |
| `OLLAMA_URL` | Ollama-Endpunkt (Standard: `http://localhost:11434`) |

## Projektstruktur

```
cmd/
  server/     – HTTP-Server (Einstiegspunkt)
  import/     – Ebook-Importer
  classify/   – Genre-Klassifizierung via Ollama
  fetchdesc/  – Beschreibungen nachladen
internal/
  db/         – Datenbankzugriff + Migrationen
  handler/    – HTTP-Handler
  parser/     – EPUB/PDF-Parser
  classifier/ – Ollama-Integration
web/
  templates/  – HTML-Templates
  static/     – CSS
```
