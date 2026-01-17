# ==============================================================================
# Global Configuration
# ==============================================================================

# インフラ設定
COMPOSE_FILE := infra/envs/dev/compose.yaml
DB_CONTAINER := dev_postgres

DB_USER := app_user
DB_PASS := app_password
DB_HOST := localhost:5432
MIGRATE_IMG := migrate/migrate

# ==============================================================================
# Service Specific Configuration
# ==============================================================================

# --- Accounts Service ---
ACCOUNTS_DB   := accounts_db
ACCOUNTS_MIG  := $(PWD)/server/services/accounts/db/migrations
ACCOUNTS_DSN  := postgres://$(DB_USER):$(DB_PASS)@$(DB_HOST)/$(ACCOUNTS_DB)?sslmode=disable

# --- Drive Service ---
DRIVE_DB      := drive_db
DRIVE_MIG     := $(PWD)/server/services/drive/db/migrations
DRIVE_DSN     := postgres://$(DB_USER):$(DB_PASS)@$(DB_HOST)/$(DRIVE_DB)?sslmode=disable

# ==============================================================================
# Macros (Reusable Functions)
# ==============================================================================

# DB作成マクロ: $(call create_db, db_name)
define create_db
	@echo "Creating Database: $(1)..."
	@docker exec $(DB_CONTAINER) psql -U $(DB_USER) -c "CREATE DATABASE $(1);" 2>/dev/null \
		&& echo "  -> Successfully created." \
		|| echo "  -> $(1) likely exists (skipping)."
	@docker exec $(DB_CONTAINER) psql -U $(DB_USER) -c "GRANT ALL PRIVILEGES ON DATABASE $(1) TO $(DB_USER);" >/dev/null 2>&1 || true
endef

# Migration実行マクロ: $(call run_migrate, migration_path, db_dsn)
define run_migrate
	@echo "Migrating: $(2) ..."
	@docker run --rm -v $(1):/migrations --network host $(MIGRATE_IMG) \
		-path=/migrations/ -database "$(2)" up
endef

# ==============================================================================
# Main Commands
# ==============================================================================
.PHONY: up down help

up: ## 開発環境(DB等)を起動
	docker compose -f $(COMPOSE_FILE) up -d

down: ## 開発環境を停止
	docker compose -f $(COMPOSE_FILE) down

down-v: 
	docker compose -f $(COMPOSE_FILE) down -v

# ==============================================================================
# Database & Migrations (Accounts)
# ==============================================================================
.PHONY: create-db-accounts migrate-accounts

create-db-accounts: ## Accounts用のDBを作成
	$(call create_db,$(ACCOUNTS_DB))

migrate-accounts: ## Accounts用のマイグレーション実行
	$(call run_migrate,$(ACCOUNTS_MIG),$(ACCOUNTS_DSN))

# ==============================================================================
# Database & Migrations (Drive)
# ==============================================================================
.PHONY: create-db-drive migrate-drive

create-db-drive: ## Drive用のDBを作成
	$(call create_db,$(DRIVE_DB))

migrate-drive: ## Drive用のマイグレーション実行
	$(call run_migrate,$(DRIVE_MIG),$(DRIVE_DSN))

# ==============================================================================
# Generators
# ==============================================================================
.PHONY: gen

gen: ## ProtoからGoコードを生成
	buf generate