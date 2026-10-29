DB_URL=./purplefeed.db
MIG_DIR=sql/schema

db-up:
	goose -dir $(MIG_DIR) sqlite3 "$(DB_URL)" up

db-down:
	goose -dir $(MIG_DIR) sqlite3 "$(DB_URL)" down

db-reset:
	goose -dir $(MIG_DIR) sqlite3 "$(DB_URL)" reset

db-wipe:
	rm -f "$(DB_URL)"

db-fresh: db-wipe db-up

db-status:
	goose -dir $(MIG_DIR) sqlite3 "$(DB_URL)" status

test:
	go test ./...

test-clean: db-fresh test