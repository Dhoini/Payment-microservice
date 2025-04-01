# Makefile for Payment Microservice Migrations

# --- Configuration ---
# Получаем DSN из переменной окружения или используем дефолтное значение (для примера)
# В CI/CD или локально лучше устанавливать DB_DSN как переменную окружения
# Пример чтения из config.yml (требует yq: brew install yq):
# CONFIG_FILE ?= config.yml
# DB_DSN ?= $(shell yq e '.database.dsn' $(CONFIG_FILE))
# Простой вариант с переменной окружения:
DB_DSN ?= postgres://admin:secretPass@localhost:5432/payment?sslmode=disable
# Убедитесь, что DSN для миграций указывает на ВАШУ ЛОКАЛЬНУЮ или ТЕСТОВУЮ БД, а не на production!
# Если используете Docker, возможно localhost нужно заменить на имя сервиса postgres (payment-postgres)
# DB_DSN_DOCKER ?= postgres://admin:secretPass@payment-postgres:5432/payment?sslmode=disable

# Директория с миграциями
MIGRATIONS_DIR := migrations
MIGRATIONS_PATH := file://$(MIGRATIONS_DIR)

# Исполняемый файл migrate
# Если вы установили его глобально и он в PATH, можно просто 'migrate'
# Если скачали бинарник, укажите путь к нему
MIGRATE_BIN := migrate
# Проверка наличия migrate
HAS_MIGRATE := $(shell command -v $(MIGRATE_BIN) 2> /dev/null)

.PHONY: help migrate-check migrate-create migrate-up migrate-up-one migrate-down migrate-down-one migrate-status migrate-force

help: ## Показывает эту справку
	@echo "Доступные команды для миграций:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo "\nПеременные окружения:"
	@echo "  DB_DSN        : Строка подключения к БД (по умолчанию: '$(DB_DSN)')"
	@echo "  MIGRATIONS_DIR: Директория с миграциями (по умолчанию: '$(MIGRATIONS_DIR)')"
	@echo "Пример создания миграции: make migrate-create NAME=add_some_column"

# Проверка, что migrate установлен
migrate-check:
ifndef HAS_MIGRATE
	$(error "migrate CLI not found. Please install it: https://github.com/golang-migrate/migrate/tree/master/cmd/migrate#installation")
endif

migrate-create: migrate-check ## Создать новую миграцию (make migrate-create NAME=migration_name)
	@read -p "Введите имя миграции (например, add_user_email): " name; \
	if [ -z "$$name" ]; then echo "Имя миграции не может быть пустым!"; exit 1; fi; \
	echo "Создание миграции с именем: $$name"; \
	$(MIGRATE_BIN) create -ext sql -dir $(MIGRATIONS_DIR) -seq $$name

# Example (replace DB_URL with your actual database connection variable if needed)
migrate-up:
	@echo "Применение миграций к БД: $(DB_URL)"
	migrate -database "$(DB_URL)" -path file://migrations up

migrate-up-one: migrate-check ## Применить следующую миграцию
	@echo "Применение следующей миграции к БД: $(DB_DSN)"
	$(MIGRATE_BIN) -database "$(DB_DSN)" -path $(MIGRATIONS_PATH) up 1

migrate-down: migrate-check ## Откатить последнюю примененную миграцию
	@echo "Откат последней миграции из БД: $(DB_DSN)"
	$(MIGRATE_BIN) -database "$(DB_DSN)" -path $(MIGRATIONS_PATH) down 1

migrate-down-all: migrate-check ## Откатить ВСЕ миграции (ОСТОРОЖНО!)
	@read -p "Вы уверены, что хотите откатить ВСЕ миграции? [y/N]: " confirm && [[ $$confirm == [yY] ]] || exit 1
	@echo "Откат ВСЕХ миграций из БД: $(DB_DSN)"
	$(MIGRATE_BIN) -database "$(DB_DSN)" -path $(MIGRATIONS_PATH) down -all

migrate-status: migrate-check ## Показать статус миграций
	@echo "Проверка статуса миграций для БД: $(DB_DSN)"
	@$(MIGRATE_BIN) -database "$(DB_DSN)" -path $(MIGRATIONS_PATH) version
	@$(MIGRATE_BIN) -database "$(DB_DSN)" -path $(MIGRATIONS_PATH) dirty # Проверить на dirty state (не обязательно)

# Использовать с осторожностью!
migrate-force: migrate-check ## Установить определенную версию миграции (ОСТОРОЖНО!) (make migrate-force VERSION=N)
	@read -p "Введите номер версии для установки: " version; \
	if [ -z "$$version" ]; then echo "Версия не указана!"; exit 1; fi; \
	@read -p "Вы уверены, что хотите принудительно установить версию $$version? Это может привести к несоответствию схемы! [y/N]: " confirm && [[ $$confirm == [yY] ]] || exit 1
	@echo "Принудительная установка версии $$version для БД: $(DB_DSN)"
	$(MIGRATE_BIN) -database "$(DB_DSN)" -path $(MIGRATIONS_PATH) force $$version