#!/bin/bash
export GOOS=linux
now=$(date +'%Y-%m-%d_%T') #no spaces to prevent quotation issues in build command
(cd ../cmd/status; go build -ldflags "-X 'github.com/practable/status/cmd/status/cmd.Version=`git describe --tags`' -X 'github.com/practable/status/cmd/status/cmd.BuildTime=$now'"; ./status version)
