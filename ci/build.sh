#!/bin/bash

export GO111MODULE="on"
go install github.com/mitchellh/gox@latest # cross compile
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build  -buildmode=plugin -ldflags "-X main.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.gitRevision=`git describe --tags || git rev-parse HEAD` -s -w"
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build  -buildmode=plugin -tags 'watch' -o ggr-watch -ldflags "-X main.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.gitRevision=`git describe --tags || git rev-parse HEAD` -s -w"
gox -os "linux darwin windows" -buildmode=plugin -arch "amd64" -osarch="windows/386" -output "dist/{{.Dir}}_{{.OS}}_{{.Arch}}" -ldflags "-X main.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.gitRevision=`git describe --tags || git rev-parse HEAD` -s -w"