# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /bin/app ./cmd/identity

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /bin/app /app/app
COPY --from=build /src/configs /app/configs
# Migrations are embedded in the binary (see internal/domain/postgres/migrate.go)
# and applied automatically at startup.
ENV CONFIG_PATH=/app/configs/values_docker.yaml
EXPOSE 8080 8081
ENTRYPOINT ["/app/app"]
