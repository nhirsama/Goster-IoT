module github.com/nhirsama/Goster-IoT/protocol-ingress

go 1.25.0

require (
	connectrpc.com/connect v1.20.0
	github.com/eclipse/paho.mqtt.golang v1.5.1
	github.com/nhirsama/Goster-IoT/proto v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/gorilla/websocket v1.5.3 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
)

replace github.com/nhirsama/Goster-IoT/proto => ../proto
