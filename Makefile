.PHONY: echoserver-protos
echoserver-protos:
	protoc -I ./internal/echo --go_out=internal/echo --go_opt=paths=source_relative --go-grpc_out=internal/echo --go-grpc_opt=paths=source_relative ./internal/echo/echo.proto

.PHONY: echoserver-certs
echoserver-certs:
	cd internal/echoserver && make verify
