#pragma once

#include <Arduino.h>
#include <functional>
#include "GosterProtocol.h" // Reuse definitions like GosterHeader if possible, or redefine
#include "ProtocolUtils.h"

// Reuse GosterHeader from GosterProtocol.h if it's there?
// Let's assume we need to define structs if they are not exposed properly or just reuse them.
// GosterProtocol.h has GosterHeader.

typedef std::function<void(uint16_t cmdId, const uint8_t* payload, size_t len)> SerialCallback;

class SerialBridge {
public:
    SerialBridge();
    void begin(SerialCallback callback);
    
    // Call this from the PacketSerial callback with the decoded buffer (COBS decoded)
    // Buffer format: [Header (32)] [Payload (N)] [Footer (16)]
    void processFrame(const uint8_t* buffer, size_t size);

private:
    SerialCallback _callback;
};
