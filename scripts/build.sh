#!/bin/bash

VERSION=$(cat version)
echo "building terraform-provider-axiom_${VERSION}"
go build -o terraform-provider-axiom_${VERSION}