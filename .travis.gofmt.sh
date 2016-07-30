#!/bin/bash

cd "$(dirname $0)"

BADLY_FORMATTED="$(go fmt ./...)"

if [[ -n $BADLY_FORMATTED ]]; then
  echo "The following files are badly formatted: $BADLY_FORMATTED"
  exit 1
fi
