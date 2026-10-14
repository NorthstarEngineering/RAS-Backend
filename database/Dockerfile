FROM golang:1.27.2-alpine AS build
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
FROM alpine:3.22
RUN apk add --no-cache ca-certificates postgresql-client
COPY --from=build /go/bin/goose /usr/local/bin/goose
COPY database/migrations /migrations
COPY database/bootstrap/roles.sql /database/bootstrap/roles.sql
ENTRYPOINT ["goose"]
