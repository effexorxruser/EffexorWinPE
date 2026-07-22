FROM golang:1.24-alpine AS build

RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/effexorwinpe-gateway ./cmd/effexorwinpe-gateway

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/effexorwinpe-gateway /effexorwinpe-gateway
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/effexorwinpe-gateway"]
