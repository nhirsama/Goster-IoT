use crate::goster_serial;
use crate::protocol_structs::{self, CMD_TIME_SYNC, MetricReport};
use embedded_hal::blocking::delay::DelayMs;
use embedded_hal::serial::{Read, Write};
use rtt_target::rprintln;
use stm32f1xx_hal::{
    gpio::{
        Alternate, Floating, Input, PushPull,
        gpioa::{PA9, PA10},
    },
    pac::USART1,
    serial::Serial,
};

pub struct EspBridge {
    serial: Serial<USART1, (PA9<Alternate<PushPull>>, PA10<Input<Floating>>)>,
    rx_buf: [u8; 128],
    rx_idx: usize,
    tx_seq: u64,
    // 发送缓冲区
    tx_buf: [u8; 256],
    // 暂存缓冲区
    scratch_buf: [u8; 256],
    // ESP32 是否已就绪
    is_ready: bool,
}

pub enum BridgeEvent {
    None,
    TimeSync(u64),
    EspReady,
}

impl EspBridge {
    pub fn new(serial: Serial<USART1, (PA9<Alternate<PushPull>>, PA10<Input<Floating>>)>) -> Self {
        Self {
            serial,
            rx_buf: [0u8; 128],
            rx_idx: 0,
            tx_seq: 0,
            tx_buf: [0u8; 256],
            scratch_buf: [0u8; 256],
            is_ready: false,
        }
    }

    /// 发送唤醒信号 (非阻塞)
    pub fn wake_up(&mut self) {
        if !self.is_ready {
            rprintln!("预唤醒 ESP32...");
            // 清空 RX
            loop {
                let res: nb::Result<u8, _> = self.serial.read();
                if res.is_err() { break; }
            }
            let _ = self.serial.write(0x00u8);
        }
    }

    /// 检查是否就绪
    pub fn is_ready(&self) -> bool {
        self.is_ready
    }

    pub fn poll(&mut self) -> BridgeEvent {
        let result: nb::Result<u8, _> = self.serial.read();

        match result {
            Ok(byte) => {
                // 1. 检查握手信号 'R' (0x52)
                if byte == 0x52u8 {
                    self.is_ready = true;
                    rprintln!("ESP32 就绪 (异步通知)");
                    return BridgeEvent::EspReady;
                }

                // 2. 数据包处理
                if self.rx_idx < self.rx_buf.len() {
                    self.rx_buf[self.rx_idx] = byte;
                    self.rx_idx += 1;
                }

                if byte == 0x00 {
                    let result = self.process_frame();
                    self.rx_idx = 0;
                    return result;
                }
            }
            Err(nb::Error::WouldBlock) => {}
            Err(nb::Error::Other(_)) => {
                // 仅清除错误
                unsafe { let _ = core::ptr::read_volatile(0x40013804 as *const u32); }
            }
        }
        BridgeEvent::None
    }

    fn process_frame(&self) -> BridgeEvent {
        if self.rx_idx <= 1 {
            return BridgeEvent::None;
        }

        let mut decode_buf = [0u8; 128];
        match goster_serial::cobs_decode(&self.rx_buf[0..self.rx_idx - 1], &mut decode_buf) {
            Ok(len) => {
                if len >= 48 {
                    let cmd_id = u16::from_le_bytes([decode_buf[6], decode_buf[7]]);
                    if cmd_id == CMD_TIME_SYNC {
                        if len >= 32 + 8 + 16 {
                            let ts_bytes = [
                                decode_buf[32],
                                decode_buf[33],
                                decode_buf[34],
                                decode_buf[35],
                                decode_buf[36],
                                decode_buf[37],
                                decode_buf[38],
                                decode_buf[39],
                            ];
                            let timestamp = u64::from_le_bytes(ts_bytes);
                            return BridgeEvent::TimeSync(timestamp);
                        }
                    }
                }
            }
            Err(_) => {
                rprintln!("COBS 解码失败");
            }
        }
        BridgeEvent::None
    }

    /// 发送批量报告
    pub fn send_batch<D>(&mut self, report: &MetricReport, delay: &mut D)
    where
        D: DelayMs<u32>,
    {
                if !self.is_ready {
                    rprintln!("ESP32 未就绪，执行阻塞唤醒...");
                    
                    // 1. 清空 RX FIFO，防止积压数据或噪声干扰
                    loop {
                        let res: nb::Result<u8, _> = self.serial.read();
                        if res.is_err() { break; }
                    }
                    
                    // 2. 发送唤醒信号
                    let _ = self.serial.write(0x00u8);
                    
                    let mut ready = false;            for i in 0..5000 {
            let result: nb::Result<u8, _> = self.serial.read();
            match result {
                Ok(byte) => {
                    if byte == 0x52u8 {
                        ready = true;
                        self.is_ready = true;
                        // rprintln!("ESP32 已唤醒"); // 可选保留
                        break;
                    }
                }
                Err(nb::Error::WouldBlock) => {}
                Err(nb::Error::Other(_)) => {
                    // 发生错误时尝试清除 ORE，但不打印刷屏
                    unsafe { let _ = core::ptr::read_volatile(0x40013804 as *const u32); }
                }
            }
            
            delay.delay_ms(1u32);
                if i > 0 && i % 500 == 0 {
                    rprintln!("重发唤醒信号...");
                    let _ = self.serial.write(0x00u8);
                }
            }

            if !ready {
                rprintln!("严重错误: ESP32 唤醒超时,放弃发送!");
                return;
            }
        } else {
            rprintln!("ESP32 已就绪,直接发送!");
        }

        // 发送数据
        // 使用结构体内部缓冲区，避免栈溢出
        match goster_serial::encode_goster_frame(
            protocol_structs::CMD_METRICS_REPORT, 
            report, 
            self.tx_seq, 
            &mut self.tx_buf,
            &mut self.scratch_buf
        ) {
            Ok(len) => {
                for i in 0..len {
                    let _ = self.serial.write(self.tx_buf[i]);
                    // 每发送一个字节延时一下，防止 ESP32 接收溢出
                    if i % 16 == 0 { delay.delay_ms(1u32); }
                }
                rprintln!(
                    "已发送批量报告 (Seq: {}, Type: {})",
                    self.tx_seq,
                    report.data_type
                );
                self.tx_seq = self.tx_seq.wrapping_add(1);
            }
            Err(_) => rprintln!("编码错误"),
        }
    }

    /// 重置就绪状态
    pub fn reset_ready_state(&mut self) {
        self.is_ready = false;
    }
}
