# ebook-search

## Stack
- Go (net/http)
- HTMX für dynamische UI ohne JS-Framework
- PostgreSQL mit pgvector für Hybrid-Suche (Volltext + Vektor)
- html/template für serverseitiges Rendering
- Ollama (lokal) für Embeddings — Modell: `nomic-embed-text`
- goose für DB-Migrationen

## Projektstruktur
cmd/server/main.go – Einstiegspunkt
internal/db/        – Datenbankzugriff
internal/handler/   – HTTP-Handler
internal/search/    – Suchlogik (Hybrid: tsvector + pgvector)
internal/parser/    – EPUB/PDF-Parser
web/templates/      – HTML-Templates (html/template)
web/static/         – CSS, minimales JS

## Konventionen
- Fehler immer explizit behandeln (kein panic in handlers)
- SQL direkt, kein ORM
- HTMX-Responses als partielle HTML-Snippets

## Deployment
- Zielsystem: DXP2800 NAS, Deployment als Docker Container via Docker Compose
- PostgreSQL läuft bereits auf dem NAS (kann angepasst werden, aber nicht neu aufgesetzt)
- `docker-compose.yml` im Projektwurzel definiert den App-Container

## Konfiguration
- `DATABASE_URL` – PostgreSQL-Verbindung (z.B. `postgres://user:pass@localhost/ebook_search`)
- Server läuft auf Port 8080

## Befehle
- `go run ./cmd/server` – Server starten
- `go test ./...` – alle Tests ausführen
- `go test -tags integration ./...` – inkl. Integrationstests (benötigt laufende DB)
- `go run github.com/pressly/goose/v3/cmd/goose -dir internal/db/migrations postgres $DATABASE_URL up` – Migrationen ausführen
- `go build ./cmd/server` – Binary bauen
- `DATABASE_URL=postgres://ebook:ebook@localhost:5432/ebook_search?sslmode=disable go run ./cmd/import` – Ebooks importieren (läuft lokal, nicht im Container)
- `docker compose up --build` – App + Ollama als Container starten
- `docker compose up --build -d` – im Hintergrund starten
