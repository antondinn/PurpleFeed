# PurpleFeed

A Go command-line RSS aggregator that organizes feeds into categories and helps curate articles in an [Obsidian](https://obsidian.md/) vault.

## Features

- Create, activate, deactivate, and remove RSS feed categories.
- Add and remove RSS feeds within categories.
- Fetch recent articles from active categories and generate an Obsidian inbox note.
- Save selected articles from the inbox to a categorized saved-articles note.
- Clean up checked items from Obsidian notes.
- Persist categories, feeds, and posts in SQLite.

## Tech stack

Go · SQLite · sqlc · goose migrations · RSS/XML · Obsidian Markdown

## Getting started

**Requirements:** Go 1.24+; [`goose`](https://github.com/pressly/goose) CLI for database migrations; an Obsidian vault.

```powershell
# From the project directory, initialize the SQLite schema:
goose -dir sql/schema sqlite3 ./purplefeed.db up

# Configure the local database and Obsidian vault:
go run . config db ./purplefeed.db
go run . config vault "C:\path\to\your\ObsidianVault"
go run . config pull "Inbox.md"
go run . config save "Saved Articles.md"

# View available commands:
go run . help

# Create a category and add feeds:
go run . addcat Technology
go run . addfeeds Technology

# Collect recent posts and create your Obsidian inbox:
go run . pullpurp

# Save checked articles from the inbox:
go run . savepurp
```

`addfeeds` prompts for RSS feed URLs interactively; enter `end` to finish. Configuration is stored in your user home directory, not in the repository.

## Project structure

- `main.go` — CLI commands and Obsidian workflow
- `internal/rss/` — RSS fetching and parsing
- `internal/database/` — generated database access layer
- `internal/config/` — local configuration management
- `sql/schema/` — SQLite migrations
- `sql/queries/` — SQL queries used by sqlc

## Status

Personal learning project; improvements and additional tests are ongoing.
