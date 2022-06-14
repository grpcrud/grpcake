package main

import "regexp"

var targetPortShorthandRegexp = regexp.MustCompile(`^:(\d+)$`)

func parseTarget(s string) string {
	t, _ := parseTargetWithShorthand(s)
	return t
}

func parseTargetWithShorthand(s string) (string, bool) {
	if s == ":" {
		return "localhost:50051", true
	}

	targetPort := targetPortShorthandRegexp.FindStringSubmatch(s)
	if targetPort != nil {
		return "localhost:" + targetPort[1], true
	}

	return s, false
}
