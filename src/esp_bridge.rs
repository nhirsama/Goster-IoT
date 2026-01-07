use crate::goster_serial;
use crate::protocol_structs::{self, CMD_TIME_SYNC, MetricReport, FRAME_BUF_SIZE, PAYLOAD_SIZE};
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
    rx_buf: [u8; 128], // RX 主要是控制指令，较短
    rx_idx: usize,
    tx_seq: u64,
    // 发送缓冲区 (根据 MAX_SAMPLES 自动计算)
    tx_buf: [u8; FRAME_BUF_SIZE],
    // 暂存缓冲区
    scratch_buf: [u8; FRAME_BUF_SIZE],
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
            tx_buf: [0u8; FRAME_BUF_SIZE],
            scratch_buf: [0u8; FRAME_BUF_SIZE],
            is_ready: false,
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
                    return BridgeEvent::EspReady;
                }
                // 2. 数据包处理 (COBS 累积)
                if self.rx_idx < self.rx_buf.len() {
                    self.rx_buf[self.rx_idx] = byte;
                    self.rx_idx += 1;
                }

                if byte == 0x00 {
                    let result = self.process_frame();
                    self.rx_idx = 0;
                    // 优化：如果收到了有效的时间同步包，也认为 ESP32 已经唤醒并就绪
                    if let BridgeEvent::TimeSync(_) = result {
                        self.is_ready = true;
                        rprintln!("ESP32 就绪 (收到 TimeSync)");
                    }
                    return result;
                }
            }
            Err(nb::Error::WouldBlock) => {}
            Err(nb::Error::Other(_)) => {
                // 清除 ORE 错误

                unsafe {
                    let _ = core::ptr::read_volatile(0x40013800 as *const u32);

                    let _ = core::ptr::read_volatile(0x40013804 as *const u32);
                }
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
                if len >= 32 {
                    let cmd_id = u16::from_le_bytes([decode_buf[6], decode_buf[7]]);
                    if cmd_id == CMD_TIME_SYNC {
                        if len >= 32 + 8 {
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

    /// 请求唤醒 (非阻塞)
    /// 如果未就绪，发送 0x00 信号
    pub fn request_wakeup(&mut self) {
        if !self.is_ready {
            // 清空 RX 以防有积压数据
            loop {
                let res: nb::Result<u8, _> = self.serial.read();
                if res.is_err() {
                    break;
                }
            }
            // 发送唤醒信号
            let _ = nb::block!(self.serial.write(0x00u8));
            rprintln!("已发送唤醒请求...");
        }
    }

    /// 发送批量报告 (非阻塞尝试)
    /// 返回 Ok(()) 表示发送成功
    /// 返回 Err(()) 表示未就绪 (调用者应继续 poll)

    pub fn send_batch(&mut self, report: &MetricReport) -> Result<(), ()> {
        if !self.is_ready {
            return Err(());
        }

        // 手动序列化 Payload (C Packed Struct Compatible)
        // 使用动态计算的常量，确保缓冲区永远足够
        let mut payload_buf = [0u8; PAYLOAD_SIZE]; 
        let mut offset = 0;
        // start_timestamp (u64, 8 bytes)
        payload_buf[offset..offset + 8].copy_from_slice(&report.start_timestamp.to_le_bytes());
        offset += 8;
        // sample_interval (u32, 4 bytes)
        payload_buf[offset..offset + 4].copy_from_slice(&report.sample_interval.to_le_bytes());
        offset += 4;
        // data_type (u8, 1 byte)
        payload_buf[offset] = report.data_type;
        offset += 1;
        // count (u32, 4 bytes)
        payload_buf[offset..offset + 4].copy_from_slice(&report.count.to_le_bytes());
        offset += 4;
        // data_blob (f32[], count * 4 bytes)
        for sample in &report.data_blob {
            if offset + 4 <= payload_buf.len() {
                payload_buf[offset..offset + 4].copy_from_slice(&sample.to_le_bytes());
                offset += 4;
            }
        }
        let payload_slice = &payload_buf[0..offset];
        // 发送数据
        match goster_serial::encode_goster_frame(
            protocol_structs::CMD_METRICS_REPORT,
            payload_slice,
            self.tx_seq,
            &mut self.tx_buf,
            &mut self.scratch_buf,
        ) {
            Ok(len) => {
                for i in 0..len {
                    let _ = nb::block!(self.serial.write(self.tx_buf[i]));
                }
                rprintln!(
                    "已发送批量报告 (Seq: {}, Type: {}, Len: {})",
                    self.tx_seq,
                    report.data_type,
                    len
                );
                self.tx_seq = self.tx_seq.wrapping_add(1);
                Ok(())
            }
            Err(_) => {
                rprintln!("编码错误");
                Ok(()) // 即使编码错误也视为处理完毕，避免死循环重试
            }
        }
    }
    /// 重置就绪状态
    pub fn reset_ready_state(&mut self) {
        self.is_ready = false;
    }
}
