#!/bin/bash

export GO111MODULE="on"
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build  -buildmode=plugin -ldflags "-X main.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.gitRevision=`git describe --tags || git rev-parse HEAD` -s -w"
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build  -buildmode=plugin -tags 'watch' -o ggr-watch -ldflags "-X main.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.gitRevision=`git describe --tags || git rev-parse HEAD` -s -w"
