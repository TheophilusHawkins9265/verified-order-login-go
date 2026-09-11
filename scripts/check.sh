#!/bin/sh
set -eu

go test ./...
go build -o /tmp/verified-order-login ./cmd/order-login
