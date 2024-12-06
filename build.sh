#!/bin/bash
go build -ldflags="-w -s -X 'main.buildUseEmbedFs=true' -buildid=" -trimpath -buildvcs=false