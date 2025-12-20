#![no_std]
#![no_main]

mod ntc_sensor;

use cortex_m_rt::entry;

use stm32f1xx_hal::{
    adc,
    i2c::{BlockingI2c, Mode}, // 只需要 Mode，不需要 DutyCycle 了
    pac,
    prelude::*,
};

use ssd1306::{
    prelude::*, rotation::DisplayRotation, size::DisplaySize128x64, I2CDisplayInterface, Ssd1306,
};

use embedded_graphics::{
    geometry::Point,
    mono_font::{ascii::FONT_6X10, MonoTextStyleBuilder},
    pixelcolor::BinaryColor,
    prelude::*,
    text::{Baseline, Text},
};

use core::fmt::Write;
// 必须导入 Write Trait
use heapless::String;

#[entry]
fn main() -> ! {
    let dp = pac::Peripherals::take().unwrap();
    let mut flash = dp.FLASH.constrain();
    let rcc = dp.RCC.constrain();

    let clocks = rcc
        .cfgr
        .use_hse(8.MHz())
        .sysclk(72.MHz())
        .freeze(&mut flash.acr);

    let mut gpiob = dp.GPIOB.split();
    let mut afio = dp.AFIO.constrain();

    let scl = gpiob.pb8.into_alternate_open_drain(&mut gpiob.crh);
    let sda = gpiob.pb9.into_alternate_open_drain(&mut gpiob.crh);

    // --- 修改开始 ---
    let i2c = BlockingI2c::i2c1(
        dp.I2C1,
        (scl, sda),
        &mut afio.mapr,
        // 使用标准模式 (100kHz)，根本不需要 DutyCycle 参数
        Mode::Standard {
            frequency: 100.kHz(),
        },
        clocks,
        1000,
        10,
        1000,
        1000,
    );
    // --- 修改结束 ---

    let interface = I2CDisplayInterface::new(i2c);

    // 初始化 OLED
    let mut display = Ssd1306::new(interface, DisplaySize128x64, DisplayRotation::Rotate0)
        .into_buffered_graphics_mode();

    display.init().unwrap();

    let text_style = MonoTextStyleBuilder::new()
        .font(&FONT_6X10)
        .text_color(BinaryColor::On)
        .build();

    display.flush().unwrap();


    let pb0_analog = gpiob.pb0.into_analog(&mut gpiob.crl);

    // 3. 初始化 ADC1（如果你之前已经初始化过了，直接用即可）
    let adc1 = adc::Adc::adc1(dp.ADC1, clocks);

    // 4. 实例化你的泛型传感器
    // 此时 Rust 编译器会自动推导出：
    // ADC_INST = pac::ADC1
    // PIN = stm32f1xx_hal::gpio::gpiob::PB0<Analog>
    let mut sensor = ntc_sensor::NtcSensor::new(adc1, pb0_analog);
    loop {
        let mut s: String<16> = String::new();
        let f32 = sensor.read_temp();
        write!(s, "Temp: {:.2} C", f32).unwrap();
        display.clear(BinaryColor::Off).unwrap();
        Text::with_baseline(&s, Point::new(10, 40), text_style, Baseline::Top)
            .draw(&mut display)
            .unwrap();
        display.flush().unwrap();
    }
}
