package main

import (
	"fmt"
	"strings"
)

func parseHeaders(headers, rawKeys, rawValues []string) ([]string, error) {
	if len(rawKeys) != len(rawValues) {
		return nil, fmt.Errorf("unequal number of keys and values")
	}

	var pairs []string
	for _, s := range headers {
		i := strings.Index(s, ": ")
		if i == -1 {
			return nil, fmt.Errorf("header must contain ': ', got: %q", s)
		}

		pairs = append(pairs, s[:i], s[i+2:])
	}

	for i, k := range rawKeys {
		pairs = append(pairs, k, rawValues[i])
	}

	return pairs, nil
}
