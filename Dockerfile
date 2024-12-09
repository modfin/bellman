FROM golang:alpine AS builder


RUN mkdir /app
COPY . /app
RUN cd /app/bellmand && go build -o /bellmand .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /bellmand /bellmand

ENTRYPOINT ["/bellmand"]