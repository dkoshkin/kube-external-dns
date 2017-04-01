FROM alpine
EXPOSE 8080

RUN apk update && apk add ca-certificates

ADD kube-external-dns /app

CMD ["/app"]
