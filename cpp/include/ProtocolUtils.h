#pragma once

#include <Arduino.h>
#include <CRC16.h>
#include <CRC32.h>

class ProtocolUtils {
public:
    static uint16_t calculateCRC16(const uint8_t* data, size_t len);
    static uint32_t calculateCRC32(const uint8_t* data, size_t len);
};
