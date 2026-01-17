.PHONY: dev-up dev-down migrate-up proto-gen

dev-up:
	docker compose -f infra/envs/dev/compose.yaml up -d

dev-down:
	docker compose -f infra/envs/dev/compose.yaml down

migrate-up:
	docker run --rm -v $(PWD)/server/apps/accounts/db/migrations:/migrations --network host migrate/migrate \
		-path=/migrations/ -database "postgres://user:password@localhost:5432/accounts_db?sslmode=disable" up

proto-gen:
	buf generate