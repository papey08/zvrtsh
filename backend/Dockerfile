FROM golang:1.26 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /zvrtsh-app ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /zvrtsh-app ./zvrtsh-app
COPY config/config.yml ./config/config.yml

CMD ["./zvrtsh-app"]
