use heapless::Vec;
use serde::{Deserialize, Serialize};

// 假设最大缓存 20 个数据点
pub const MAX_SAMPLES: usize = 64;

// 根据 MAX_SAMPLES 自动计算缓冲区大小
// MetricReportHeader (17 bytes) + Data (4 bytes/sample)
pub const PAYLOAD_SIZE: usize = 17 + MAX_SAMPLES * 4;

// GosterHeader(32) + Payload + Footer(16) + COBS Overhead
// Overhead 最坏情况是每 254 字节增加 1 字节，加上首尾 0x00，预留 32 字节非常充足
pub const FRAME_BUF_SIZE: usize = 32 + PAYLOAD_SIZE + 16 + 32;

// 传感器类型常量
pub const SENSOR_TYPE_TEMP: u8 = 0x01;
pub const SENSOR_TYPE_HUMI: u8 = 0x02;
pub const SENSOR_TYPE_PM25: u8 = 0x03;
pub const SENSOR_TYPE_LUX: u8 = 0x04;

// 批量上传的数据包结构 (匹配 API 文档 3.1)
#[derive(Serialize, Deserialize)]
pub struct MetricReport {
    pub start_timestamp: u64, // ms
    pub sample_interval: u32, // ms
    pub data_type: u8,        // 使用 u8 确保兼容性
    pub count: u32,
    pub data_blob: Vec<f32, MAX_SAMPLES>,
}

// 实时遥测数据包 (STM32 -> ESP32)
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SensorPacket {
    pub temperature: i8, // 摄氏度
    pub humidity: u8,    // 相对湿度 %
    pub lux: f32,        // 光照强度
}

pub const GOSTER_MAGIC: u16 = 0x5759;
pub const GOSTER_VERSION: u8 = 0x01;

pub const CMD_METRICS_REPORT: u16 = 0x0101;
pub const CMD_HEARTBEAT: u16 = 0x0104;
pub const CMD_TIME_SYNC: u16 = 0x0204;

#[repr(C, packed)]
#[derive(Debug, Clone, Copy)]
pub struct GosterHeader {
    pub magic: u16,      // 0-1
    pub version: u8,     // 2
    pub flags: u8,       // 3
    pub status: u16,     // 4-5
    pub cmd_id: u16,     // 6-7
    pub key_id: u32,     // 8-11
    pub length: u32,     // 12-15
    pub nonce: [u8; 12], // 16-27
    pub h_crc16: u16,    // 28-29
    pub padding: u16,    // 30-31
}

impl Default for GosterHeader {
    fn default() -> Self {
        Self {
            magic: GOSTER_MAGIC,
            version: GOSTER_VERSION,
            flags: 0,
            status: 0,
            cmd_id: 0,
            key_id: 0,
            length: 0,
            nonce: [0u8; 12],
            h_crc16: 0,
            padding: 0,
        }
    }
}
