FROM golang:alpine as builder

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download && go mod verify

COPY main.go ./

RUN go build -o ./app

FROM scratch

WORKDIR /bin

COPY --from=builder /build/app /app

ENTRYPOINT ["/app"]