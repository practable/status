#!/bin/bash
go=$(which go)
if [ ! $? -eq 0 ]; then
   echo "please install go then try again"
fi

mkdir .source 2> /dev/null || true
mkdir .bin 2> /dev/null || true

cd .source
if cd book 2> /dev/null; then git pull origin main; else git clone https://github.com/practable/book && cd book; fi
cd scripts
./build.sh
cp ../cmd/book/book ../../../.bin/book
