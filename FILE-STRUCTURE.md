# Ebook-Bibliothek: Verzeichnisstruktur

## Struktur

```
/ebooks/
└── Author Name/
    └── Book Title/
        ├── book.epub
        ├── book.pdf      (optional, weiteres Format)
        └── cover.jpg     (optional)
```

Zweistufige Hierarchie: **Autor → Titel → Dateien**. Alle Formate eines Buches liegen im selben Ordner.

---

## Namenskonventionen

| Element | Regel | Beispiel |
|---------|-------|---------|
| Autorenname | Natürliche Reihenfolge | `Douglas Adams` |
| Mehrere Autoren | Mit ` & ` verbinden | `Terry Pratchett & Neil Gaiman` |
| Unbekannter Autor | Ordnername `Unknown` | `Unknown/` |
| Buchtitel | Originaltitel; Doppelpunkt → ` - ` | `Good Omens - The Nice and Accurate Prophecies` |
| Sonderzeichen | `/`, `?`, `*`, `<`, `>`, `"` entfernen oder ersetzen | |
| Dateinamen | Immer `book.epub`, `book.pdf`, `book.mobi` | |

---

## Beispiele

```
/ebooks/
├── Douglas Adams/
│   ├── The Hitchhiker's Guide to the Galaxy/
│   │   ├── book.epub
│   │   └── cover.jpg
│   └── The Restaurant at the End of the Universe/
│       └── book.epub
├── Martin Fowler/
│   └── Refactoring - Improving the Design of Existing Code/
│       ├── book.epub
│       └── book.pdf
├── Terry Pratchett & Neil Gaiman/
│   └── Good Omens/
│       └── book.epub
└── Unknown/
    └── Some Untitled Scan/
        └── book.pdf
```

---

## Bewusste Entscheidungen

**Kein Genre-Ordner als Top-Level** (`Fiction/Author/Title/`): Bücher gehören oft zu mehreren
Genres. Genre wird in der Datenbank verwaltet (via `cmd/classify`), nicht im Dateisystem.

**Kein `metadata.json`-Sidecar**: Metadaten (Titel, Autor, Genre, Beschreibung) liegen in der
Datenbank. Ein JSON-Sidecar wäre ein zweiter Sync-Point ohne Mehrwert.

**Keine Flat-Struktur** (`Author - Title.epub`): Skaliert nicht bei >1000 Büchern, und mehrere
Formate pro Buch lassen sich nicht sauber gruppieren.

---

## Migration von Calibre

Calibre verwendet die Struktur `Author/Title (CalibreID)/file.ext`. Das Migrationsskript
(`cmd/migrate`, noch zu implementieren) soll:

1. Calibre-Verzeichnis einlesen
2. ID-Suffix aus Ordnernamen entfernen: `The Hobbit (42)` → `The Hobbit`
3. Datei als `book.ext` in neues Verzeichnis kopieren
4. Konflikte (gleicher Titel, anderer Inhalt) melden

Nach der Migration: `cmd/import` auf das neue Verzeichnis laufen lassen.
