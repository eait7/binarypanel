.PHONY: up down build logs status restart clean

up: build ## Start BinaryPanel ecosystem
	docker compose up -d

down: ## Stop everything
	docker compose down

build: ## Build the dashboard image
	docker compose build dashboard

logs: ## Tail all logs
	docker compose logs -f

status: ## Show running services
	docker compose ps

restart: ## Restart everything
	docker compose restart

clean: ## Full teardown (preserves data volumes)
	docker compose down --remove-orphans
