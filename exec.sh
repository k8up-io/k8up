#!/bin/sh

# runs a command and echo's it's pid

PID_FILE=${PID_FILE-${1}}
shift 1

./clean.sh "${PID_FILE}"
if [ -f "${PID_FILE}" ]; then
  echo "The process is already running as PID '$(cat "${PID_FILE}")'."
  exit 0
fi

echo ">>>>>>>>> $(date) <<<<<<<<" | \
  tee -a "${PID_FILE}.stdout" >> "${PID_FILE}.stderr"

env | grep MINIO

"${@}" 1>>"${PID_FILE}.stdout" 2>>"${PID_FILE}.stderr" &
PID=$!

echo $PID > "${PID_FILE}"

echo "Running '${*}' with PID $PID"
echo "Writing STDOUT to '${PID_FILE}.stdout'"
echo "Writing STDERR to '${PID_FILE}.stderr'"

