# --- build stage ---
# gcr.io mirror: Docker Hub is rate-limited on the build machine.
FROM mirror.gcr.io/library/golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /bin/app ./cmd/log-server
# Pre-create the data dir owned by the nonroot uid so a freshly-mounted
# volume inherits writable ownership (Docker seeds empty volumes from the image).
RUN mkdir -p /data && chown 65532:65532 /data

# --- run stage ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /bin/app /app/app
COPY --from=build /src/configs /app/configs
COPY --from=build --chown=65532:65532 /data /data
ENV CONFIG_PATH=/app/configs/values_docker.yaml
EXPOSE 8086
VOLUME ["/data"]
ENTRYPOINT ["/app/app"]
