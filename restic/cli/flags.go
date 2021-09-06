package cli

// Flags stores arguments to pass to `restic` and can return them as array, see ApplyToCommand().
type Flags map[string][]string

// AddFlag appends the given values to the existing flag (identified by key) if it exists or
// appends it and it's values to the end of the list of globalFlags otherwise.
func (f Flags) AddFlag(key string, values ...string) {
	currentValues, found := f[key]
	if found {
		f[key] = append(currentValues, values...)
		return
	}

	f[key] = values
}

// Combine returns a new Flags instance that contains the flags and their values of both,
// the given Flags and the Flags instance
func Combine(first, second Flags) Flags {
	combined := Flags{}
	for firstK, firstV := range first {
		combined[firstK] = firstV
	}
	for secondK, secondV := range second {
		firstV, foundInFirst := first[secondK]
		if foundInFirst {
			combined[secondK] = append(firstV, secondV...)
		} else {
			combined[secondK] = secondV
		}
	}
	return combined
}

// ApplyToCommand applies the globalFlags to the given command and it's arguments, such that `newArgs = [command, globalFlags..., commandArgs...]`,
// in order the returning array to be passed to the `restic` process.
func (f Flags) ApplyToCommand(command string, commandArgs ...string) []string {
	args := make([]string, 0)
	if command != "" {
		args = append(args, command)
	}

	for flag, values := range f {
		args = append(args, expand(flag, values)...)
		continue
	}
	return append(args, commandArgs...)
}

func expand(flag string, values []string) []string {
	if len(values) == 0 {
		return []string{flag}
	}

	args := make([]string, 0)
	for _, value := range values {
		args = append(args, flag, value)
	}
	return args
}
