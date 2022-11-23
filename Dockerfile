FROM golang:1.18-alpine
WORKDIR /build

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

ARG DB_PASSWORD

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o astra-indexing ./cmd/astra-indexing

EXPOSE 8080

CMD ["./astra-indexing"]