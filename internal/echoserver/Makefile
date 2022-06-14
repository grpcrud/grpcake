server-ca.key:
	openssl genrsa -out server-ca.key 4096

server-ca.crt: server-ca.key
	openssl req -x509 -new -nodes -sha256 -key server-ca.key -subj "/CN=grpcake-test-server-ca" -days 365 -out server-ca.crt

server.key:
	openssl genrsa -out server.key 4096

server.csr: server.key
	openssl req -new -sha256 -key server.key -subj "/CN=grpcake-test-server" -config openssl.conf -reqexts server -out server.csr

server.crt: server.csr server-ca.crt server-ca.key
	openssl x509 -req -sha256 -in server.csr -CA server-ca.crt -CAkey server-ca.key -set_serial 1 -out server.crt -days 365 -extfile openssl.conf -extensions server

client-ca.key:
	openssl genrsa -out client-ca.key 4096

client-ca.crt: client-ca.key
	openssl req -x509 -new -nodes -sha256 -key client-ca.key -subj "/CN=grpcake-test-client-ca" -days 365 -out client-ca.crt

client.key:
	openssl genrsa -out client.key 4096

client.csr: client.key
	openssl req -new -sha256 -key client.key -subj "/CN=grpcake-test-client" -out client.csr

client.crt: client.csr client-ca.crt client-ca.key
	openssl x509 -req -sha256 -in client.csr -CA client-ca.crt -CAkey client-ca.key -set_serial 1 -out client.crt -days 365

.PHONY: verify
verify: server.crt client.crt
	openssl verify -verbose -CAfile server-ca.crt server.crt
	openssl verify -verbose -CAfile client-ca.crt client.crt

.PHONY: clean
clean:
	rm *.key *.crt *.csr
