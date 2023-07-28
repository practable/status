#!/bin/bash
export MAIL=~/secret/zoho/mail.yaml
export STATUS_BASEPATH_BOOK=/dev/book
export STATUS_BASEPATH_JUMP=/dev/jump
export STATUS_BASEPATH_RELAY=/dev/access
export STATUS_EMAIL_FROM=$(yq -r '.username' $MAIL)
export STATUS_EMAIL_HOST=$(yq -r '.host' $MAIL)
export STATUS_EMAIL_LINK=https://app.practable.io/dev/status
export STATUS_EMAIL_PASSWORD=$(yq -r '.password' $MAIL)
export STATUS_EMAIL_PORT=(yq r '.port' $MAIL)
export STATUS_EMAIL_SUBJECT=app.practable.io/dev
# while our send to addresses are not secret, this step avoids receiving spam
export STATUS_EMAIL_TO=$(yq -r '.to' $MAIL)
export STATUS_HOST_BOOK=app.practable.io
# in production use >5m
export STATUS_HEALTH_EVENTS=100
export STATUS_HEALTH_LAST=10s
export STATUS_HEALTH_LOG_EVERY=10s
export STATUS_HEALTH_STARTUP=5s
export STATUS_HOST_JUMP=app.practable.io
export STATUS_HOST_RELAY=app.practable.io
export STATUS_LOG_LEVEL=trace
export STATUS_LOG_FORMAT=text
export STATUS_LOG_FILE=stdout
export STATUS_PORT_PROFILE=6061
export STATUS_PORT_SERVE=3007
export STATUS_PROFILE=false
# in production use > 15min 
export STATUS_QUERY_BOOK_EVERY=15s
export STATUS_RECONNECT_JUMP_EVERY=30s
export STATUS_RECONNECT_RELAY_EVERY=30s
export STATUS_SCHEME_BOOK=https
export STATUS_SCHEME_JUMP=https
export STATUS_SCHEME_RELAY=https
export STATUS_SECRET_JUMP=$(cat ~/secret/app.practable.io/dev/jump.pat)
export STATUS_SECRET_BOOK=$(cat ~/secret/app.practable.io/dev/book.pat)
export STATUS_SECRET_RELAY=$(cat ~/secret/app.practable.io/dev/relay.pat)
export STATUS_TIMEOUT_BOOK=5s
../cmd/status/status serve 
