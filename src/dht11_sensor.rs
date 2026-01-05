use dht_sensor::{DhtReading, dht11};
use stm32f1xx_hal::gpio::{OpenDrain, Output, gpioa::PA1};

pub struct Dht11Sensor {
    pin: PA1<Output<OpenDrain>>,
}

impl Dht11Sensor {
    pub fn new(pin: PA1<Output<OpenDrain>>) -> Self {
        Self { pin }
    }

    pub fn read<D>(&mut self, delay: &mut D) -> Result<dht11::Reading, &'static str>
    where
        D: dht_sensor::Delay,
    {
        dht11::Reading::read(delay, &mut self.pin).map_err(|_| "Read Error")
    }
}
