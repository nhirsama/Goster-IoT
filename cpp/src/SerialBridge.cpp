#include "SerialBridge.h"

SerialBridge::SerialBridge() : _callback(nullptr) {}

void SerialBridge::begin(SerialCallback callback) {
    _callback = callback;
}

void SerialBridge::processFrame(const uint8_t* buffer, size_t size) {
    // Min size = Header(32) + Footer(16) = 48 bytes (Empty payload)
    if (size < sizeof(GosterHeader) + 16) {
        Serial.printf("[SerialBridge] Error: Frame too short (%d bytes): ", size);
        for(size_t i=0; i<size; i++) Serial.printf("%02X ", buffer[i]);
        Serial.println();
        return;
    }

    const GosterHeader* header = (const GosterHeader*)buffer;

    // 1. Check Magic
    if (header->magic != GOSTER_MAGIC) {
        Serial.printf("[SerialBridge] Error: Invalid Magic %04X\n", header->magic);
        return;
    }

    // 2. Check Header CRC16 (Offset 0-27)
    uint16_t calc_h_crc = ProtocolUtils::calculateCRC16(buffer, 28);
    if (header->h_crc16 != calc_h_crc) {
        Serial.printf("[SerialBridge] Error: Header CRC Mismatch (Exp: %04X, Got: %04X)\n", calc_h_crc, header->h_crc16);
        return;
    }

    // 3. Check Payload Length consistency
    if (size != sizeof(GosterHeader) + header->length + 16) {
        Serial.printf("[SerialBridge] Error: Length Mismatch. Hdr Says: %d, Actual: %d\n", header->length, size - 48);
        return;
    }

    // 4. Check Body CRC32 (Footer)
    // Footer is last 16 bytes. First 4 bytes is CRC32.
    // CRC Covers Header + Payload
    const uint8_t* footer_ptr = buffer + sizeof(GosterHeader) + header->length;
    uint32_t received_crc32;
    memcpy(&received_crc32, footer_ptr, 4);

    uint32_t calc_crc32 = ProtocolUtils::calculateCRC32(buffer, sizeof(GosterHeader) + header->length);

    if (received_crc32 != calc_crc32) {
        Serial.printf("[SerialBridge] Error: Body CRC32 Mismatch (Exp: %08X, Got: %08X)\n", calc_crc32, received_crc32);
        return;
    }

    // 5. Success
    if (_callback) {
        const uint8_t* payload = buffer + sizeof(GosterHeader);
        _callback(header->cmd_id, payload, header->length);
    }
}