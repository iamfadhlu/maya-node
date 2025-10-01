#!/bin/bash

echo "Linting managers.go file"

inited=$(grep "return new" x/mayachain/managers.go | awk '{print $2}' | awk -F "(" '{print $1}')
created=$(grep --exclude "*_test.go" "func new" x/mayachain/manager_* | awk '{print $2}' | awk -F "(" '{print $1}')
missing=$(echo -e "$inited\n$created" | grep -Ev 'Dummy|Helper|newStoreMgr|newSwapper' | sort -n | uniq -u)
echo "$missing"

[ -z "$missing" ] && echo "OK" && exit 0

[[ -n $missing ]] && echo "Not OK" && exit 1
