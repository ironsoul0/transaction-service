FROM golang:1.17-alpine as builder

WORKDIR /app
COPY . .
RUN go build && \
      chmod 777 transactions-service

FROM alpine:latest
WORKDIR /root/
COPY app.env .
COPY --from=builder /app/transactions-service .
CMD [ "./transactions-service" ]