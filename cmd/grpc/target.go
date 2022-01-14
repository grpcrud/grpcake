package main

import "regexp"

var targetPortShorthandRegexp = regexp.MustCompile(`^:(\d+)$`)

func parseTarget(s string) string {
	if s == ":" {
		return "localhost:50051"
	}

	targetPort := targetPortShorthandRegexp.FindStringSubmatch(s)
	if targetPort != nil {
		return "localhost:" + targetPort[1]
	}

	return s
}
