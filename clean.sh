#!/bin/sh

# checks whether the PID in the given file exists


pidfile_exists() {
  test -f "${1}"
  return $?
}

pid_alive() {
  if ps --help 2>&1 | grep -q BusyBox; then
    xargs ps p >/dev/null < "${1}"
  else
    xargs ps -p >/dev/null < "${1}"
  fi

  return $?
}

if ! pidfile_exists "${1}"; then
  exit 0
fi

if ! pid_alive "${1}"; then
  rm "${1}"
fi
