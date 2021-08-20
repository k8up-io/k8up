#!/bin/sh

# checks whether the PID in the given file exists


pidfile_exists() {
  test -f "${1}"
  return $?
}

pid_alive() {
  xargs ps -p >/dev/null < "${1}"
  return $?
}

if ! pidfile_exists "${1}"; then
  exit 0
fi

if ! pid_alive "${1}"; then
  rm "${1}"
fi
