#pragma once

#include <Arduino.h>
#include <mbedtls/ecdh.h>
#include <mbedtls/gcm.h>
#include <mbedtls/entropy.h>
#include <mbedtls/ctr_drbg.h>

class CryptoLayer {
public:
    CryptoLayer();
    ~CryptoLayer();

    // 初始化随机数生成器
    bool begin();

    // 生成 X25519 密钥对
    bool generateKeyPair();
    
    // 获取我的公钥 (32 bytes)
    const uint8_t* getPublicKey();

    // 计算共享密钥 (Session Key)
    // peer_pubkey: 对端公钥 (32 bytes)
    bool computeSharedSecret(const uint8_t* peer_pubkey);

    // 获取 Session Key (AES Key)
    const uint8_t* getSessionKey();

    // AES-GCM 加密
    // input: 明文
    // len: 明文长度
    // aad: 附加认证数据 (Header 前28字节)
    // aad_len: AAD 长度
    // output: 密文 (缓冲区需 >= len)
    // tag: 输出认证标签 (16 bytes)
    // nonce: 输入 IV (12 bytes)
    bool encrypt(const uint8_t* input, size_t len, 
                 const uint8_t* aad, size_t aad_len,
                 uint8_t* output, uint8_t* tag, const uint8_t* nonce);

    // AES-GCM 解密
    bool decrypt(const uint8_t* input, size_t len,
                 const uint8_t* aad, size_t aad_len,
                 uint8_t* output, const uint8_t* tag, const uint8_t* nonce);

private:
    mbedtls_ecdh_context _ecdh;
    mbedtls_entropy_context _entropy;
    mbedtls_ctr_drbg_context _ctr_drbg;
    
    uint8_t _my_pubkey[32];  // 存放导出后的原始公钥
    uint8_t _session_key[32]; // 计算出的共享密钥
    bool _has_session_key = false;
};
