package main

import "fmt"

var (
	connErrEarlyClose      = `rpc error: code = Unavailable desc = connection closed before server preface received`
	connErrBadTLSHandshake = `rpc error: code = Unavailable desc = connection error: desc = "transport: authentication handshake failed: tls: first record does not look like a TLS handshake"`
)

func humanizeConnErr(args args, err error) error {
	if err.Error() == connErrEarlyClose {
		if _, shorthand := parseTarget(args.Target); shorthand || args.Insecure {
			return fmt.Errorf("%w (is the server expecting TLS?)", err)
		} else {
			return fmt.Errorf("%w (is the server expecting mutual TLS?)", err)
		}
	}

	if err.Error() == connErrBadTLSHandshake {
		return fmt.Errorf("%w (is the server expecting plaintext?)", err)
	}

	return err
}
