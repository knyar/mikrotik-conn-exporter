FROM golang:1.19-alpine

COPY . /app
WORKDIR /app

RUN go build

EXPOSE 2514
EXPOSE 8122
USER nobody:nobody

ENTRYPOINT ["/app/mikrotik-conn-exporter"]
