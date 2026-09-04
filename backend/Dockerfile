# Бэкенд: сборка статического бинарника и запуск на голом alpine.
# Драйверы Postgres и Redis на чистом Go, поэтому CGO не нужен.
# Go 1.25: этого требует gotd/td, клиент MTProto (решение №52).
FROM golang:1.25-alpine AS build
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

# Данные лежат в Postgres, адреса приходят из окружения (docker-compose.yml).
ENV ADDR=:8000
USER app
EXPOSE 8000
CMD ["/app/server"]
