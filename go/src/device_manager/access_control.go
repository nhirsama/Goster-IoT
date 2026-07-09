package device_manager

import "github.com/nhirsama/Goster-IoT/src/inter"

const (
	// MetricTypeAccessSignalA 是门禁模块输入信号 A 的 legacy metric type。
	MetricTypeAccessSignalA uint8 = 8
	// MetricTypeAccessSignalB 是门禁模块输入信号 B 的 legacy metric type。
	MetricTypeAccessSignalB uint8 = 16
)

// AccessControlState 表示根据门禁模块两个输入信号计算出的门状态。
type AccessControlState struct {
	SignalA       *int
	SignalB       *int
	Open          *bool
	EvaluatedAtMs *int64
	StatusText    string
}

// EvaluateAccessControl 使用最近一次信号 A/B 指标计算门禁状态。
// 规则：signal_a == 1 && signal_b == 1 时开门；任一信号缺失时状态 unknown。
func EvaluateAccessControl(points []inter.MetricPoint) AccessControlState {
	var (
		signalA *int
		signalB *int
		tsA     int64
		tsB     int64
	)

	for _, point := range points {
		switch point.Type {
		case MetricTypeAccessSignalA:
			if signalA == nil || point.Timestamp >= tsA {
				normalized := normalizeAccessSignal(point.Value)
				signalA = &normalized
				tsA = point.Timestamp
			}
		case MetricTypeAccessSignalB:
			if signalB == nil || point.Timestamp >= tsB {
				normalized := normalizeAccessSignal(point.Value)
				signalB = &normalized
				tsB = point.Timestamp
			}
		}
	}

	evaluatedAt := latestAccessSignalTimestamp(tsA, tsB)
	state := AccessControlState{
		SignalA:       signalA,
		SignalB:       signalB,
		EvaluatedAtMs: evaluatedAt,
		StatusText:    "unknown",
	}
	if signalA == nil || signalB == nil {
		return state
	}

	open := *signalA == 1 && *signalB == 1
	state.Open = &open
	if open {
		state.StatusText = "open"
	} else {
		state.StatusText = "closed"
	}
	return state
}

func normalizeAccessSignal(value float32) int {
	if value >= 1 {
		return 1
	}
	return 0
}

func latestAccessSignalTimestamp(values ...int64) *int64 {
	var latest int64
	for _, value := range values {
		if value > latest {
			latest = value
		}
	}
	if latest <= 0 {
		return nil
	}
	return &latest
}
