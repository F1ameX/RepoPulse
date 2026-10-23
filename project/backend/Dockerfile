FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG SERVICE=api
RUN CGO_ENABLED=0 go build -o /service ./cmd/${SERVICE}

FROM alpine:3.23

RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /service ./service
ENTRYPOINT ["/app/service"]
