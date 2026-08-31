FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/veille ./cmd/veille

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -H -u 10001 veille
WORKDIR /app
COPY --from=build /out/veille /app/veille
COPY migrations /app/migrations
USER veille
STOPSIGNAL SIGTERM
ENTRYPOINT ["/app/veille"]
