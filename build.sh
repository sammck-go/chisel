#!/bin/bash
go build \
    -ldflags "-X github.com/sammck-go/chisel/share.BuildVersion=$(git describe --abbrev=0 --tags)" \
    -o chisel
