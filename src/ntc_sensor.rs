use micromath::F32Ext;
use stm32f1xx_hal::adc;
use embedded_hal::adc::{Channel, OneShot};

pub struct NtcSensor<AdcInst, PIN> {
    adc: adc::Adc<AdcInst>,
    pin: PIN,
}

impl<AdcInst, PIN> NtcSensor<AdcInst, PIN>
where
    // 约束 1: PIN 必须是 AdcInst 的一个合法通道
    PIN: Channel<AdcInst, ID = u8>,
    // 约束 2: ADC 必须支持对该 PIN 的单次采样读取
    adc::Adc<AdcInst>: OneShot<AdcInst, u16, PIN>,
{
    pub fn new(adc: adc::Adc<AdcInst>, pin: PIN) -> Self {
        Self { adc, pin }
    }

    pub fn read_temp(&mut self) -> f32 {
        // 使用 .ok().unwrap() 处理 Result
        let data: u16 = self.adc.read(&mut self.pin).ok().unwrap();

        // NTC 计算逻辑
        let r_ntc = 10000.0 / (4095.0 / data as f32 - 1.0);
        let b_constant: f32 = 3950.0;
        let r_nominal: f32 = 10000.0;
        let t_nominal: f32 = 298.15;

        let ln_r = (r_ntc / r_nominal).ln();
        let res_t_inv = 1.0 / t_nominal + ln_r / b_constant;
        let temp_k = 1.0 / res_t_inv;

        temp_k - 273.15
    }
}
