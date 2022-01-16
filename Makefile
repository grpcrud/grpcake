.PHONY: echoserver-protos
echoserver-protos:
	protoc -I ./internal/echo --go_out=internal/echo --go_opt=paths=source_relative --go-grpc_out=internal/echo --go-grpc_opt=paths=source_relative ./internal/echo/echo.proto

.PHONY: echoserver
echoserver:
	go run ./internal/echoserver/...

.PHONY: echoserver-tls
echoserver-tls:
	go run ./internal/echoserver/... -tls
