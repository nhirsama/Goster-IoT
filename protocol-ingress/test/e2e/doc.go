// Package e2e provides end-to-end tests for the protocol-ingress MQTT adapter.
//
// The suite starts an embedded MQTT broker, injects an in-memory Mock Core API,
// runs the real protocol-ingress application, and verifies MQTT ingress,
// retained startup state, heartbeat, ACK, authentication rejection, high
// throughput, and downlink command flows.
package e2e
