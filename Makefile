# PerfectGift — local Docker stack convenience targets.
# Requires Docker Desktop (or a running Docker daemon) + Docker Compose v2+.

.DEFAULT_GOAL := help
COMPOSE := docker compose

.PHONY: help up build down clean logs ps restart psql nats-info

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

up: ## Build (if needed) and start the whole stack in the background
	$(COMPOSE) up --build -d

build: ## Build all service images
	$(COMPOSE) build

down: ## Stop the stack (keep data)
	$(COMPOSE) down

clean: ## Stop the stack and delete the Postgres volume (fresh DB next up)
	$(COMPOSE) down -v

logs: ## Tail logs for all services (use `make logs S=gateway` for one)
	$(COMPOSE) logs -f $(S)

ps: ## Show container status / health
	$(COMPOSE) ps

restart: ## Restart one service: `make restart S=surprise`
	$(COMPOSE) restart $(S)

psql: ## Open a psql shell in the Postgres container
	$(COMPOSE) exec postgres psql -U postgres

nats-info: ## Show NATS JetStream monitoring summary
	@curl -s http://localhost:8222/jsz | head -40 || echo "nats monitoring not reachable"
