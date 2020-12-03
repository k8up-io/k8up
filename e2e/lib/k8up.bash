#!/bin/bash

setup() {
  debug "-- $BATS_TEST_DESCRIPTION"
  debug "-- $(date)"
  debug ""
  debug ""
}

teardown() {
  cp -r /tmp/detik debug || true
}
