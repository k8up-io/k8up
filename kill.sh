#!/bin/sh

if [ "${#}" != "1" ]; then
  echo "Usage: ${0} <pidfile>"
  exit 1
fi

if [ -f "${1}" ]; then
  xargs kill < "${1}"
  rm -f "${1}"
fi
