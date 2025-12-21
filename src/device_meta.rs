use core::fmt::Write;
use heapless::String;
use stm32f1xx_hal::pac;
// 导入社区提供的安全签名读取函数
use stm32_device_signature::{device_id, flash_size_kb};

pub struct DeviceMeta {
    pub uid: [u8; 12],
    pub flash_size: u16,
    pub dev_id: u16,
    pub rev_id: u16,
}

impl DeviceMeta {
    /// 使用 PAC 和签名库进行安全读取
    pub fn collect(dbgmcu: &pac::dbgmcu::RegisterBlock) -> Self {
        // 1. 安全读取 UID (内部已处理内存映射)
        let uid = *device_id();

        // 2. 安全读取 Flash 容量 (单位 KB)
        let flash_size = flash_size_kb();

        // 3. 通过 PAC 访问 DBGMCU 寄存器 (去地址化)
        // 使用 PAC 提供的 read() 方法，返回类型安全的结构体
        let idcode = dbgmcu.idcode.read();
        let dev_id = idcode.dev_id().bits(); // 获取 0-11 位
        let rev_id = idcode.rev_id().bits(); // 获取 16-31 位

        Self {
            uid,
            flash_size,
            dev_id,
            rev_id,
        }
    }

    pub fn uid_hex(&self) -> String<24> {
        let mut s: String<24> = String::new();
        for byte in self.uid {
            write!(s, "{:02X}", byte).unwrap();
        }
        s
    }
}
