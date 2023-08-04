#!/bin/bash
# usage ./check.sh manifest.yaml
yamllint $1
.bin/book manifest check  $1
c=$(cat $1 | grep '}}}' | wc -l)
if ! [ $c -eq 0 ]; then
    echo "too many curly braces - app may not work!"
	cat $1 | grep '}}}'
fi
c=$(cat $1 | grep '{{{' | wc -l)
if ! [ $c -eq 0 ]; then
    echo "too many curly braces - app may not work!"
	cat $1 | grep '{{{'
fi
