use embedded_hal::adc::{Channel, OneShot};
use stm32f1xx_hal::adc;
use micromath::F32Ext;

pub struct MhSensor<AdcInst, PIN> {
    adc: adc::Adc<AdcInst>,
    pin: PIN,
}

impl<AdcInst, PIN> MhSensor<AdcInst, PIN>
where
    PIN: Channel<AdcInst, ID = u8>,
    adc::Adc<AdcInst>: OneShot<AdcInst, u16, PIN>,
{
    pub fn new(adc: adc::Adc<AdcInst>, pin: PIN) -> Self {
        Self { adc, pin }
    }

    pub fn read(&mut self) -> u16 {
        self.adc.read(&mut self.pin).ok().unwrap()
    }

    /// 计算光照强度 (Lux)
    /// 基于 GL5528 光敏电阻特性估算:
    /// 1. 假设模块为 10k 分压电阻
    /// 2. 使用 Log-Log 物理模型转换阻值为 Lux
    pub fn read_lux(&mut self) -> f32 {
        let raw = self.read() as f32;
        
        // 避免除以零或边界值
        if raw < 10.0 { return 0.0; } 
        if raw > 4085.0 { return 0.0; }

        // 电路：VCC(3.3V) -> 10k Resistor -> Output -> LDR -> GND
        // V_out = V_cc * R_ldr / (R_10k + R_ldr)
        // 变形得 R_ldr = R_10k * V_out / (V_cc - V_out)
        // 代入 ADC 值: R_ldr = 10000 * raw / (4095 - raw)
        let r_ldr = 10000.0 * raw / (4095.0 - raw);

        // GL5528 特性参数 (近似值)
        // Gamma 值 ~0.603
        // 10 Lux 时的阻值 R10 ~15kΩ (范围 10k-20k)
        // 公式: Lux = 10 * (R10 / R_ldr)^(1/Gamma)
        let r_10 = 15000.0;
        let gamma = 0.603;
        
        let lux = 10.0 * (r_10 / r_ldr).powf(1.0 / gamma);
        lux
    }
}
