FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/manage ./cmd/manage

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 app
COPY --from=build /out/ /app/
USER 10001:10001
WORKDIR /app
# Configuration comes only from the process environment (no .env in the image).
ENV PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null "http://127.0.0.1:${PORT}/api/v1/health" || exit 1
ENTRYPOINT ["/app/api"]
