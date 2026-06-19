FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM builder AS server-builder
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server services/topics/cmd/server/main.go

FROM builder AS seed-builder
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o seed services/topics/cmd/seed/main.go

FROM alpine:3.19 AS server
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=server-builder /app/server .
CMD ["./server"]

FROM alpine:3.19 AS seed
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=seed-builder /app/seed .
CMD ["./seed"]
