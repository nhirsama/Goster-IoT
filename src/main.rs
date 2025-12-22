#![no_std]
#![no_main]
mod device_meta;
mod logo;
mod ntc_sensor;
mod w25q64;

use cortex_m_rt::entry;
// #[cfg(not(test))]
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

    // 4. 实例化 w25q64
    // let mut flash = w25q64::W25Q64::new(spi, wq_cs).unwrap();
    // {
    //     let flash_id = flash.read_id().unwrap();
    //     rprintln!(
    //         "Flash ID   : {:02X?}{:02X?}{:02X?}",
    //         flash_id[0],
    //         flash_id[1],
    //         flash_id[2]
    //     );
    // }
    //
    // // --- 2. 准备测试数据 ---
    // let test_addr = 0x000000; // 写入到第 0 地址
    // let test_data = b"Goster-IoT Test"; // 15 字节的测试字符串
    // let mut read_buf = [0u8; 15]; // 用于读取的缓冲区
    //
    // rprintln!("开始测试读写流程...");
    //
    // flash.read(test_addr, &mut read_buf).unwrap();
    // match core::str::from_utf8(&read_buf) {
    //     Ok(s) => rprintln!("字符串内容: {}", s),
    //     Err(e) => rprintln!("无效的 UTF-8 编码: {:?}", e),
    // }
    // // --- 3. 执行：擦除 -> 写入 -> 读取 ---
    // let test_res: Result<(), &str> = (|| {
    //     // A. 擦除：写入前必须擦除所在扇区（4KB）
    //     flash.erase_sector(test_addr).map_err(|_| "擦除失败")?;
    //     rprintln!("扇区擦除完成");
    //
    //     // B. 写入：将数据写入 Page
    //     flash
    //         .write_page(test_addr, test_data)
    //         .map_err(|_| "写入失败")?;
    //     rprintln!(
    //         "数据写入完成: {:?}",
    //         core::str::from_utf8(test_data).unwrap()
    //     );
    //
    //     // C. 读取：验证数据是否一致
    //     flash
    //         .read(test_addr, &mut read_buf)
    //         .map_err(|_| "读取失败")?;
    //     rprintln!("数据读取完成");
    //
    //     Ok(())
    // })();
    //
    // // --- 4. 结果比对 ---
    // match test_res {
    //     Ok(_) => {
    //         if read_buf == *test_data {
    //             rprintln!("✅ 测试成功！写入与读取数据完全一致。");
    //         } else {
    //             rprintln!("❌ 测试失败：数据不匹配！");
    //             rprintln!("读取到的数据: {:X?}", read_buf);
    //         }
    //     }
    //     Err(e) => rprintln!("❌ 流程出错: {}", e),
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
        delay.delay_ms(500_u16);
    }
}
