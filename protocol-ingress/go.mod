module github.com/nhirsama/Goster-IoT/protocol-ingress

go 1.25.0

require (
	connectrpc.com/connect v1.20.0
	github.com/eclipse/paho.mqtt.golang v1.5.1
	github.com/mochi-mqtt/server/v2 v2.6.5
	github.com/nhirsama/Goster-IoT/proto v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.10.0
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/xid v1.6.0 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/nhirsama/Goster-IoT/proto => ../proto
