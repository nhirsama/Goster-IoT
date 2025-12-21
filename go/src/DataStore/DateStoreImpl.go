package DataStore

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"stm32f103keshe/src/inter"
	"sync"
)

// LocalStore 实现了 inter.DataStore 接口
type LocalStore struct {
	basePath string
	locks    sync.Map // 用于存储每个 uuid 的 *sync.RWMutex，实现分段锁
}

// NewLocalStore 初始化存储根目录
func NewLocalStore(basePath string) (*LocalStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}
	return &LocalStore{basePath: basePath}, nil
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
	return s.SaveConfig(uuid, meta) // 初始元数据作为基础配置保存
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
