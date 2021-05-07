FROM golang:alpine as build
COPY ./src /usr/src/app
WORKDIR /usr/src/app
RUN go build -v

FROM alpine:3.13

RUN apk add postgresql-client

COPY --from=build /usr/src/app/pg-utils-web-adapter /app/

CMD ["/app/pg-utils-web-adapter"]