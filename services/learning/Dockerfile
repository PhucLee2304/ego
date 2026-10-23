FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM builder AS server-builder
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server services/learning/cmd/server/main.go

# Target seed được bật ở P12 khi có services/learning/cmd/seed.
# FROM builder AS seed-builder
# RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o seed ./services/learning/cmd/seed

FROM alpine:3.19 AS server
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=server-builder /app/server .
CMD ["./server"]

# FROM alpine:3.19 AS seed
# RUN apk add --no-cache ca-certificates
# WORKDIR /app
# COPY --from=seed-builder /app/seed .
# CMD ["./seed"]
