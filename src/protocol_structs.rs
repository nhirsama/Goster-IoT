use heapless::Vec;
use serde::{Deserialize, Serialize};

// 假设最大缓存 20 个数据点 (根据内存调整)
pub const MAX_SAMPLES: usize = 20;

// 传感器类型枚举 (对应 API 文档 DataType)
#[derive(Serialize, Deserialize, Clone, Copy)]
#[repr(u8)]
pub enum SensorType {
    Temperature = 0x01,
    Humidity = 0x02,
    PM25 = 0x03,
}

// 批量上传的数据包结构 (匹配 API 文档 3.1)
// 结构: StartTimestamp(8) + Interval(4) + Type(1) + Count(4) + Data([f32])
#[derive(Serialize, Deserialize)]
pub struct MetricReport {
    pub start_timestamp: u64, // ms
    pub sample_interval: u32, // ms
    pub data_type: SensorType,
    pub count: u32,
    // 使用 heapless::Vec 在栈/静态内存中存储数据
    pub data_blob: Vec<f32, MAX_SAMPLES>,
}

// 实时遥测数据包 (STM32 -> ESP32)
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct SensorPacket {
    pub temperature: i8, // 摄氏度
    pub humidity: u8,    // 相对湿度 %
    pub lux: f32,        // 光照强度
}
