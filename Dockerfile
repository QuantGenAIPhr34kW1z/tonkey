FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/tonkeyd ./cmd/tonkeyd

FROM debian:stable-slim
WORKDIR /app
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/tonkeyd /app/tonkeyd
EXPOSE 8080
ENTRYPOINT ["/app/tonkeyd"]
