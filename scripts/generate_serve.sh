#!/bin/bash
rm -rf ../internal/serve/models
rm -rf ../internal/serve/restapi
swagger generate server -t ../internal/serve -f ../api/status.yml --exclude-main -A serve
go mod tidy

