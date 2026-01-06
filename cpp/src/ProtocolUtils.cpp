#include "ProtocolUtils.h"

uint16_t ProtocolUtils::calculateCRC16(const uint8_t* data, size_t len) {
    CRC16 crc;
    crc.setPolynome(0x8005);
    crc.setInitial(0xFFFF);   
    crc.setXorOut(0x0000);    
    crc.setReverseIn(true);
    crc.setReverseOut(true);
    crc.restart();
    crc.add(data, len);
    return crc.calc();
}

uint32_t ProtocolUtils::calculateCRC32(const uint8_t* data, size_t len) {
    CRC32 crc;
    crc.setPolynome(0x04C11DB7);
    crc.setInitial(0xFFFFFFFF);
    crc.setXorOut(0xFFFFFFFF);
    crc.setReverseIn(true);
    crc.setReverseOut(true);
    crc.restart();
    crc.add(data, len);
    return crc.calc();
}
