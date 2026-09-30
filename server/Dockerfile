# 微信云托管构建用。云托管默认把流量转到容器的 80 端口
FROM golang:1.23-alpine AS build
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY migrations ./migrations
RUN go build -trimpath -ldflags="-s -w" -o /out/server .

FROM alpine:3.20
# 以后调微信支付、订阅消息等 HTTPS 接口要用到根证书；时区数据已经编进二进制
RUN apk add --no-cache ca-certificates
COPY --from=build /out/server /app/server
EXPOSE 80
CMD ["/app/server"]
