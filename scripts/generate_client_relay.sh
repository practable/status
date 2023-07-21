#!/bin/bash
rm -rf ../internal/relay/models
rm -rf ../internal/relay/restapi
swagger generate client -t ../internal/relay -f ../api/relay.yml -A relay
go mod tidy

