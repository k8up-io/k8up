#!/usr/bin/env bats
load "lib/utils"
load "lib/linter"

@test "lint assertions" {

	run lint "test1.bats"
	# echo -e "$output" > /tmp/errors.txt
	[ "$status" -eq 0 ]

}
