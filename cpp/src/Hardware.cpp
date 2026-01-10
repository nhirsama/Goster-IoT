#include "Hardware.h"

void Hardware::begin() {
    pinMode(PIN_LED, OUTPUT);
    setLed(false);

    // 初始化按钮 (GPIO 9, 低电平有效, 内部上拉)
    _btn = OneButton(PIN_BUTTON, true, true);
    _btn.setPressMs(5000); // 设置长按触发时间为 5s

    // 初始化与 STM32 的串口 (UART 1)
    Serial1.setRxBufferSize(2048);
    Serial1.begin(115200, SERIAL_8N1, PIN_UART_RX, PIN_UART_TX);
    _packetSerial.setStream(&Serial1);
}

void Hardware::setLed(bool on) {
    digitalWrite(PIN_LED, on ? LOW : HIGH);
}

void Hardware::blinkLed(int times, int delay_ms) {
    for (int i = 0; i < times; i++) {
        setLed(true);
        delay(delay_ms);
        setLed(false);
        delay(delay_ms);
    }
}

void Hardware::update() {
    _btn.tick(); // 处理按键状态机
    _packetSerial.update(); // 处理串口接收与解包
}
