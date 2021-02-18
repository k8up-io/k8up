#!/usr/bin/env bats
load "lib/utils"
load "lib/linter"

@test "lint assertions" {

	for file in test*.bats; do
		run lint "${file}"
		# echo -e "$output" > /tmp/errors.txt
		[ "$status" -eq 0 ]
	done

}
