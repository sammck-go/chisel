#!/bin/bash
go build \
    -ldflags "-X github.com/sammck/chisel/share.BuildVersion=$(git describe --abbrev=0 --tags)" \
    -o chisel
