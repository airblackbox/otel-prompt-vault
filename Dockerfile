FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /otelcol-promptvault ./cmd/otelcol-promptvault/
RUN CGO_ENABLED=0 go build -o /promptvaultctl ./cmd/promptvaultctl/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates

COPY --from=builder /otelcol-promptvault /usr/local/bin/otelcol-promptvault
COPY --from=builder /promptvaultctl /usr/local/bin/promptvaultctl

EXPOSE 4317 4318 13133

ENTRYPOINT ["otelcol-promptvault"]
CMD ["--config", "/etc/otelcol/config.yaml"]
