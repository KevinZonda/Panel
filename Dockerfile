FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /out/panel ./cmd/panel

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends nginx procps ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /var/lib/kvzd-panel /etc/kvzd-panel /var/cert /var/cert-self-sign

COPY --from=build /out/panel /usr/local/bin/panel
COPY deploy/docker/config.json /usr/local/share/kvzd-panel/docker-defaults.json
COPY deploy/docker/entrypoint.sh /entrypoint.sh
COPY deploy/docker/index.html /var/www/html/index.html
RUN chmod +x /entrypoint.sh

EXPOSE 8080 80
ENTRYPOINT ["/entrypoint.sh"]
