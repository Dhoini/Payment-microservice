FROM golang:1.24-alpine AS builder

# Устанавливаем необходимые зависимости
RUN apk add --no-cache gcc musl-dev git

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы go.mod и go.sum
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o payment-service ./cmd/payment-service

# Используем минимальный образ для запуска
FROM alpine:latest

# Устанавливаем необходимые зависимости для работы приложения
RUN apk --no-cache add ca-certificates tzdata

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем исполняемый файл из предыдущего этапа
COPY --from=builder /app/payment-service .
# Копируем файл конфигурации
COPY --from=builder /app/config.yml .

# Открываем порты для HTTP и gRPC
EXPOSE 8080 50051

# Устанавливаем переменную окружения
ENV APP_ENV=production

# Запускаем приложение
CMD ["./payment-service"]