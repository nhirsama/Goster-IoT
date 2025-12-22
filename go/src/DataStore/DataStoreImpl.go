package DataStore

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// LocalStore 实现了 inter.DataStore 接口
type LocalStore struct {
	basePath   string
	indexPath  string            // 全局索引文件路径：basePath/tokens.json
	locks      sync.Map          // 用于存储每个 uuid 的 *sync.RWMutex，实现分段锁
	tokenIndex sync.RWMutex      // 用于保护全局 Token 索引文件的读写锁
	tokenMu    sync.RWMutex      // 保护 tokenMap 的并发安全
	tokenMap   map[string]string // 内存中的 Token -> UUID 映射表

}

// NewLocalStore 初始化存储根目录
func NewLocalStore(basePath string) (*LocalStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	s := &LocalStore{
		basePath:  basePath,
		indexPath: filepath.Join(basePath, "tokens.json"),
		tokenMap:  make(map[string]string),
	}

	// 执行初始化检索与同步
	if err := s.reconcileTokens(); err != nil {
		return nil, fmt.Errorf("token reconciliation failed: %v", err)
	}

	return s, nil
}

// reconcileTokens 核心逻辑：确保索引文件与物理目录一致
func (s *LocalStore) reconcileTokens() error {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	// 1. 尝试加载现有的索引文件
	if data, err := os.ReadFile(s.indexPath); err == nil {
		json.Unmarshal(data, &s.tokenMap)
	}

	// 2. 扫描物理目录进行校验（以物理目录为准）
	entries, _ := os.ReadDir(s.basePath)
	changed := false

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		uuid := entry.Name()

		// 读取每个设备的 config.json 验证 Token
		var meta inter.DeviceMetadata
		confPath := filepath.Join(s.basePath, uuid, "config.json")
		if data, err := os.ReadFile(confPath); err == nil {
			if err := json.Unmarshal(data, &meta); err == nil && meta.Token != "" {
				// 如果索引缺失或不匹配，则补偿
				if s.tokenMap[meta.Token] != uuid {
					s.tokenMap[meta.Token] = uuid
					changed = true
				}
			}
		}
	}

	// 3. 如果发现不一致，回写索引文件保持同步
	if changed {
		s.saveTokenIndexLocked()
	}
	return nil
}

// getLock 获取或创建特定 UUID 的读写锁
func (s *LocalStore) getLock(uuid string) *sync.RWMutex {
	lock, _ := s.locks.LoadOrStore(uuid, &sync.RWMutex{})
	return lock.(*sync.RWMutex)
}

// InitDevice 初始化设备目录结构及基础特征文件
func (s *LocalStore) InitDevice(uuid string, meta inter.DeviceMetadata) error {
	dir := filepath.Join(s.basePath, uuid)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 保存私有配置
	if err := s.SaveConfig(uuid, meta); err != nil {
		return err
	}

	// 同步到全局 Token 索引
	if meta.Token != "" {
		return s.UpdateToken(uuid, meta.Token)
	}
	return nil
}

// DestroyDevice 物理删除该 uuid 对应的整个目录
func (s *LocalStore) DestroyDevice(uuid string) error {
	lock := s.getLock(uuid)
	lock.Lock()
	defer lock.Unlock()

	dir := filepath.Join(s.basePath, uuid)
	s.locks.Delete(uuid) // 清理锁映射
	return os.RemoveAll(dir)
}

