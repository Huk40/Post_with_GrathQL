FROM golang:1.26.10-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/posts .

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/posts /usr/local/bin/posts
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["posts"]
CMD ["-addr=:8080", "-storage=memory"]

