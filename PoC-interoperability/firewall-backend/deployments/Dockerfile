FROM golang:1.23 AS build
WORKDIR /src
COPY . .
RUN go mod download
RUN go generate ./... || true
RUN CGO_ENABLED=0 go build -o /out/firewall-backend ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/firewall-backend /usr/local/bin/firewall-backend
COPY configs/config.yaml /app/configs/config.yaml
ENTRYPOINT ["/usr/local/bin/firewall-backend"]
