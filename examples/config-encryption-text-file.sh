#!/usr/bin/env bash

if [ "$1" = "get" ]
then    
    cat ~/.pinpoint/example-encryption-text-file-key 2>/dev/null
    exit 0
elif [ "$1" = "set" ]
then
    if [ "$2" = "" ]
    then
        echo "provide key value to set as 2nd arg"
        exit 1
    fi
    echo "$2" > ~/.pinpoint/example-encryption-text-file-key
else
echo "pass get or set as 1nd arg"
exit 1
fi