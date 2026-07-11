
# --- build stage ---
FROM mirror.gcr.io/library/golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /bin/app ./cmd/surprise

# --- run stage ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /bin/app /app/app
COPY --from=build /src/configs /app/configs
ENV CONFIG_PATH=/app/configs/values_docker.yaml
EXPOSE 9090 8080
ENTRYPOINT ["/app/app"]