// LoadConfig 从 config.json 加载配置
func (s *LocalStore) LoadConfig(uuid string, out interface{}) error {
	lock := s.getLock(uuid)
	lock.RLock()
	defer lock.RUnlock()

	path := filepath.Join(s.basePath, uuid, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// SaveConfig 原子化保存配置（写临时文件 + Rename）
func (s *LocalStore) SaveConfig(uuid string, data interface{}) error {
	lock := s.getLock(uuid)
	lock.Lock()
	defer lock.Unlock()

	dir := filepath.Join(s.basePath, uuid)
	tmpPath := filepath.Join(dir, "config.json.tmp")
	finalPath := filepath.Join(dir, "config.json")

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// 1. 写入临时文件
	if err := os.WriteFile(tmpPath, jsonData, 0644); err != nil {
		return err
	}
	// 2. 原子替换
	return os.Rename(tmpPath, finalPath)
}

// GetMetadata 获取元数据
func (s *LocalStore) GetMetadata(uuid string) (inter.DeviceMetadata, error) {
	var meta inter.DeviceMetadata
	err := s.LoadConfig(uuid, &meta)
	return meta, err
}

// ListDevices 扫描根目录获取设备列表
func (s *LocalStore) ListDevices(page, size int) ([]inter.DeviceRecord, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, err
	}

	var records []inter.DeviceRecord
	start := (page - 1) * size
	if start >= len(entries) {
		return records, nil
	}

	for i := start; i < len(entries) && i < start+size; i++ {
		if entries[i].IsDir() {
			uuid := entries[i].Name()
			meta, _ := s.GetMetadata(uuid)
			records = append(records, inter.DeviceRecord{
				UUID: uuid,
				Meta: meta,
			})
		}
	}
	return records, nil
}

// AppendMetric 向二进制文件追加高频采样点 (8B Time + 4B Value = 12B/Record)
func (s *LocalStore) AppendMetric(uuid string, ts int64, value float32) error {
	lock := s.getLock(uuid)
	lock.Lock()
	defer lock.Unlock()

	path := filepath.Join(s.basePath, uuid, "metrics.bin")
	// 注意：使用 O_APPEND 追加写入
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 12)
	// 1. 转换时间戳 (8字节)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(ts))
	// 2. 转换浮点数为比特流 (4字节) - 核心修复点
	binary.LittleEndian.PutUint32(buf[8:12], math.Float32bits(value))

	_, err = f.Write(buf)
	return err
}

func (s *LocalStore) QueryMetrics(uuid string, start, end int64) ([]inter.MetricPoint, error) {
	lock := s.getLock(uuid)
	lock.RLock()
	defer lock.RUnlock()

	path := filepath.Join(s.basePath, uuid, "metrics.bin")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []inter.MetricPoint{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var result []inter.MetricPoint
	buf := make([]byte, 12) // 必须与写入长度严格一致
	for {
		_, err := io.ReadFull(f, buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		t := int64(binary.LittleEndian.Uint64(buf[0:8]))
		// 只有在时间范围内才解析浮点数，节省 CPU
		if t >= start && t <= end {
			// 核心修复点：比特流转回浮点数
			v := math.Float32frombits(binary.LittleEndian.Uint32(buf[8:12]))
			result = append(result, inter.MetricPoint{Timestamp: t, Value: v})
		}
	}
	return result, nil
}

// WriteLog 写入简单的文本日志
func (s *LocalStore) WriteLog(uuid string, level string, message string) error {
	lock := s.getLock(uuid)
	lock.Lock()
	defer lock.Unlock()

	path := filepath.Join(s.basePath, uuid, "operation.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	logLine := fmt.Sprintf("[%s] %s\n", level, message)
	_, err = f.WriteString(logLine)
	return err
}

// 辅助函数：处理 float32 的二进制转换
func convertFloat32(f float32) uint32 {
	return binary.LittleEndian.Uint32(make([]byte, 4)) // 占位说明，实际应用应使用 math.Float32bits
}

func decodeFloat32(u uint32) float32 {
	return 0.0 // 占位说明，实际应用应使用 math.Float32frombits
}

// GetDeviceByToken 实现 inter.DataStore 接口
func (s *LocalStore) GetDeviceByToken(token string) (string, error) {
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()

	uuid, ok := s.tokenMap[token]
	if !ok {
		return "", fmt.Errorf("token not found")
	}
	return uuid, nil
}

// UpdateToken 更新指定设备的 Token 并同步全局索引
func (s *LocalStore) UpdateToken(uuid string, newToken string) error {
	// 1. 更新设备私有 config.json
	var meta inter.DeviceMetadata
	if err := s.LoadConfig(uuid, &meta); err != nil {
		return err
	}

	oldToken := meta.Token
	meta.Token = newToken
	if err := s.SaveConfig(uuid, meta); err != nil {
		return err
	}

	// 2. 同步更新内存和全局索引
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	if oldToken != "" {
		delete(s.tokenMap, oldToken)
	}
	s.tokenMap[newToken] = uuid

	return s.saveTokenIndexLocked()
}

// 辅助方法：将内存 Map 持久化到 tokens.json
func (s *LocalStore) saveTokenIndexLocked() error {
	jsonData, _ := json.MarshalIndent(s.tokenMap, "", "  ")
	tmpPath := s.indexPath + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.indexPath)
}
