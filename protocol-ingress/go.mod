module github.com/nhirsama/Goster-IoT/protocol-ingress

go 1.25.0

require (
	connectrpc.com/connect v1.20.0
	github.com/nhirsama/Goster-IoT/proto v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.11
)

replace github.com/nhirsama/Goster-IoT/proto => ../proto
