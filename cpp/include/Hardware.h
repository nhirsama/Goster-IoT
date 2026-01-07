#pragma once

#include <Arduino.h>
#include <OneButton.h>
#include <PacketSerial.h>
#include "GosterProtocol.h" // 引入 RX_BUFFER_SIZE 定义

constexpr uint8_t PIN_LED = 8;
constexpr uint8_t PIN_BUTTON = 9;
constexpr uint8_t PIN_UART_RX = 5;
constexpr uint8_t PIN_UART_TX = 6;

class Hardware {
public:
    void begin();
    void update(); // 在 loop 中调用
    
    void setLed(bool on);
    void blinkLed(int times, int delay_ms);

    // 定义自定义类型的 PacketSerial，指定缓冲区大小
    using MyPacketSerial = PacketSerial_<COBS, 0, RX_BUFFER_SIZE>;

    // 获取 PacketSerial 实例
    MyPacketSerial& getPacketSerial() { return _packetSerial; }

    // 外部设置长按回调 (用于 Factory Reset)
    void setResetCallback(parameterizedCallbackFunction cb, void* parameter) {
        _btn.attachLongPressStart(cb, parameter);
    }

private:
    OneButton _btn;
    
    // 使用自定义类型
    MyPacketSerial _packetSerial;
};
