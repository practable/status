#!/bin/bash
export STATUS_BASEPATH_BOOK=/tenant/book
export STATUS_BASEPATH_JUMP=/jump
export STATUS_BASEPATH_RELAY=/access
export STATUS_EMAIL_FROM=other@b.org
export STATUS_EMAIL_HOST=stmp.b.org
export STATUS_EMAIL_PASSWORD=something
export STATUS_EMAIL_PORT=587
export STATUS_EMAIL_TO=some@a.org
export STATUS_HOST_BOOK=app.practable.io
export STATUS_HOST_JUMP=dev.practable.io
export STATUS_HOST_RELAY=dev.practable.io
export STATUS_LOG_LEVEL=debug
export STATUS_LOG_FORMAT=text
export STATUS_LOG_FILE=stdout
export STATUS_PORT_PROFILE=6061
export STATUS_PORT_SERVE=3007
export STATUS_PROFILE=false
export STATUS_RECONNECT_JUMP_EVERY=15
export STATUS_RECONNECT_RELAY_EVERY=15
export STATUS_SCHEME_BOOK=https
export STATUS_SCHEME_JUMP=https
export STATUS_SCHEME_RELAY=https
export STATUS_SECRET_JUMP=$(cat ~/secret/v0/jump.pat)
export STATUS_SECRET_BOOK=$(cat ~/secret/v0/book.pat)
export STATUS_SECRET_RELAY=$(cat ~/secret/v0/relay.pat)
../cmd/status/status serve 
