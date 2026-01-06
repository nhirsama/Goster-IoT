#![no_std]
#![no_main]

mod device_meta;
mod dht11_sensor;
mod esp_bridge;
mod goster_serial;
mod mh_sensor;
mod protocol_structs;
mod sensor_manager;
mod storage;

use cortex_m_rt::entry;
use nb;
use panic_rtt_target as _;
use stm32f1xx_hal::{
    adc, pac,
    prelude::*,
    rtc::Rtc,
    serial::{Config, Serial},
};

use rtt_target::{rprintln, rtt_init_print};

use crate::esp_bridge::{BridgeEvent, EspBridge};
use crate::sensor_manager::SensorManager;

#[entry]
fn main() -> ! {
    rtt_init_print!();
    let cp = cortex_m::Peripherals::take().unwrap();
    let dp = pac::Peripherals::take().unwrap();

    let mut flash = dp.FLASH.constrain();
    let mut rcc = dp.RCC.constrain();

    let clocks = rcc
        .cfgr
        .use_hse(8.MHz())
        .sysclk(72.MHz())
        .freeze(&mut flash.acr);
    let mut gpioa = dp.GPIOA.split();
    let mut gpiob = dp.GPIOB.split();
    let mut afio = dp.AFIO.constrain();

    // --- 1. 硬件外设初始化 ---
    let mut delay = cp.SYST.delay(&clocks);
    let mut push_pull1 = gpiob.pb0.into_push_pull_output(&mut gpiob.crl);
    let mut push_pull2 = gpiob.pb1.into_push_pull_output(&mut gpiob.crl);

    // 传感器
    let adc1 = adc::Adc::adc1(dp.ADC1, clocks);
    let mh_sensor = mh_sensor::MhSensor::new(adc1, gpioa.pa0.into_analog(&mut gpioa.crl));
    let dht_sensor =
        dht11_sensor::Dht11Sensor::new(gpioa.pa1.into_open_drain_output(&mut gpioa.crl));
    let mut manager = SensorManager::new(dht_sensor, mh_sensor);

    // RTC (Real Time Clock)
    let mut pwr = dp.PWR;
    let mut backup = rcc.bkp.constrain(dp.BKP, &mut pwr);
    let mut rtc = Rtc::new(dp.RTC, &mut backup);

    // UART
    let serial = Serial::new(
        dp.USART1,
        (
            gpioa.pa9.into_alternate_push_pull(&mut gpioa.crh),
            gpioa.pa10.into_floating_input(&mut gpioa.crh),
        ),
        &mut afio.mapr,
        Config::default().baudrate(115200.bps()),
        &clocks,
    );
    let mut bridge = EspBridge::new(serial);

    // 采样定时器 (TIM2, 设定 1Hz 即 1秒触发一次)
    // 使用 10kHz 基频 (PSC=7199)，1秒=10000计数 (ARR=9999)，均在 16位范围内
    let mut sample_timer = dp.TIM2.counter::<10_000>(&clocks);
    sample_timer.start(1.secs()).unwrap();

    rprintln!("系统启动，进入任务循环...");

    loop {
        // 1. 串口轮询 (处理 TimeSync 和其他指令)
        match bridge.poll() {
            BridgeEvent::TimeSync(server_ts) => {
                rprintln!("同步时间: {}", server_ts);
                rtc.set_time(server_ts as u32);
            }
            BridgeEvent::EspReady => {
                // 日志已在 bridge 内部打印，这里无需操作，状态已更新
            }
            BridgeEvent::None => {}
        }

        // 2. 检查定时器 (采样任务)
        match sample_timer.wait() {
            Ok(_) => {
                                                // 执行采样
                                                let current_time = rtc.current_time();
                                                manager.do_sample(&mut delay, current_time);
                                                push_pull1.toggle();
                                
                                                                // 缓冲区达到 75% 阈值时触发发送
                                                                if manager.is_almost_full() {
                                                                    rprintln!("达到 75% 阈值，准备唤醒并发送...");
                                                                    
                                                                    // 获取当前所有缓存的数据                                                    let reports = manager.take_reports(1000); 
                                                    
                                                    for report in reports.iter() {
                                                        if report.count > 0 {
                                                            // send_batch 内部会处理 0x00 唤醒和等待 'R' 信号
                                                            bridge.send_batch(report, &mut delay);
                                                        }
                                                    }
                                                    
                                                    // 发送完毕，重置桥接器状态
                                                    bridge.reset_ready_state();
                                                    push_pull2.toggle();
                                                }
                                            }            Err(nb::Error::WouldBlock) => {
                // 等待中...
            }
            Err(_) => {}
        }
    }
}
