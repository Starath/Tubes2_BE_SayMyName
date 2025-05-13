FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o serverapp ./main_server.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/serverapp .

COPY elements_with_images.json .

EXPOSE 8080

CMD ["./serverapp"]