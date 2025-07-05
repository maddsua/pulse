from docker.io/golang:1.23.2-alpine3.20 as builder

workdir /app

copy . .

run go mod download
run go build -v -ldflags "-s -w" -o pulse-cmd ./cmd

from alpine:3.20

run apk add --no-cache ca-certificates

copy --from=builder /app/pulse-cmd /usr/bin/pulse
copy ./cmd/pulse.yml /etc/mws/pulse/pulse.yml

entrypoint ["/usr/bin/pulse"]
