#![no_std]
#![no_main]
mod device_meta;
mod dht11_sensor;
mod logo;
mod mh_sensor;
mod ntc_sensor;
mod protocol_structs;
mod storage;

use core::fmt::Write;
use cortex_m_rt::entry;
use panic_rtt_target as _;
use stm32f1xx_hal::{
    adc,
    i2c::{BlockingI2c, Mode}, // 只需要 Mode，不需要 DutyCycle 了
    pac,
    prelude::*,
    serial::{Config, Serial},
};

use ssd1306::{
    I2CDisplayInterface, Ssd1306, prelude::*, rotation::DisplayRotation, size::DisplaySize128x64,
};

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
    let mut gpioa = dp.GPIOA.split();
    let mut gpiob = dp.GPIOB.split();
    let mut afio = dp.AFIO.constrain();

    let scl = gpiob.pb8.into_alternate_open_drain(&mut gpiob.crh);
    let sda = gpiob.pb9.into_alternate_open_drain(&mut gpiob.crh);

    let mut push_pull1 = gpiob.pb0.into_push_pull_output(&mut gpiob.crl);
    let mut push_pull2 = gpiob.pb1.into_push_pull_output(&mut gpiob.crl);

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

    let interface = I2CDisplayInterface::new(i2c);

    // 初始化 OLED
    let mut display = Ssd1306::new(interface, DisplaySize128x64, DisplayRotation::Rotate0)
        .into_buffered_graphics_mode();

    display.init().unwrap();

    display.flush().unwrap();

    let mut delay = cp.SYST.delay(&clocks);
    let adc_pin = gpioa.pa0.into_analog(&mut gpioa.crl);
    // 3. 初始化 ADC1（如果你之前已经初始化过了，直接用即可）
    let adc1 = adc::Adc::adc1(dp.ADC1, clocks);

    let mut mh_sensor = mh_sensor::MhSensor::new(adc1, adc_pin);

    display.draw(&logo::IMAGE_DATA).unwrap();
    display.flush().unwrap();
    let meta = DeviceMeta::collect(&dp.DBGMCU);
    rprintln!("--- Device Metadata ---");
    rprintln!("Flash Size : {} KB", meta.flash_size);
    rprintln!("Device ID  : 0x{:03X}", meta.dev_id); // 0x{:03X} 格式化为3位十六进制
    rprintln!("Rev ID     : 0x{:04X}", meta.rev_id); // 0x{:04X} 格式化为4位十六进制
    rprintln!("UID        : {}", meta.uid_hex().as_str());
    delay.delay_ms(5000_u16);

    // 初始化 DHT11 传感器 (PA1)
    let pa1 = gpioa.pa1.into_open_drain_output(&mut gpioa.crl);
    let mut dht_sensor = dht11_sensor::Dht11Sensor::new(pa1);

    // --- 初始化 UART (PA9=TX, PA10=RX) ---
    let tx = gpioa.pa9.into_alternate_push_pull(&mut gpioa.crh);
    let rx = gpioa.pa10;

    let mut serial = Serial::new(
        dp.USART1,
        (tx, rx),
        &mut afio.mapr,
        Config::default().baudrate(115200.bps()),
        &clocks,
    );

    let _ = serial.write_str("Goster-IoT Started.\r\n");
    use crate::protocol_structs::SensorPacket;
    loop {
        // 读取 DHT11 数据
        let (mut temp, mut humi) = (0, 0);
        match dht_sensor.read(&mut delay) {
            Ok(reading) => {
                temp = reading.temperature;
                humi = reading.relative_humidity;
                rprintln!("DHT11: Temp: {} C, Humidity: {} %", temp, humi);
            }
            Err(e) => {
                rprintln!("DHT11 Error: {:?}", e);
            }
        }
        let light_val = mh_sensor.read_lux();
        rprintln!("MH Sensor: {:.2} Lux", light_val);
        // --- 构建数据包并发送 (COBS 编码) ---
        let packet = SensorPacket {
            temperature: temp,
            humidity: humi,
            lux: light_val,
        };
        // 缓冲区: 32字节通常足够容纳这些简单数据 + COBS overhead
        let mut buf = [0u8; 32];
        // postcard::to_slice_cobs 会自动序列化并进行 COBS 编码 (以 0x00 结尾)
        match postcard::to_slice_cobs(&packet, &mut buf) {
            Ok(encoded) => {
                // encoded 是包含结尾 0x00 的 slice
                for byte in encoded.iter() {
                    let _ = serial.write(*byte);
                }
                rprintln!("Sent COBS Packet: {} bytes", encoded.len());
            }
            Err(e) => {
                rprintln!("Serialization Error: {:?}", e);
            }
        }
        push_pull1.toggle();
        push_pull2.toggle();
        delay.delay_ms(1000_u16);
    }
}
