FROM golang:1.23 AS builder

WORKDIR /app

ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.org

ENV GOPROXY=${GOPROXY}
ENV GOSUMDB=${GOSUMDB}

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/exchanged ./cmd/exchanged

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /out/exchanged /app/exchanged

EXPOSE 8080

ENTRYPOINT ["/app/exchanged"]
