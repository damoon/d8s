#!/bin/sh
find /go/bin/wedding | entr -d -r wedding server $@
