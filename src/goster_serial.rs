use crate::protocol_structs::GosterHeader;
use crc::{CRC_16_MODBUS, CRC_32_ISO_HDLC, Crc};

// 定义 CRC 算法实例
// CRC-16/MODBUS: 用于头部校验 (Poly 0x8005, Init 0xFFFF)
const CRC16_ALG: Crc<u16> = Crc::<u16>::new(&CRC_16_MODBUS);

// CRC-32 (ISO/HDLC): 用于整帧校验 (Poly 0x04C11DB7, 等同于以太网/Zip标准)
const CRC32_ALG: Crc<u32> = Crc::<u32>::new(&CRC_32_ISO_HDLC);

// 辅助函数：将结构体序列化为字节数组 (小端序)
fn serialize_header(header: &GosterHeader, buf: &mut [u8]) {
    buf[0..2].copy_from_slice(&header.magic.to_le_bytes());
    buf[2] = header.version;
    buf[3] = header.flags;
    buf[4..6].copy_from_slice(&header.status.to_le_bytes());
    buf[6..8].copy_from_slice(&header.cmd_id.to_le_bytes());
    buf[8..12].copy_from_slice(&header.key_id.to_le_bytes());
    buf[12..16].copy_from_slice(&header.length.to_le_bytes());
    buf[16..28].copy_from_slice(&header.nonce);
    buf[28..30].copy_from_slice(&header.h_crc16.to_le_bytes());
    buf[30..32].copy_from_slice(&header.padding.to_le_bytes());
}

/// 编码 Goster 协议帧
pub fn encode_goster_frame(
    cmd_id: u16,
    payload: &[u8],
    seq: u64,
    buffer: &mut [u8],
    temp_buf: &mut [u8], // 外部传入的临时缓冲区 (大小需 >= Header+Payload+Footer)
) -> Result<usize, ()> {
    // 1. 检查缓冲区大小
    // Header 32 bytes.
    if temp_buf.len() < 32 + payload.len() + 5 {
        return Err(());
    }

    // 2. 拷贝 Payload 到 temp_buf[32..]
    temp_buf[32..32 + payload.len()].copy_from_slice(payload);
    let payload_len = payload.len() as u32;

    // 3. 准备 Header
    let mut header = GosterHeader::default();
    header.cmd_id = cmd_id;
    header.length = payload_len;
    let seq_bytes = seq.to_le_bytes();
    header.nonce[4..12].copy_from_slice(&seq_bytes);

    // 4. 序列化 Header 到 temp_buf[0..32]
    // 借用 temp_buf 的前32字节
    // 注意：我们需要先计算 CRC，这需要完整的 Header 字节
    // 这里我们直接在 temp_buf 上操作

    // 先临时序列化一次以计算 CRC
    let mut header_buf = [0u8; 32];
    serialize_header(&header, &mut header_buf);
    header.h_crc16 = CRC16_ALG.checksum(&header_buf[0..28]);

    // 最终序列化 Header
    serialize_header(&header, &mut temp_buf[0..32]);

    // 5. 计算 Body CRC32 (Header + Payload)
    let mut digest = CRC32_ALG.digest();
    digest.update(&temp_buf[0..32]);
    digest.update(&temp_buf[32..32 + payload.len()]);
    let crc32 = digest.finalize();

    // 6. Footer (写入 temp_buf 末尾)
    let total_len = 32 + payload.len() + 16;
    if temp_buf.len() < total_len {
        return Err(());
    }

    let crc_bytes = crc32.to_le_bytes();
    // Padding is 0, so just write CRC and ensure rest is 0 if needed?
    // Footer: CRC32(4) + Padding(12)
    // temp_buf is likely dirty, so we must write 0s.
    let footer_start = 32 + payload.len();
    temp_buf[footer_start..footer_start + 4].copy_from_slice(&crc_bytes);
    for i in 0..12 {
        temp_buf[footer_start + 4 + i] = 0;
    }

    // 7. COBS 编码 (从 temp_buf 到 buffer)
    if buffer.len() < total_len + 5 {
        return Err(());
    }
    cobs_encode(&temp_buf[0..total_len], buffer)
}

/// 简单的 COBS 编码器 (以 0x00 结尾)
fn cobs_encode(input: &[u8], output: &mut [u8]) -> Result<usize, ()> {
    let mut read_index = 0;
    let mut write_index = 1;
    let mut code_index = 0;
    let mut code = 1;

    if output.len() < input.len() + 2 {
        return Err(());
    }

    while read_index < input.len() {
        if input[read_index] == 0 {
            output[code_index] = code;
            code = 1;
            code_index = write_index;
            write_index += 1;
        } else {
            output[write_index] = input[read_index];
            write_index += 1;
            code += 1;
            if code == 0xFF {
                output[code_index] = code;
                code = 1;
                code_index = write_index;
                write_index += 1;
            }
        }
        read_index += 1;
    }

    output[code_index] = code;
    output[write_index] = 0x00; // 终止符
    Ok(write_index + 1)
}

/// Simple COBS decoder
pub fn cobs_decode(input: &[u8], output: &mut [u8]) -> Result<usize, ()> {
    if input.is_empty() {
        return Ok(0);
    }

    let mut read_index = 0;
    let mut write_index = 0;

    while read_index < input.len() {
        let code = input[read_index];
        read_index += 1;

        if code == 0 {
            return Err(());
        } // Zero byte found in data (should only be terminator if handled outside)

        // Copy code-1 bytes
        for _ in 0..(code - 1) {
            if read_index >= input.len() {
                return Err(());
            } // Premature end
            if write_index >= output.len() {
                return Err(());
            } // Output overflow
            output[write_index] = input[read_index];
            write_index += 1;
            read_index += 1;
        }

        // Append implicit 0x00 if not the last block
        if code < 0xFF && read_index < input.len() {
            if write_index >= output.len() {
                return Err(());
            }
            output[write_index] = 0x00;
            write_index += 1;
        }
    }

    Ok(write_index)
}
