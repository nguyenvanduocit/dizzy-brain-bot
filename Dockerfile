FROM golang:latest as builder

WORKDIR /codebase

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . /codebase
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ./bin/server .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /codebase/bin/server ./

ENTRYPOINT ["./server"]
