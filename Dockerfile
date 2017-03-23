FROM golang:1.8-alpine
EXPOSE 8080

ADD kube-external-dns /app

CMD ["/app"]

