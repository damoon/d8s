#!/bin/sh
find /go/bin/dinner | entr -d -r dinner server $@
