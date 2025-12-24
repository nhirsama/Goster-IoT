// src/w25q64.rs
use embedded_storage_async::nor_flash::{NorFlash, ReadNorFlash};
use sequential_storage::cache::NoCache;
use sequential_storage::map::{MapConfig, MapStorage};

pub struct GosterStorage<'a, FLASH> {
    flash: &'a mut FLASH,
    range: core::ops::Range<u32>,
}

// 这里的约束必须和驱动实现的约束完全一致，new 才会出现
impl<'a, FLASH> GosterStorage<'a, FLASH>
where
    FLASH: NorFlash + ReadNorFlash, // 只要 FLASH 实现了这两个特征
{
    pub fn new(flash: &'a mut FLASH, start_addr: u32, end_addr: u32) -> Self {
        Self {
            flash,
            range: start_addr..end_addr,
        }
    }

    // 核心修正：接收 V 而不是 &V。
    // 如果存数字，V 是 u32 (Sized)；如果存字符串，V 是 &[u8] (Sized)
    pub async fn set<'v, V>(&mut self, key: u16, value: V) -> Result<(), &'static str>
    where
        V: sequential_storage::map::Value<'v>,
    {
        let mut data_buffer = [0u8; 128];
        let mut storage = MapStorage::new(
            &mut *self.flash,
            MapConfig::new(self.range.clone()),
            NoCache::new(),
        );

        // 这里传引用 &value 给库。由于 V 已经是 &[u8]，&value 就是 &&[u8]，库可以处理
        storage
            .store_item(&mut data_buffer, &key, &value)
            .await
            .map_err(|_| "Storage Write Error")?;

        Ok(())
    }

    pub async fn get<V>(&mut self, key: u16) -> Result<Option<V>, &'static str>
    where
        V: for<'d> sequential_storage::map::Value<'d>, // 获取时必须是拥有所有权的类型
    {
        let mut data_buffer = [0u8; 128];
        let mut storage = MapStorage::new(
            &mut *self.flash,
            MapConfig::new(self.range.clone()),
            NoCache::new(),
        );

        storage
            .fetch_item::<V>(&mut data_buffer, &key)
            .await
            .map_err(|_| "Storage Read Error")
    }
}
