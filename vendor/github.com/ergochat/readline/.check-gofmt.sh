#!/bin/bash

# exclude vendor/
SOURCES="*.go internal/ example/"

if [ "$1" = "--fix" ]; then
	exec gofmt -s -w $SOURCES
fi

if [ -n "$(gofmt -s -l $SOURCES)" ]; then
	echo "Go code is not formatted correctly with \`gofmt -s\`:"
	gofmt -s -d $SOURCES
	exit 1
fi
