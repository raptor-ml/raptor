#!/bin/bash
output=$(go-licenses check ./... 1>/dev/null 2>&1)
if [ -z "$output" ]; then
    echo -e "\033[0;32mLicense Check Success\033[0m"
    exit 0
else
    echo -e "\033[0;31mLicense Check Failed - You're importing a blocked license:\033[0m"
    echo "$output"
    exit 1
fi