FROM golang:1.27.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api

FROM alpine:3.22
RUN apk --no-cache add ca-certificates wget
WORKDIR /app
COPY --from=builder /out/api ./api
EXPOSE 9005
CMD ["./api"]
