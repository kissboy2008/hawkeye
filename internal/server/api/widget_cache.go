package api

import (
	"sync"
	"time"
)

// 组件取数结果缓存（TTL + single-flight）。
//
// 背景：getWidgetData 每次都实时打目标（HTTP API + TCP/TLS 探活），前端 30s × N 个组件，
// 并且组件卡片失败重试会叠加。目标服务一抖动就雪崩成一片 502，还会把目标自身压垮。
// 这里给每个组件一份短 TTL 缓存，并发未命中的请求只让一个去抓，其余等它的结果。
type widgetCacheEntry struct {
	status int
	body   []byte
	exp    time.Time
}

type widgetFlight struct {
	done   chan struct{}
	status int
	body   []byte
}

var (
	widgetCacheMu   sync.Mutex
	widgetCacheData = make(map[string]*widgetCacheEntry)
	widgetInflight  = make(map[string]*widgetFlight)
)

// cachedWidgetPayload 返回 (status, body)。命中缓存直接返回；正在抓取的相同 key 会等待共享结果。
func cachedWidgetPayload(key string, ttl time.Duration, build func() (int, []byte)) (int, []byte) {
	now := time.Now()

	widgetCacheMu.Lock()
	if e, ok := widgetCacheData[key]; ok && now.Before(e.exp) {
		status, body := e.status, e.body
		widgetCacheMu.Unlock()
		return status, body
	}
	if f, ok := widgetInflight[key]; ok {
		widgetCacheMu.Unlock()
		<-f.done
		return f.status, f.body
	}
	f := &widgetFlight{done: make(chan struct{})}
	widgetInflight[key] = f
	widgetCacheMu.Unlock()

	// build 里是纯外部 IO，不持锁；panic 也要保证等待者被唤醒（否则调用方永久阻塞）。
	func() {
		defer func() {
			if r := recover(); r != nil {
				f.status = 502
				f.body = mustJSON(map[string]string{"error": "widget data panic"})
			}
		}()
		f.status, f.body = build()
	}()

	// 失败结果缓存时间短很多：目标恢复后能很快回到正常，又不会让重试风暴直接打到目标。
	cacheFor := ttl
	if f.status >= 500 {
		cacheFor = 2 * time.Second
	}

	widgetCacheMu.Lock()
	if len(widgetCacheData) > 512 {
		for k, e := range widgetCacheData {
			if !now.Before(e.exp) {
				delete(widgetCacheData, k)
			}
		}
	}
	widgetCacheData[key] = &widgetCacheEntry{status: f.status, body: f.body, exp: now.Add(cacheFor)}
	delete(widgetInflight, key)
	widgetCacheMu.Unlock()

	close(f.done)
	return f.status, f.body
}

// purgeWidgetCache 清空缓存（测试与配置变更时使用）。
func purgeWidgetCache() {
	widgetCacheMu.Lock()
	widgetCacheData = make(map[string]*widgetCacheEntry)
	widgetCacheMu.Unlock()
}
