FROM golang:1.26-alpine AS build

RUN CGO_ENABLED=0 go install github.com/pressly/goose/v3/cmd/goose@v3.27.0

FROM alpine:3.23

RUN apk add --no-cache ca-certificates
COPY --from=build /go/bin/goose /usr/local/bin/goose
WORKDIR /migrations
COPY auth ./auth
COPY analyzer ./analyzer
COPY report ./report
