# Build stuff
FROM golang:1.20-alpine as builder

WORKDIR /app

COPY go.mod ./
RUN go mod download
RUN go mod verify

COPY . .

WORKDIR /app/cmd

RUN go build -o ./build/listener-build

# Build tiny image
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/cmd/build/listener-build /app

# Don't run as root
RUN adduser -D -u 1001 bogo
RUN chown -R bogo:bogo /app
USER bogo

CMD ["./listener-build"]