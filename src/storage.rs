use embedded_storage_async::nor_flash::{MultiwriteNorFlash, NorFlash, ReadNorFlash};
use sequential_storage::cache::NoCache;
use sequential_storage::map::{MapConfig, MapStorage};
use sequential_storage::queue::{QueueConfig, QueueStorage};
use serde::Serialize;
pub struct Storage<'a, FLASH> {
    flash: &'a mut FLASH,
    map_range: core::ops::Range<u32>,
    queue_range: core::ops::Range<u32>,
}

impl<'a, FLASH, E> Storage<'a, FLASH>
where
    FLASH: NorFlash<Error = E> + ReadNorFlash<Error = E>,
    for<'b> &'b mut FLASH:
        MultiwriteNorFlash<Error = E> + NorFlash<Error = E> + ReadNorFlash<Error = E>,
    E: core::fmt::Debug,
{
    pub fn new(flash: &'a mut FLASH, start_addr: u32, end_addr: u32, map_total_size: u32) -> Self {
        Self {
            flash,
            map_range: start_addr..end_addr - map_total_size,
            queue_range: end_addr - map_total_size..end_addr,
        }
    }

    pub async fn set<V>(&mut self, key: u16, value: &V) -> Result<(), &'static str>
    where
        V: for<'d> sequential_storage::map::Value<'d>,
    {
        let mut data_buffer = [0u8; 128];
        // 临时创建一个 MapStorage 视图
        let mut storage = MapStorage::new(
            &mut *self.flash, // 临时再借用
            MapConfig::new(self.map_range.clone()),
            NoCache::new(),
        );
        storage
            .store_item(&mut data_buffer, &key, value)
            .await
            .map_err(|_| "Map Write Error")
    }

    pub async fn get<V>(&mut self, key: u16) -> Result<Option<V>, &'static str>
    where
        V: for<'d> sequential_storage::map::Value<'d>,
    {
        let mut data_buffer = [0u8; 128];
        // 同样，这里也使用临时创建的 storage 实例
        let mut storage = MapStorage::new(
            &mut *self.flash,
            MapConfig::new(self.map_range.clone()),
            NoCache::new(),
        );

        let item = storage
            .fetch_item::<V>(&mut data_buffer, &key)
            .await
            .map_err(|_| "Map Read Error")?;

        Ok(item)
    }

    pub async fn push<V>(&mut self, value: &V) -> Result<(), &'static str>
    where
        V: for<'d> sequential_storage::map::Value<'d> + Serialize,
    {
        let mut data_buffer = [0u8; 128];

        let mut storage = QueueStorage::new(
            &mut *self.flash,
            QueueConfig::new(self.queue_range.clone()),
            NoCache::new(),
        );

        let _serialized_bytes = postcard::to_slice(value, &mut data_buffer)
            .map_err(|_| "Postcard Serialization Error")?;

        storage
            .push(&mut data_buffer, true)
            .await
            .map_err(|_| "Queue Write Error")
    }

    pub async fn pop<V>(&mut self) -> Result<Option<V>, &'static str>
    where
        V: for<'d> sequential_storage::map::Value<'d>,
    {
        let mut data_buffer = [0u8; 128];
        let mut storage = QueueStorage::new(
            &mut *self.flash,
            QueueConfig::new(self.queue_range.clone()),
            NoCache::new(),
        );

        let item = storage
            .pop(&mut data_buffer)
            .await
            .map_err(|_| "Queue pop Error")?;

        match item {
            Some(bytes) => {
                let decoded_item = V::deserialize_from(bytes)
                    .map_err(|_| "Deserialize Error")?;

                Ok(Some(decoded_item.0)) // 直接返回解析出来的对象
            }
            None => Ok(None),
        }
    }
}
