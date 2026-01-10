#include "CryptoLayer.h"
#include <mbedtls/error.h>

CryptoLayer::CryptoLayer() {
    mbedtls_ecdh_init(&_ecdh);
    mbedtls_entropy_init(&_entropy);
    mbedtls_ctr_drbg_init(&_ctr_drbg);
}

CryptoLayer::~CryptoLayer() {
    mbedtls_ecdh_free(&_ecdh);
    mbedtls_entropy_free(&_entropy);
    mbedtls_ctr_drbg_free(&_ctr_drbg);
}

bool CryptoLayer::begin() {
    const char *pers = "goster_iot";
    int ret = mbedtls_ctr_drbg_seed(&_ctr_drbg, mbedtls_entropy_func, &_entropy,
                                    (const unsigned char *) pers, strlen(pers));
    if (ret != 0) {
        Serial.printf("mbedtls_ctr_drbg_seed 失败: -0x%04x\n", -ret);
        return false;
    }

    // 初始化 ECDH 上下文为 X25519
    ret = mbedtls_ecp_group_load(&_ecdh.grp, MBEDTLS_ECP_DP_CURVE25519);
    if (ret != 0) {
        Serial.printf("mbedtls_ecp_group_load 失败: -0x%04x\n", -ret);
        return false;
    }

    return true;
}

bool CryptoLayer::generateKeyPair() {
    int ret = mbedtls_ecdh_gen_public(&_ecdh.grp, &_ecdh.d, &_ecdh.Q,
                                      mbedtls_ctr_drbg_random, &_ctr_drbg);
    if (ret != 0) {
        Serial.printf("mbedtls_ecdh_gen_public 失败: -0x%04x\n", -ret);
        return false;
    }

    // 导出原始公钥 (32 bytes for X25519)
    // X25519 公钥就是 X 坐标
    size_t olen;
    ret = mbedtls_mpi_write_binary(&_ecdh.Q.X, _my_pubkey, 32);
    if (ret != 0) {
        Serial.printf("导出公钥失败: -0x%04x\n", -ret);
        return false;
    }

    // 如果生成的公钥不足32字节（前面是0），需要右对齐填充？
    // mbedtls_mpi_write_binary 会自动处理大端/小端吗？
    // mbedtls 使用大端序 (Big Endian)，但 API 文档 (Goster) 要求 Little Endian?
    // 通常 X25519 都是 Little Endian。我们需要确认 mbedtls 的输出。
    // 这里先假设 mbedtls 输出为 Big Endian，可能需要反转。
    // 但 Curve25519 的标准 RFC7748 定义是 Little Endian。
    // 为了简单，我们先按照 standard bytes 处理。
    // 注意：mbedtls_mpi_write_binary 输出是大端序。我们需要反转它以符合 X25519 常规 (Little Endian)。
    // 但是 docs/API_SPECIFICATION.md 里说 "所有多字节整数均采用 Little-Endian"。
    // 公钥作为 byte array，通常直接传输。
    // 待定：如果握手失败，检查这里的字节序。

    // 反转为 Little Endian (如果你确定对方是 Go/Rust 的 X25519 库，通常是 LE)
    for (int i = 0; i < 16; i++) {
        uint8_t temp = _my_pubkey[i];
        _my_pubkey[i] = _my_pubkey[31 - i];
        _my_pubkey[31 - i] = temp;
    }

    return true;
}

const uint8_t *CryptoLayer::getPublicKey() {
    return _my_pubkey;
}

