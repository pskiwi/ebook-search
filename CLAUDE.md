# ebook-search

## Stack
- Go (net/http)
- HTMX für dynamische UI ohne JS-Framework
- PostgreSQL mit pgvector für Hybrid-Suche (Volltext + Vektor)
- html/template für serverseitiges Rendering
- Ollama (lokal) für Embeddings (`nomic-embed-text`) und Genre-Klassifikation (`llama3.2`)
- goose für DB-Migrationen

## Projektstruktur
cmd/server/    – HTTP-Server (Einstiegspunkt)
cmd/import/    – Importer: scannt Calibre-Verzeichnis, importiert EPUB/PDF/MOBI
cmd/classify/  – Genre-Klassifikation via Ollama (lokal oder im Container)
cmd/fetchdesc/ – Beschreibungen via Google Books API nachladen
cmd/fixtitles/ – Titelfehler anhand der Calibre-Verzeichnisstruktur korrigieren
internal/db/         – Datenbankzugriff (SQL direkt, kein ORM)
internal/handler/    – HTTP-Handler inkl. Format-Gruppierung (EPUB/PDF/MOBI pro Buch), Google Books Info-Button (`/book/info/<id>`)
internal/importer/   – Import-Logik: Hashing, Metadaten-Fallback via Calibre-Pfad
internal/classifier/ – Ollama-basierte Genre-Klassifikation
internal/embeddings/ – Embedding-Generierung via Ollama (nomic-embed-text)
internal/search/     – Suchlogik (Hybrid: tsvector + pgvector)
internal/parser/     – EPUB/PDF/MOBI-Parser (Metadaten + Volltext)
web/templates/       – HTML-Templates (html/template), HTMX-Partials
web/static/          – CSS, minimales JS

## Konventionen
- Fehler immer explizit behandeln (kein panic in handlers)
- SQL direkt, kein ORM
- HTMX-Responses als partielle HTML-Snippets
- Keine sensitiven Daten committen: Passwörter, API-Keys, Secrets gehören in `.env` (ist in `.gitignore`), nie in versionierte Dateien

## Deployment
- Zielsystem: DXP2800 NAS, Deployment als Docker Container via Docker Compose
- PostgreSQL läuft bereits auf dem NAS (kann angepasst werden, aber nicht neu aufgesetzt)
- `docker-compose.yml` im Projektwurzel definiert den App-Container

## Konfiguration
- `DATABASE_URL` – PostgreSQL-Verbindung (z.B. `postgres://user:pass@localhost/ebook_search`)
- `GOOGLE_BOOKS_API_KEY` – API-Key für Google Books (Info-Button im Detailpanel, `/book/info/<id>`)
- Server läuft auf Port 8080

## Befehle
- `go run ./cmd/server` – Server starten
- `go test ./...` – alle Tests ausführen
- `go test -tags integration ./...` – inkl. Integrationstests (benötigt laufende DB)
- `go run github.com/pressly/goose/v3/cmd/goose -dir internal/db/migrations postgres $DATABASE_URL up` – Migrationen ausführen
- `go build ./cmd/server` – Binary bauen
- `DATABASE_URL=postgres://USER:PASSWORD@localhost:5432/ebook_search?sslmode=disable go run ./cmd/import` – EPUB/PDF/MOBI importieren (lokal, Calibre-Verzeichnis)
- `DATABASE_URL=postgres://USER:PASSWORD@localhost:5432/ebook_search?sslmode=disable go run ./cmd/classify` – Bücher klassifizieren (Mac lokal, Ollama mit Metal/GPU)
- `DATABASE_URL=postgres://USER:PASSWORD@localhost:5432/ebook_search?sslmode=disable go run ./cmd/fetchdesc` – Beschreibungen via Google Books API holen
- `DATABASE_URL=postgres://USER:PASSWORD@localhost:5432/ebook_search?sslmode=disable go run ./cmd/fixtitles` – Titelfehler korrigieren
- `docker compose up --build` – App + PostgreSQL starten
- `docker compose up --build -d` – im Hintergrund starten
- `docker compose --profile classify run --rm classify` – Bücher klassifizieren im Container (NAS, Ollama läuft ebenfalls in Docker)
