FROM golang:1.24-alpine AS builder

WORKDIR /home
COPY . .
RUN rm /home/docker -rf

RUN export GOPROXY=https://goproxy.cn,direct && export CGO_ENABLED=0 && export GOARCH=amd64 && export GOOS=linux && go mod tidy && go build -o /usr/bin/rangine *.go

FROM busybox

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crtdddddw
ENV TZ Asia/Shanghai

COPY --from=builder /usr/bin/rangine /home/rangine

CMD ["/home/rangine", "server:start"]
