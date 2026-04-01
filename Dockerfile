FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod .
COPY *.go .
RUN go build -o bpm .

FROM alpine:3.20
RUN apk add --no-cache ffmpeg
WORKDIR /app
COPY --from=builder /src/bpm .
EXPOSE 8080
ENTRYPOINT ["./bpm", "-serve", "-port", "8080"]
