package helper

import "strings"

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}

	return false
}

func GetStringPart(source, separator string, part int) string {
	parts := strings.Split(source, separator)

	return parts[part]
}
