use embedded_storage_async::nor_flash::{NorFlash, ReadNorFlash};
use sequential_storage::cache::NoCache;
use sequential_storage::map::{MapConfig, MapStorage};

// 1. 结构体定义：只存引用和原始范围，不存 MapConfig 对象
pub struct GosterStorage<'a, FLASH> {
    flash: &'a mut FLASH,
    range: core::ops::Range<u32>,
}

impl<'a, FLASH, E> GosterStorage<'a, FLASH>
where
    FLASH: NorFlash<Error = E> + ReadNorFlash<Error = E>,
    E: core::fmt::Debug,
{
    pub fn new(flash: &'a mut FLASH, start_addr: u32, end_addr: u32) -> Self {
        Self {
            flash,
            range: start_addr..end_addr,
        }
    }

    pub async fn set<V>(&mut self, key: u16, value: &V) -> Result<(), &'static str>
    where
        V: for<'d> sequential_storage::map::Value<'d>,
    {
        let mut data_buffer = [0u8; 128];

        // --- 核心修正点 ---
        // 使用 &mut *self.flash 进行“再借用 (Re-borrow)”，而不是直接 Move
        // 这样 MapStorage 只会临时占用 Flash，函数结束就还回来
        let mut storage = MapStorage::new(
            &mut *self.flash,
            MapConfig::new(self.range.clone()),
            NoCache::new(),
        );

        storage
            .store_item(&mut data_buffer, &key, value)
            .await
            .map_err(|_| "Storage Write Error")?;

        Ok(())
    }

    pub async fn get<V>(&mut self, key: u16) -> Result<Option<V>, &'static str>
    where
        V: for<'d> sequential_storage::map::Value<'d>,
    {
        let mut data_buffer = [0u8; 128];

        // 同样，这里也使用临时创建的 storage 实例
        let mut storage = MapStorage::new(
            &mut *self.flash,
            MapConfig::new(self.range.clone()),
            NoCache::new(),
        );

        let item = storage
            .fetch_item::<V>(&mut data_buffer, &key)
            .await
            .map_err(|_| "Storage Read Error")?;

        Ok(item)
    }
}
