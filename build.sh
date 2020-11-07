#!/bin/bash

go build -ldflags "-s -w" -v -o crontab-controller .
