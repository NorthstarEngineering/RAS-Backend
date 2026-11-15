FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api

FROM alpine:3.22
RUN apk --no-cache add ca-certificates wget
WORKDIR /app
COPY --from=build /out/api ./api
EXPOSE 9004
CMD ["./api"]
