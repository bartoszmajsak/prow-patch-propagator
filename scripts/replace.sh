#!/bin/bash

set -euo pipefail

function placeholders() {
  local path=$1
  local old=$2
  local new=$3
  local cmd=( "sed" )

  if [ -n "${4+x}" ]; then
      cmd+=( -i )
  fi
  cmd+=( -e "s@${old}@${new}@g" "${path}")

  "${cmd[@]}"
}

case ${1-noop} in
    placeholders) "$@"; exit;;
esac
