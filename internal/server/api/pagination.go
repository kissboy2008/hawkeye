package api

import "strconv"

// clampLimit 把 URL 上的 limit 夹到 [1, max]。
// 之前是裸 strconv.Atoi，?limit=1000000 会一次性把整张表读进内存
// （probe_results 单表几十万行时足以把容器打爆）。
func clampLimit(raw string, def, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
