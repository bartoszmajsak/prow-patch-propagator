#!/bin/bash

set -euo pipefail

function placeholders() {
  local path=$1
  local cmd=( "sed" )

  for ((i=2; i<=$#; i=i+2))
  do
    local j=$((i+1))
    cmd+=( -e "s@${!i}@${!j}@g" )
  done
  cmd+=("${path}")

  "${cmd[@]}"
}

case ${1-noop} in
    placeholders) "$@"; exit;;
esac
