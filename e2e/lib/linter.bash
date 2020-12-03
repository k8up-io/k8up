#!/bin/bash

directory=$(dirname "${BASH_SOURCE[0]}")
source "$directory/utils.bash"


# Constants
lint_try_regex="^(run[[:space:]]+)?[[:space:]]*try[[:space:]]+(.*)$"
lint_verify_regex="^(run[[:space:]]+)?[[:space:]]*verify[[:space:]]+(.*)$"

# Global variables
errors_count=0
verified_entries_count=0


# Verifies the syntax of DETIK queries.
# @param {string} A file path
# @return
#	Any integer above 0: the number of found errors
#	0 Everything is fine
lint() {

	# Verify the file exists
	if [ ! -f "$1" ]; then
		handle_error "'$1' does not exist or is not a regular file."
		return 1
	fi
	
	# Make the regular expression case-insensitive
	shopt -s nocasematch;

	current_line=""
	current_line_number=0
	user_line_number=0
	multi_line=1
	was_multi_line=1
	while IFS='' read -r line || [[ -n "$line" ]]; do

		# Increase the line number
		current_line_number=$((current_line_number + 1))
		
		# Debug
		detik_debug "Read line $current_line_number: $line"
		
		# Skip empty lines and comments
		if [[ ! -n "$line" ]] || [[ "$line" =~ ^[[:space:]]*#.* ]]; then
			if [[ "$multi_line" == "0" ]]; then
				handle_error "Incomplete multi-line statement at $current_line_number."
				$current_line=""
			fi
			continue
		fi
		
		# Is this line a part of a multi-line statement?
		was_multi_line="$multi_line"
		[[ "$line" =~ ^.*\\$ ]]
		multi_line="$?"
		
		# Do we need to update the user line number?
		if [[ "$was_multi_line" != "0" ]]; then
			user_line_number="$current_line_number"
		fi

		# Is this the continuation of a previous line?
		if [[ "$multi_line" == "0" ]]; then
			current_line="$current_line ${line::-1}"
		elif [[ "$was_multi_line" == "0" ]]; then
			current_line="$current_line $line"
		else
			current_line="$line"
		fi
		
		# When we have a complete line...
		if [[ "$multi_line" != "0" ]]; then
			check_line "$current_line" "$user_line_number"
			current_line=""
		fi
		line=""
	done < "$1"

	# Output
	if [[ "$verified_entries_count" == "1" ]]; then
		echo "1 DETIK query was verified."
	else
		echo "$verified_entries_count DETIK queries were verified."
	fi
	
	if [[ "$errors_count" == "1" ]]; then
		echo "1 DETIK query was found to be invalid or malformed."
	else
		echo "$errors_count DETIK queries were found to be invalid or malformed."
	fi

	# Prepare the result
	res="$errors_count"
	
	# Reset global variables
	errors_count=0
	verified_entries_count=0

	return "$res"
}


# Verifies the correctness of a read line.
# @param {string} The line to verify
# @param {integer} The line number
# @return 0
check_line() {

	# Make the regular expression case-insensitive
	shopt -s nocasematch;

	# Get parameters and prepare the line
	line="$1"
	line_number="$2"
	context="Current line: $line"
	
	line=$(echo "$line" | sed -e 's/"[[:space:]]*"//g')
	line=$(trim "$line")
	context="$context\nPurged line:  $line"
	
	# Basic case: "run try", "run verify", "try", "verify" alone
	if [[ "$line" =~ ^(run[[:space:]]+)?try$ ]] || [[ "$line" =~ ^(run[[:space:]]+)?verify$ ]]; then
		verified_entries_count=$((verified_entries_count + 1))
		handle_error "Empty statement at line $line_number." "$context"
	
	# We have "try" or "run try" followed by something
	elif [[ "$line" =~ $lint_try_regex ]]; then
		verified_entries_count=$((verified_entries_count + 1))
		
		part=$(clean_regex_part "${BASH_REMATCH[2]}")
		context="$context\nRegex part:  $part"

		verify_against_pattern "$part" "$try_regex_verify"
		p_verify="$?"
		
		verify_against_pattern "$part" "$try_regex_find"
		p_find="$?"
		
		# detik_debug "p_verify=$p_verify, p_find=$p_find, part=$part"
		if [[ "$p_verify" != "0" ]] && [[ "$p_find" != "0" ]]; then
			handle_error "Invalid TRY statement at line $line_number." "$context"
		fi

	# We have "verify" or "run verify" followed by something
	elif [[ "$line" =~ $lint_verify_regex ]]; then
		verified_entries_count=$((verified_entries_count + 1))
		
		part=$(clean_regex_part "${BASH_REMATCH[2]}")
		context="$context\nRegex part:  $part"

		verify_against_pattern "$part" "$verify_regex_count_is"
		p_is="$?"
		
		verify_against_pattern "$part" "$verify_regex_count_are"
		p_are="$?"
		
		verify_against_pattern "$part" "$verify_regex_property_is"
		p_prop="$?"
		
		# detik_debug "p_is=$p_is, p_are=$p_are, p_prop=$p_prop, part=$part"
		if [[ "$p_is" != "0" ]] && [[ "$p_are" != "0" ]]  && [[ "$p_prop" != "0" ]] ; then
			handle_error "Invalid VERIFY statement at line $line_number." "$context"
		fi
	fi
}


# Cleans a string before being checked by a regexp.
# @param {string} The string to clean
# @return 0
clean_regex_part() {

	part=$(trim "$1")
	part=$(remove_surrounding_quotes "$part")
	part=$(trim "$part")
	echo "$part"
}


# Removes surrounding quotes.
# @param {string} The string to clean
# @return 0
remove_surrounding_quotes() {

	# Starting and ending with a quote? Remove them.
	if [[ "$1" =~ ^\"(.*)\"$ ]]; then
		echo "${BASH_REMATCH[1]}"
	
	# Otherwise, ignore it
	else
		echo "$1"
	fi
	
	return 0
}


# Verifies an assertion part against a regular expression.
# Given that assertions can skip double quotes around the whole
# assertion, we try the given regular expression and an altered one.
#
# Example: run try at most 5 times ... "'nginx'" ...
#
# Here, "'nginx'" is not part of the default regular expression.
# So, we update it to allow this kind of assertions.
#
# @param {string} The line to verify
# @param {string} The pattern
# @return
#	0 if everyhing went fine
#	not-zero in case of error
verify_against_pattern() {

	# Make the regular expression case-insensitive
	shopt -s nocasematch;

	line="$1"
	pattern="$2"
	code=0
	if ! [[ "$line" =~ $pattern ]]; then
		line=${line//\"\'/\'}
		line=${line//\'\"/\'}
		[[ "$line" =~ $pattern ]]
		code="$?"
	fi
	
	return "$code"
}


# Handles an error by printing it and updating the error count.
# @param {string} The error message
# @param2 {string} The error context
# @return 0
handle_error() {

	detik_debug "$2"
	detik_debug "Error: $1"

	echo "$1"
	errors_count=$((errors_count + 1))
}
