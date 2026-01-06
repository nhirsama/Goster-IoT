use heapless::Vec;
use crate::protocol_structs::{MetricReport, MAX_SAMPLES, SENSOR_TYPE_TEMP, SENSOR_TYPE_HUMI, SENSOR_TYPE_LUX};
use crate::dht11_sensor::Dht11Sensor;
use crate::mh_sensor::MhSensor;
use stm32f1xx_hal::{
    gpio::{gpioa::PA0, Analog},
    pac::ADC1,
};
use rtt_target::rprintln;
use embedded_hal::blocking::delay::DelayMs;

/// 传感器管理器：负责数据的采集、缓存与打包
pub struct SensorManager {
    dht: Dht11Sensor,
    mh: MhSensor<ADC1, PA0<Analog>>,
    
    // 数据缓冲区
    temp_buf: Vec<f32, MAX_SAMPLES>,
    humi_buf: Vec<f32, MAX_SAMPLES>,
    lux_buf: Vec<f32, MAX_SAMPLES>,

    // 当前批次的起始时间戳 (毫秒)
    batch_start_ms: u64,
}

impl SensorManager {
    pub fn new(dht: Dht11Sensor, mh: MhSensor<ADC1, PA0<Analog>>) -> Self {
        Self {
            dht,
            mh,
            temp_buf: Vec::new(),
            humi_buf: Vec::new(),
            lux_buf: Vec::new(),
            batch_start_ms: 0,
        }
    }

    /// 执行一次采样
    /// @param current_sec: RTC 当前的秒数
    pub fn do_sample<D>(&mut self, delay: &mut D, current_sec: u32) 
    where D: DelayMs<u16> + dht_sensor::Delay
    {
        // 如果是批次的第一组数据，记录起始时间戳
        if self.temp_buf.is_empty() {
            self.batch_start_ms = (current_sec as u64) * 1000;
        }

        // 读取硬件传感器
        let (mut t, mut h) = (0, 0);
        if let Ok(reading) = self.dht.read(delay) {
            t = reading.temperature;
            h = reading.relative_humidity;
        }
        let lux = self.mh.read_lux();

        // 存入缓冲区
        let _ = self.temp_buf.push(t as f32);
        let _ = self.humi_buf.push(h as f32);
        let _ = self.lux_buf.push(lux);

        rprintln!("采样成功 [{}]: T={} H={} L={:.1} (进度: {}/{})", 
            current_sec, t, h, lux, self.temp_buf.len(), MAX_SAMPLES);
    }

    /// 检查缓冲区是否已满
    pub fn is_full(&self) -> bool {
        self.temp_buf.is_full()
    }

    /// 检查缓冲区是否接近满 (>= 75%)，用于预唤醒
    pub fn is_almost_full(&self) -> bool {
        self.temp_buf.len() >= (MAX_SAMPLES * 3 / 4)
    }

    /// 弹出当前的所有数据并清空缓存
    pub fn take_reports(&mut self, sample_interval_ms: u32) -> [MetricReport; 3] {
        let ts = self.batch_start_ms;
        let count = self.temp_buf.len() as u32;

        let reports = [
            MetricReport {
                start_timestamp: ts,
                sample_interval: sample_interval_ms,
                data_type: SENSOR_TYPE_TEMP,
                count,
                data_blob: self.temp_buf.clone(),
            },
            MetricReport {
                start_timestamp: ts,
                sample_interval: sample_interval_ms,
                data_type: SENSOR_TYPE_HUMI,
                count,
                data_blob: self.humi_buf.clone(),
            },
            MetricReport {
                start_timestamp: ts,
                sample_interval: sample_interval_ms,
                data_type: SENSOR_TYPE_LUX,
                count,
                data_blob: self.lux_buf.clone(),
            },
        ];

        // 清空以便开始下一批次
        self.temp_buf.clear();
        self.humi_buf.clear();
        self.lux_buf.clear();

        reports
    }
}