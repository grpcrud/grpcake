package main

import (
	"encoding/base64"
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

		k, v := s[:i], s[i+2:]
		decodedVal, err := decodeMetadataHeader(k, v)
		if err != nil {
			return nil, fmt.Errorf("decode %q: %w", v, err)
		}

		pairs = append(pairs, k, decodedVal)
	}

	for i, k := range rawKeys {
		v := rawValues[i]
		decodedVal, err := decodeMetadataHeader(k, v)
		if err != nil {
			return nil, fmt.Errorf("decode %q: %w", v, err)
		}

		pairs = append(pairs, k, decodedVal)
	}

	return pairs, nil
}

// the below is copied from internal/transport/http_util.go in grpc

const binHdrSuffix = "-bin"

func decodeBinHeader(v string) ([]byte, error) {
	if len(v)%4 == 0 {
		// Input was padded, or padding was not necessary.
		return base64.StdEncoding.DecodeString(v)
	}
	return base64.RawStdEncoding.DecodeString(v)
}

func decodeMetadataHeader(k, v string) (string, error) {
	if strings.HasSuffix(k, binHdrSuffix) {
		b, err := decodeBinHeader(v)
		return string(b), err
	}
	return v, nil
}
