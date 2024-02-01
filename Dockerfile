FROM golang:1.21.6

ENV GOPATH=/

WORKDIR /go/src/zvrtsh
COPY . .

RUN go mod download
RUN go build -o zvrtsh-app cmd/server/main.go

CMD ["./zvrtsh-app"]
