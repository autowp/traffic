FROM golang:1.15-alpine as builder

RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates

ENV USER=appuser
ENV UID=10001

# See https://stackoverflow.com/a/55757473/12429735RUN
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"
WORKDIR $GOPATH/src/github.com/autowp/traffic/
COPY . $GOPATH/src/github.com/autowp/traffic/

RUN cd $GOPATH/src/github.com/autowp/traffic/ && \
    go mod download && \
    go mod verify && \
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o /app cmd/traffic/traffic.go

############################
FROM scratch

LABEL app_name="autowp.traffic"
LABEL maintainer="dmitry@pereslegin.ru"

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

COPY --from=builder /app /app
COPY migrations /migrations
COPY defaults.yaml /defaults.yaml

USER appuser:appuser

ENTRYPOINT ["/app"]
