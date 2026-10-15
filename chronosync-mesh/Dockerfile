FROM golang:1.21-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/chronosync-proxy ./cmd/proxy

FROM alpine:3.20
RUN adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=build /out/chronosync-proxy /app/chronosync-proxy
COPY config/config.yaml /app/config/config.yaml
USER app
EXPOSE 8080
ENTRYPOINT ["/app/chronosync-proxy"]
