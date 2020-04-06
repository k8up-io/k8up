package args

import "strings"

type ArrayOpts []string

func (a *ArrayOpts) String() string {
	return strings.Join(*a, ", ")
}

func (a *ArrayOpts) BuildArgs() []string {
	argList := []string{}
	for _, elem := range *a {
		argList = append(argList, "--tag", elem)
	}
	return argList
}

func (a *ArrayOpts) Set(value string) error {
	*a = append(*a, value)
	return nil
}
