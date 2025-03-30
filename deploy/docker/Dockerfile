FROM golang:1.24-alpine AS builder

WORKDIR /app

# Копируем только необходимые файлы для загрузки зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o payment-service ./cmd/server

# Финальный образ
FROM alpine:3.18

WORKDIR /app

# Установка необходимых пакетов
RUN apk --no-cache add ca-certificates tzdata

# Копируем бинарный файл из билдера
COPY --from=builder /app/payment-service .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/.env ./

# Определяем пользователя с меньшими привилегиями
RUN adduser -D appuser
USER appuser

EXPOSE 8080

CMD ["./payment-service"]