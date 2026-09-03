# Бэкенд: сборка статического бинарника и запуск на голом alpine.
# Драйвер SQLite чистый на Go, поэтому CGO не нужен.
FROM golang:1.24-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM alpine:3.20
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/seed /app/seed

# База лежит в томе: пересборка образа не стирает данные.
ENV DATABASE_PATH=/data/personal_inbox.db \
    ADDR=:8000
RUN mkdir -p /data && chown app:app /data
USER app
VOLUME ["/data"]
EXPOSE 8000
CMD ["/app/server"]
