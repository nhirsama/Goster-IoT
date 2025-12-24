use core::convert::Infallible;
use embedded_hal::blocking::spi::{Transfer, Write};
use embedded_hal::digital::v2::OutputPin;
use embedded_storage_async::nor_flash::{
    ErrorType, NorFlash, NorFlashError, NorFlashErrorKind, ReadNorFlash,
};

pub const SECTOR_SIZE: u32 = 4096;
pub const PAGE_SIZE: u32 = 256;
pub const CAPACITY: u32 = 8 * 1024 * 1024;

// 必须为泛型增加 Debug 约束，否则无法 derive Debug
#[derive(Debug)]
pub enum DeviceError<SpiErr: core::fmt::Debug> {
    Spi(SpiErr),
    AddressOutOfBounds,
}

// 实现 NorFlashError 时，同样需要声明 SpiErr 满足 Debug
impl<SpiErr: core::fmt::Debug> NorFlashError for DeviceError<SpiErr> {
    fn kind(&self) -> NorFlashErrorKind {
        match self {
            Self::AddressOutOfBounds => NorFlashErrorKind::OutOfBounds,
            _ => NorFlashErrorKind::Other,
        }
    }
}

pub struct W25q64Device<SPI, CS> {
    spi_inst: SPI,
    cs_pin: CS,
}

impl<SPI, CS, SpiErr> W25q64Device<SPI, CS>
where
    SPI: Transfer<u8, Error = SpiErr> + Write<u8, Error = SpiErr>,
    CS: OutputPin<Error = Infallible>,
    SpiErr: core::fmt::Debug, // 核心修复点
{
    pub fn new(spi_inst: SPI, cs_pin: CS) -> Self {
        Self { spi_inst, cs_pin }
    }

    fn wait_busy(&mut self) -> Result<(), DeviceError<SpiErr>> {
        loop {
            self.cs_pin.set_low().ok();
            let _ = self.spi_inst.write(&[0x05]);
            let mut status_buf = [0u8; 1];
            let transfer_res = self.spi_inst.transfer(&mut status_buf);
            self.cs_pin.set_high().ok();

            transfer_res.map_err(DeviceError::Spi)?;
            if (status_buf[0] & 0x01) == 0 {
                break;
            }
        }
        Ok(())
    }

    fn write_enable(&mut self) -> Result<(), DeviceError<SpiErr>> {
        self.cs_pin.set_low().ok();
        let write_res = self.spi_inst.write(&[0x06]);
        self.cs_pin.set_high().ok();
        write_res.map_err(DeviceError::Spi)
    }
}

impl<SPI, CS, SpiErr> ErrorType for W25q64Device<SPI, CS>
where
    SPI: Transfer<u8, Error = SpiErr> + Write<u8, Error = SpiErr>,
    CS: OutputPin<Error = Infallible>,
    SpiErr: core::fmt::Debug,
{
    type Error = DeviceError<SpiErr>;
}

impl<SPI, CS, SpiErr> ReadNorFlash for W25q64Device<SPI, CS>
where
    SPI: Transfer<u8, Error = SpiErr> + Write<u8, Error = SpiErr>,
    CS: OutputPin<Error = Infallible>,
    SpiErr: core::fmt::Debug,
{
    const READ_SIZE: usize = 1;

    async fn read(&mut self, addr: u32, data_buf: &mut [u8]) -> Result<(), Self::Error> {
        if addr + data_buf.len() as u32 > CAPACITY {
            return Err(DeviceError::AddressOutOfBounds);
        }

        let cmd_read = [0x03, (addr >> 16) as u8, (addr >> 8) as u8, addr as u8];

        self.cs_pin.set_low().ok();
        // 使用 ? 直接返回错误，并自动将 Result<&mut [u8], E> 转为 Result<(), E> 的逻辑处理
        let res = (|| {
            self.spi_inst.write(&cmd_read)?;
            self.spi_inst.transfer(data_buf)?;
            Ok(())
        })();
        self.cs_pin.set_high().ok();

        res.map_err(DeviceError::Spi)
    }

    fn capacity(&self) -> usize {
        CAPACITY as usize
    }
}

impl<SPI, CS, SpiErr> NorFlash for W25q64Device<SPI, CS>
where
    SPI: Transfer<u8, Error = SpiErr> + Write<u8, Error = SpiErr>,
    CS: OutputPin<Error = Infallible>,
    SpiErr: core::fmt::Debug,
{
    const WRITE_SIZE: usize = 1;
    const ERASE_SIZE: usize = SECTOR_SIZE as usize;

    async fn erase(&mut self, from_addr: u32, to_addr: u32) -> Result<(), Self::Error> {
        let mut current_ptr = from_addr;
        while current_ptr < to_addr {
            self.write_enable()?;
            let cmd_erase = [
                0x20,
                (current_ptr >> 16) as u8,
                (current_ptr >> 8) as u8,
                current_ptr as u8,
            ];

            self.cs_pin.set_low().ok();
            let res = self.spi_inst.write(&cmd_erase);
            self.cs_pin.set_high().ok();

            res.map_err(DeviceError::Spi)?;
            self.wait_busy()?;
            current_ptr += SECTOR_SIZE;
        }
        Ok(())
    }

    async fn write(&mut self, addr: u32, payload: &[u8]) -> Result<(), Self::Error> {
        let mut current_ptr = addr;
        let mut data_offset = 0;
        let mut data_remain = payload.len();

        while data_remain > 0 {
            let page_space = PAGE_SIZE - (current_ptr % PAGE_SIZE);
            let chunk_len = core::cmp::min(data_remain, page_space as usize);

            self.write_enable()?;
            let cmd_page = [
                0x02,
                (current_ptr >> 16) as u8,
                (current_ptr >> 8) as u8,
                current_ptr as u8,
            ];

            self.cs_pin.set_low().ok();
            let res = (|| {
                self.spi_inst.write(&cmd_page)?;
                self.spi_inst
                    .write(&payload[data_offset..data_offset + chunk_len])?;
                Ok(())
            })();
            self.cs_pin.set_high().ok();

            res.map_err(DeviceError::Spi)?;
            self.wait_busy()?;

            current_ptr += chunk_len as u32;
            data_offset += chunk_len;
            data_remain -= chunk_len;
        }
        Ok(())
    }
}
