package main

import "fmt"

func listMethods(msrc methodSource, args args) error {
	methods, err := msrc.Methods()
	if err != nil {
		return err
	}

	for _, m := range methods {
		if args.Long {
			var streamClient string
			if m.IsStreamingClient() {
				streamClient = "stream "
			}

			var streamServer string
			if m.IsStreamingServer() {
				streamServer = "stream "
			}

			fmt.Printf("rpc %s(%s%s) returns (%s%s)\n", m.FullName(), streamClient, m.Input().FullName(), streamServer, m.Output().FullName())
		} else {
			fmt.Println(m.FullName())
		}
	}

	return nil
}
