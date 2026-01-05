#pragma once

#include <Arduino.h>
#include <OneButton.h>
#include <PacketSerial.h>

constexpr uint8_t PIN_LED = 8;
constexpr uint8_t PIN_BUTTON = 9;
constexpr uint8_t PIN_UART_RX = 5;
constexpr uint8_t PIN_UART_TX = 4;

class Hardware {
public:
    void begin();
    void update(); // 在 loop 中调用
    
    void setLed(bool on);
    void blinkLed(int times, int delay_ms);

    // 获取 PacketSerial 实例以便在外面绑定回调
    PacketSerial& getPacketSerial() { return _packetSerial; }

    // 外部设置长按回调 (用于 Factory Reset)
    void setResetCallback(parameterizedCallbackFunction cb, void* parameter) {
        _btn.attachLongPressStart(cb, parameter);
    }

private:
    OneButton _btn;
    PacketSerial _packetSerial;
};
