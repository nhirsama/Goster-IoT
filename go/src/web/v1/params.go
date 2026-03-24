package v1

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// DecodeBody 只解析一个 JSON 文档，并拒绝未知字段。
func DecodeBody(r *http.Request, out interface{}, maxBodyBytes int64) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}

	var tail struct{}
	if err := dec.Decode(&tail); err != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func requestWithStringMapJSON(r *http.Request, values map[string]string) *http.Request {
	payload, _ := json.Marshal(values)
	rr := r.Clone(r.Context())
	rr.Body = io.NopCloser(bytes.NewReader(payload))
	rr.ContentLength = int64(len(payload))
	return rr
}

// ParseDeviceStatusFilter 规范化设备状态过滤参数。
func ParseDeviceStatusFilter(raw string) (string, *inter.AuthenticateStatusType, error) {
	status := strings.TrimSpace(strings.ToLower(raw))
	if status == "" {
		status = "authenticated"
	}

	switch status {
	case "all":
		return status, nil, nil
	case "authenticated":
		v := inter.Authenticated
		return status, &v, nil
	case "pending":
		v := inter.AuthenticatePending
		return status, &v, nil
	case "refused":
		v := inter.AuthenticateRefuse
		return status, &v, nil
	case "revoked":
		v := inter.AuthenticateRevoked
		return status, &v, nil
	default:
		return "", nil, strconv.ErrSyntax
	}
}

// ParseDownlinkCommand 校验 v1 API 对外暴露的下行命令名称。
func ParseDownlinkCommand(raw string) (inter.CmdID, string, error) {
	command := strings.TrimSpace(strings.ToLower(raw))
	switch command {
	case "config_push":
		return inter.CmdConfigPush, command, nil
	case "ota_data":
		return inter.CmdOtaData, command, nil
	case "action_exec":
		return inter.CmdActionExec, command, nil
	case "screen_wy":
		return inter.CmdScreenWy, command, nil
	default:
		return 0, "", strconv.ErrSyntax
	}
}

// ParsePositiveIntQuery 解析分页等场景使用的正整数查询参数。
func ParsePositiveIntQuery(raw string, fallback int, max int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("must be an integer")
	}
	if v <= 0 {
		return 0, errors.New("must be greater than 0")
	}
	if max > 0 && v > max {
		return 0, fmt.Errorf("must be less than or equal to %d", max)
	}
	return v, nil
}

// ResolveMetricsRange 计算指标接口实际使用的时间窗口。
func ResolveMetricsRange(r *http.Request, minValidTimestampMs int64, defaultRangeLabel string) (start int64, end int64, rangeLabel string, err error) {
	end = time.Now().UnixMilli()
	rangeLabel = r.URL.Query().Get("range")
	if rangeLabel == "" {
		rangeLabel = defaultRangeLabel
	}
	if !IsValidMetricsRange(rangeLabel) {
		return 0, 0, "", errors.New("invalid range")
	}

	startRaw := r.URL.Query().Get("start_ms")
	endRaw := r.URL.Query().Get("end_ms")
	if startRaw != "" || endRaw != "" {
		if startRaw == "" || endRaw == "" {
			return 0, 0, "", errors.New("start_ms and end_ms must be provided together")
		}
		parsedStart, startErr := strconv.ParseInt(startRaw, 10, 64)
		parsedEnd, endErr := strconv.ParseInt(endRaw, 10, 64)
		if startErr != nil || endErr != nil {
			return 0, 0, "", errors.New("start_ms and end_ms must be integers")
		}
		if parsedStart > parsedEnd {
			return 0, 0, "", errors.New("start_ms must be less than or equal to end_ms")
		}
		start = parsedStart
		end = parsedEnd
		if start < minValidTimestampMs {
			start = minValidTimestampMs
		}
		return start, end, rangeLabel, nil
	}

	switch rangeLabel {
	case "all":
		start = minValidTimestampMs
	case "1h":
		start = time.Now().Add(-time.Hour).UnixMilli()
	case "6h":
		start = time.Now().Add(-6 * time.Hour).UnixMilli()
	case "24h":
		start = time.Now().Add(-24 * time.Hour).UnixMilli()
	case "7d":
		start = time.Now().Add(-7 * 24 * time.Hour).UnixMilli()
	}

	if start < minValidTimestampMs {
		start = minValidTimestampMs
	}
	return start, end, rangeLabel, nil
}

// IsValidMetricsRange 判断指标范围标签是否受接口支持。
func IsValidMetricsRange(raw string) bool {
	switch raw {
	case "1h", "6h", "24h", "7d", "all":
		return true
	default:
		return false
	}
}
