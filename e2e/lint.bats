#!/usr/bin/env bats
load "lib/utils"
load "lib/linter"

@test "Lint assertions in e2e tests" {

	for file in test*.bats; do
		echo "Linting '${file}'..."
		run lint "${file}"

		if [ $status -ne 0 ]; then
			echo "$output";
			return $status;
		fi
	done
}
