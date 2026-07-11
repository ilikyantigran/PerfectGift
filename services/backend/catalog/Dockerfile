
# --- build stage ---
FROM mirror.gcr.io/library/golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /bin/app ./cmd/catalog

# --- run stage ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /bin/app /app/app
COPY --from=build /src/configs /app/configs
ENV CONFIG_PATH=/app/configs/values_docker.yaml
EXPOSE 9096 8096
ENTRYPOINT ["/app/app"]