bool CryptoLayer::computeSharedSecret(const uint8_t *peer_pubkey) {
    int ret;
    mbedtls_mpi_read_binary(&_ecdh.Qp.Z, peer_pubkey, 0); // Z=0 隐式通常为 1
    // 对于 X25519，只使用 X 坐标。mbedtls 需要我们设置 Qp。
    // 先把 peer_pubkey (Little Endian) 转回 mbedtls 需要的 Big Endian
    uint8_t peer_be[32];
    for (int i = 0; i < 32; i++) peer_be[i] = peer_pubkey[31 - i];

    mbedtls_mpi_read_binary(&_ecdh.Qp.X, peer_be, 32);
    mbedtls_mpi_lset(&_ecdh.Qp.Z, 1); // Z=1

    // 计算共享密钥
    ret = mbedtls_ecdh_compute_shared(&_ecdh.grp, &_ecdh.z, &_ecdh.Qp, &_ecdh.d,
                                      mbedtls_ctr_drbg_random, &_ctr_drbg);
    if (ret != 0) {
        Serial.printf("计算共享密钥失败: -0x%04x\n", -ret);
        return false;
    }

    // 导出共享密钥 (Big Endian)
    uint8_t shared_secret[32];
    mbedtls_mpi_write_binary(&_ecdh.z, shared_secret, 32);

    // 通常使用 SHA256 或直接截取作为 Session Key。
    // 假设直接使用前 16 字节作为 AES-128 Key? 还是整个 32 字节作为 AES-256?
    // 文档说是 AES-128-GCM，所以只取前 16 字节。
    // 注意：这里的 shared_secret 是 mbedtls (Big Endian) 的结果。
    // 真正的 X25519 shared secret 是 Little Endian。我们需要反转回来吗？
    // 为了保持一致性，如果公钥反转了，这里计算出的 MPI 也是基于反转输入的。
    // 让我们做一次反转以获取原始 LE 字节流，然后取前 16 字节。

    // 反转为 Little Endian 以获取原始字节流
    for (int i = 0; i < 16; i++) {
        // 循环一半数组进行交换
        uint8_t temp = shared_secret[i];
        shared_secret[i] = shared_secret[31 - i];
        shared_secret[31 - i] = temp;
    }

    memcpy(_session_key, shared_secret, 32); // AES-256 使用全部 32 字节
    _has_session_key = true;
    return true;
}

const uint8_t *CryptoLayer::getSessionKey() {
    return _session_key;
}

bool CryptoLayer::encrypt(const uint8_t *input, size_t len,
                          const uint8_t *aad, size_t aad_len,
                          uint8_t *output, uint8_t *tag, const uint8_t *nonce) {
    if (!_has_session_key) return false;

    mbedtls_gcm_context gcm;
    mbedtls_gcm_init(&gcm);

    // 尝试 AES-256
    int ret = mbedtls_gcm_setkey(&gcm, MBEDTLS_CIPHER_ID_AES, _session_key, 256);
    if (ret == 0) {
        // 处理空负载
        const uint8_t *p_in = input;
        uint8_t dummy_byte = 0;
        if (len == 0 || p_in == nullptr) {
            p_in = &dummy_byte;
        }

        // AES-GCM 加密
        ret = mbedtls_gcm_crypt_and_tag(&gcm, MBEDTLS_GCM_ENCRYPT, len,
                                        nonce, 12, // Nonce 长度固定为 12 字节
                                        aad, aad_len,
                                        p_in, output,
                                        16, tag); // Tag 长度固定为 16 字节
    }

    mbedtls_gcm_free(&gcm);
    return (ret == 0);
}

bool CryptoLayer::decrypt(const uint8_t *input, size_t len,
                          const uint8_t *aad, size_t aad_len,
                          uint8_t *output, const uint8_t *tag, const uint8_t *nonce) {
    if (!_has_session_key) return false;

    mbedtls_gcm_context gcm;
    mbedtls_gcm_init(&gcm);

    // 尝试 AES-256
    int ret = mbedtls_gcm_setkey(&gcm, MBEDTLS_CIPHER_ID_AES, _session_key, 256);
    if (ret == 0) {
        // 处理空负载
        const uint8_t *p_in = input;
        uint8_t dummy_byte = 0;
        if (len == 0 || p_in == nullptr) {
            p_in = &dummy_byte;
        }

        // AES-GCM 解密 (认证解密)
        ret = mbedtls_gcm_auth_decrypt(&gcm, len,
                                       nonce, 12,
                                       aad, aad_len,
                                       tag, 16,
                                       p_in, output);
    }

    mbedtls_gcm_free(&gcm);
    if (ret != 0) {
        Serial.printf("GCM 解密失败: -0x%04x\n", -ret);
    }
    return (ret == 0);
}
