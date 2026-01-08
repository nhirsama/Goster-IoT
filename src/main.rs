#![no_std]
#![no_main]

mod device_meta;
mod dht11_sensor;
mod esp_bridge;
mod goster_serial;
mod mh_sensor;
mod protocol_structs;
mod sensor_manager;

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
    let rcc = dp.RCC.constrain();

    // 使用 HSE (外部高速时钟 8MHz) 并倍频到 72MHz
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
    // 使用默认配置 (尝试 LSE 32.768kHz)
    let mut pwr = dp.PWR;
    let mut backup = rcc.bkp.constrain(dp.BKP, &mut pwr);
    let mut rtc = Rtc::new(dp.RTC, &mut backup);
    rtc.set_time(0);
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
    let mut sample_timer = dp.TIM2.counter::<10_000>(&clocks);
    sample_timer.start(1.secs()).unwrap();

    // 状态机变量
    use crate::protocol_structs::{MAX_SAMPLES, MetricReport};

    rprintln!("系统启动 (Clock: HSE 72MHz, LSE RTC)，进入任务循环...");

    let mut pending_reports: Option<[MetricReport; 3]> = None;
    let mut wakeup_retry_counter = 0u32;

    loop {
        // 1. 串口轮询 (处理 TimeSync 和其他指令)
        match bridge.poll() {
            BridgeEvent::TimeSync(server_ts) => {
                rprintln!("同步时间: {}", server_ts);
                rtc.set_time(server_ts as u32);
                // 收到时间同步，说明链路已通，无需额外操作，bridge.is_ready 已为 true
            }
            BridgeEvent::EspReady => {
                rprintln!("ESP32 就绪 (收到 'R')");
            }
            BridgeEvent::None => {}
        }

        // 2. 发送逻辑 (状态机)
        if let Some(reports) = &pending_reports {
            if bridge.is_ready() {
                rprintln!("桥接器就绪，开始发送 {} 条报告...", reports.len());
                let mut success = true;
                for (i, report) in reports.iter().enumerate() {
                    rprintln!(
                        "正在发送第 {}/{} 个报告 (Type: {})",
                        i + 1,
                        reports.len(),
                        report.data_type
                    );
                    if bridge.send_batch(report).is_err() {
                        rprintln!("发送失败!");
                        success = false;
                        break;
                    }
                    // 增加到 50ms，确保接收端 PacketSerial 彻底处理完上一包并重置状态
                    delay.delay_ms(50u32);
                }
                if success {
                    rprintln!("发送完成，重置状态。");
                    bridge.reset_ready_state();
                    pending_reports = None; // 清空待发送队列
                    push_pull2.toggle();
                }
            } else {
                // 未就绪，请求唤醒
                wakeup_retry_counter += 1;
                if wakeup_retry_counter > 200000 {
                    // 约每秒触发一次 (取决于主循环速度)
                    rprintln!("请求唤醒...");
                    bridge.request_wakeup();
                    wakeup_retry_counter = 0;
                }
            }
        }

        // 3. 检查定时器 (采样任务)
        match sample_timer.wait() {
            Ok(_) => {
                // 打印 Tick 以验证定时器准确性
                let current_time = rtc.current_time();

                // 执行采样
                manager.do_sample(&mut delay, current_time);

                // 缓冲区达到 75% 阈值时触发发送
                if manager.is_almost_full() && pending_reports.is_none() {
                    rprintln!("达到 75% 阈值，加入发送队列...");
                    pending_reports = Some(manager.take_reports(1000));
                    // 立即触发一次唤醒请求
                    bridge.request_wakeup();
                }
            }
            Err(nb::Error::WouldBlock) => {
                // 等待中...
            }
            Err(_) => {}
        }
    }
}
