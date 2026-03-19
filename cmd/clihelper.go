package cmd

import "strings"

func SplitAtComma(slice []string) []string {
	result := []string{}

	for _, item := range slice {
		result = append(result, strings.Split(item, ",")...)
	}

	return result
}
