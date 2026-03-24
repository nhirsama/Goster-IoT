package web

import (
	"net/http"

	"github.com/nhirsama/Goster-IoT/src/inter"
	apiv1 "github.com/nhirsama/Goster-IoT/src/web/v1"
)

func decodeAPIBody(r *http.Request, out interface{}, maxBodyBytes int64) error {
	return apiv1.DecodeBody(r, out, maxBodyBytes)
}

func parseDeviceStatusFilter(raw string) (string, *inter.AuthenticateStatusType, error) {
	return apiv1.ParseDeviceStatusFilter(raw)
}

func parseDownlinkCommand(raw string) (inter.CmdID, string, error) {
	return apiv1.ParseDownlinkCommand(raw)
}

func parsePositiveIntQuery(raw string, fallback int, max int) (int, error) {
	return apiv1.ParsePositiveIntQuery(raw, fallback, max)
}

func resolveMetricsRange(r *http.Request, minValidTimestampMs int64, defaultRangeLabel string) (start int64, end int64, rangeLabel string, err error) {
	return apiv1.ResolveMetricsRange(r, minValidTimestampMs, defaultRangeLabel)
}
