#![no_std]
#![no_main]
mod device_meta;
mod logo;
mod ntc_sensor;
mod storage;

use cortex_m_rt::entry;
use panic_rtt_target as _;
use stm32f1xx_hal::{
    adc,
    i2c::{BlockingI2c, Mode}, // 只需要 Mode，不需要 DutyCycle 了
    pac,
    prelude::*,
    spi,
};

use ssd1306::{
    I2CDisplayInterface, Ssd1306, prelude::*, rotation::DisplayRotation, size::DisplaySize128x64,
};

use embedded_graphics::{
    geometry::Point,
    mono_font::{MonoTextStyleBuilder, ascii::FONT_6X10},
    pixelcolor::BinaryColor,
    prelude::*,
    text::{Baseline, Text},
};

use core::fmt::Write;
// 必须导入 Write Trait
use heapless::String;

use crate::device_meta::DeviceMeta;
use cortex_m as _;
use rtt_target;
use rtt_target::rprintln;

#[entry]
fn main() -> ! {
    rtt_target::rtt_init_print!();
    let cp = cortex_m::Peripherals::take().unwrap();
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

    let mut delay = cp.SYST.delay(&clocks);
    let pb0_analog = gpiob.pb0.into_analog(&mut gpiob.crl);

    // 3. 初始化 ADC1（如果你之前已经初始化过了，直接用即可）
    let adc1 = adc::Adc::adc1(dp.ADC1, clocks);

    // 4. 实例化你的泛型传感器
    // 此时 Rust 编译器会自动推导出：
    // ADC_INST = pac::ADC1
    // PIN = stm32f1xx_hal::gpio::gpiob::PB0<Analog>
    display.draw(&logo::IMAGE_DATA).unwrap();
    display.flush().unwrap();
    let meta = DeviceMeta::collect(&dp.DBGMCU);
    rprintln!("--- Device Metadata ---");
    rprintln!("Flash Size : {} KB", meta.flash_size);
    rprintln!("Device ID  : 0x{:03X}", meta.dev_id); // 0x{:03X} 格式化为3位十六进制
    rprintln!("Rev ID     : 0x{:04X}", meta.rev_id); // 0x{:04X} 格式化为4位十六进制
    rprintln!("UID        : {}", meta.uid_hex().as_str());
    delay.delay_ms(5000_u16);

    let mut sensor = ntc_sensor::NtcSensor::new(adc1, pb0_analog);
    let mut s: String<16> = String::new();

    let mut gpioa = dp.GPIOA.split();

    // 片选：PB1 (保持不变)
    let mut wq_cs = gpiob.pb1.into_push_pull_output(&mut gpiob.crl);
    wq_cs.set_high(); // 初始不选中

    // 时钟：必须是 PA5 才能对应 SPI1 的时钟信号
    let wq_slk = gpioa.pa5.into_alternate_push_pull(&mut gpioa.crl);

    // 主机输入 (MISO)：对应 PA6
    let wq_do = gpioa.pa6;

    // 主机输出 (MOSI)：必须是 PA7 才能对应 SPI1 的数据输出
    let wq_di = gpioa.pa7.into_alternate_push_pull(&mut gpioa.crl);
    let spi = spi::Spi::spi1(
        dp.SPI1,
        (wq_slk, wq_do, wq_di), // 传入你定义的变量名元组
        &mut afio.mapr,
        spi::Mode {
            polarity: spi::Polarity::IdleLow,
            phase: spi::Phase::CaptureOnFirstTransition,
        },
        8.MHz(),
        clocks,
    );

    // // 4. 实例化 w25q64
    // let mut flash_device = w25q46_drive::W25q64Device::new(spi, wq_cs);
    // let mut storage = GosterStorage::new(&mut flash_device, 0, 0x10000);
    //
    // // 假设在 async 上下文中，变量 flash_device 和 storage 已经按照你之前的定义初始化
    // // storage 范围 0 到 0x10000
    //
    // rprintln!("--- 启动 GosterStorage KV 读写测试 ---");
    //
    // // 1. 定义测试用的 Key 和 Value
    // let test_key: u16 = 0x02;
    // let test_value: u32 = 0x1313; // 使用 u32 作为测试数据
    //
    // // 2. 测试 set (写入/存储)
    // // 内部会调用 sequential-storage 的 store_item，处理擦除和负载均衡
    // rprintln!("Storage Setting: Key={}, Value={:#X}...", test_key, test_value);
    // match storage.set(test_key, test_value).await {
    //     Ok(_) => rprintln!("Storage set success."),
    //     Err(e) => {
    //         rprintln!("Storage set error: {}", e);
    //         return;
    //     }
    // }
    //
    // // 3. 测试 get (读取/查询)
    // // 需要显式指定获取的类型 V 为 u32
    // rprintln!("Storage Getting: Key={}...", test_key);
    // match storage.get::<u32>(test_key).await {
    //     Ok(Some(val)) => {
    //         rprintln!("Storage get success: {:#X}", val);
    //
    //         // 4. 校验数据
    //         if val == test_value {
    //             rprintln!("✅ 测试通过：读取到的 Value 与写入一致。");
    //         } else {
    //             rprintln!("❌ 测试失败：数据不匹配！");
    //             rprintln!("   预期: {:#X}", test_value);
    //             rprintln!("   实际: {:#X}", val);
    //         }
    //     }
    //     Ok(None) => {
    //         rprintln!("❌ 测试失败：未找到对应的 Key ({})", test_key);
    //     }
    //     Err(e) => {
    //         rprintln!("❌ Storage get error: {}", e);
    //     }
    // }
    //
    // // 5. 可选：测试更新同一个 Key
    // let new_value: u32 = 0x12345678;
    // rprintln!("Testing update: Setting Key={} to {:#X}...", test_key, new_value);
    // if let Ok(_) = storage.set(test_key, new_value).await {
    //     if let Ok(Some(val)) = storage.get::<u32>(test_key).await {
    //         if val == new_value {
    //             rprintln!("✅ 更新测试通过。");
    //         }
    //     }
    // }


    loop {
        //传感器核心代码，但是测试w25q64先注释掉
        // let f32 = sensor.read_temp();
        // let res: Result<(), &str> = (|| {
        //     s.clear();
        //
        //     write!(s, "Temp: {:.2} C", f32).map_err(|_| "String Error")?;
        //
        //     display.clear(BinaryColor::Off).map_err(|_| "Clear Error")?;
        //
        //     Text::with_baseline(&s, Point::new(10, 40), text_style, Baseline::Top)
        //         .draw(&mut display)
        //         .map_err(|_| "Draw Error")?;
        //
        //     display.flush().map_err(|_| "Flush Error")?;
        //
        //     Ok(())
        // })();
        //
        // if let Err(e) = res {
        //     rprintln!("Error: {}", e);
        // }
        delay.delay_ms(5000_u16);
    }
}
