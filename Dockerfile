# Этап 1: Сборка
FROM golang:1.25-alpine AS builder

# Устанавливаем зависимости ОС, необходимые для сборки
RUN apk add --no-cache git

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем модули и устанавливаем зависимости (кешируется Docker'ом)
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем бинарник (отключаем CGO для статической линковки)
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/server ./cmd/server

# Этап 2: Финальный легковесный образ
FROM alpine:latest  

# Устанавливаем ca-certificates для HTTPS (например, для запросов к API Gemini)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Копируем сбилженный бинарник
COPY --from=builder /app/bin/server .

# Копируем папку с SQL-миграциями
# Помните, что код читает "migrations/001_init.sql"
COPY --from=builder /app/migrations ./migrations

# Указываем, какой порт слушает приложение
EXPOSE 8080

# Запускаем сервер
CMD ["./server"]
