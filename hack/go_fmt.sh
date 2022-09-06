#!/bin/sh

cd "$(dirname "$0")/.." || exit 1

go fix ./...
gofmt -w -s -l .
goimports -w -l .
gofumpt -w -l .
go mod tidy -v

cd - || exit 2
