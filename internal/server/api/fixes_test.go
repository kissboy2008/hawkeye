package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
)

func openTestDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testRouter(db *storage.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/widgets", listWidgets(db))
	r.POST("/api/v1/widgets", createWidget(db))
	r.PUT("/api/v1/widgets/:id", updateWidget(db))
	return r
}

// 验证 #20：并发未命中的请求只抓取一次（single-flight），TTL 内命中缓存。
func TestWidgetPayloadSingleFlight(t *testing.T) {
	purgeWidgetCache()
	var calls int32

	const n = 50
	var wg sync.WaitGroup
	results := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, body := cachedWidgetPayload("k1", 5*time.Second, func() (int, []byte) {
				atomic.AddInt32(&calls, 1)
				time.Sleep(50 * time.Millisecond) // 模拟外部 HTTP 抓取
				return 200, []byte(`{"ok":true}`)
			})
			results[idx] = string(body)
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 upstream call, got %d", got)
	}
	for i, r := range results {
		if r != `{"ok":true}` {
			t.Fatalf("goroutine %d got %q", i, r)
		}
	}

	// TTL 内再来一次：仍不调用 build
	cachedWidgetPayload("k1", 5*time.Second, func() (int, []byte) {
		atomic.AddInt32(&calls, 1)
		return 200, []byte(`{"ok":false}`)
	})
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("cache miss within TTL, calls=%d", got)
	}
}

// 验证 #20：失败结果用短 TTL，故障恢复后能很快重新抓；成功结果用正常 TTL。
func TestWidgetPayloadErrorShortTTL(t *testing.T) {
	purgeWidgetCache()
	var calls int32
	build := func() (int, []byte) {
		atomic.AddInt32(&calls, 1)
		return 502, []byte(`{"error":"boom"}`)
	}

	cachedWidgetPayload("k2", 15*time.Second, build)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls=%d", got)
	}

	// 2s 短 TTL：等 2.2s 后应重新抓取（成功结果在 15s 内不会）
	time.Sleep(2200 * time.Millisecond)
	cachedWidgetPayload("k2", 15*time.Second, build)
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("error result should expire in ~2s, calls=%d", got)
	}
}

// 验证 #20：build 里 panic 不能把等待者永久卡死。
func TestWidgetPayloadPanicRecovered(t *testing.T) {
	purgeWidgetCache()
	done := make(chan int, 1)
	go func() {
		status, _ := cachedWidgetPayload("k3", time.Second, func() (int, []byte) {
			panic("boom")
		})
		done <- status
	}()
	select {
	case status := <-done:
		if status != 502 {
			t.Fatalf("expected 502, got %d", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cachedWidgetPayload hung on panic")
	}
}

// 验证 #19：limit 夹逼。
func TestClampLimit(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"", 100},
		{"0", 100},
		{"-5", 100},
		{"abc", 100},
		{"50", 50},
		{"1000000", 1000},
		{"1000", 1000},
	}
	for _, c := range cases {
		if got := clampLimit(c.raw, 100, 1000); got != c.want {
			t.Errorf("clampLimit(%q) = %d, want %d", c.raw, got, c.want)
		}
	}
}

// 验证 #14 + #11：
//   - description 传空串能真正清空（旧实现永远清不掉）
//   - 没传的字段保持原值（api_token 不会被打码值覆盖）
//   - 列表接口返回的 api_token / config 密码是打码的
//   - 前端把打码值原样回传时，库里的真实值不被破坏
func TestWidgetPatchSemanticsAndMasking(t *testing.T) {
	db := openTestDB(t)
	r := testRouter(db)

	body := `{"name":"PVE","type":"proxmox","url":"https://10.10.10.2:8006","api_token":"root@pam!hawkeye=SECRET","node":"pve","config":"{\"ssh_password\":\"s3cret\"}","description":"old desc","widget_group":"home"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/widgets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("create failed: %d %s", w.Code, w.Body.String())
	}
	var created map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &created)
	if created["api_token"] != credentialMask {
		t.Fatalf("create response leaks token: %v", created["api_token"])
	}
	id := int64(created["id"].(float64))

	// 列表：打码
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/widgets", nil))
	if strings.Contains(w.Body.String(), "SECRET") || strings.Contains(w.Body.String(), "s3cret") {
		t.Fatalf("list leaks credentials: %s", w.Body.String())
	}

	// 清空 description：传空串必须生效
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/widgets/"+itoa(id), strings.NewReader(`{"description":""}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("patch failed: %d %s", w.Code, w.Body.String())
	}
	got, err := db.GetWidget(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Description != "" {
		t.Fatalf("description not cleared: %q", got.Description)
	}
	if got.APIToken != "root@pam!hawkeye=SECRET" {
		t.Fatalf("api_token changed by unrelated patch: %q", got.APIToken)
	}
	if !strings.Contains(got.Config, "s3cret") {
		t.Fatalf("config secret lost: %q", got.Config)
	}

	// 前端把打码值和打码 config 原样回传 → 不应破坏真实凭据
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/widgets/"+itoa(id),
		strings.NewReader(`{"api_token":"`+credentialMask+`","config":"{\"ssh_password\":\"`+credentialMask+`\"}"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	got, _ = db.GetWidget(id)
	if got.APIToken != "root@pam!hawkeye=SECRET" {
		t.Fatalf("mask round-trip destroyed token: %q", got.APIToken)
	}
	if !strings.Contains(got.Config, "s3cret") {
		t.Fatalf("mask round-trip destroyed ssh password: %q", got.Config)
	}

	// 显式传新值 → 生效
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/widgets/"+itoa(id), strings.NewReader(`{"api_token":"newtoken"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	got, _ = db.GetWidget(id)
	if got.APIToken != "newtoken" {
		t.Fatalf("new token not applied: %q", got.APIToken)
	}
}

// 验证 #13：WS 的 Origin 校验不再恒 true。
func TestWebSocketOriginCheck(t *testing.T) {
	mk := func(origin, host string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://"+host+"/ws", nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}
	if !upgrader.CheckOrigin(mk("", "hawkeye.local")) {
		t.Error("empty origin (native client) should be allowed")
	}
	if !upgrader.CheckOrigin(mk("http://hawkeye.local", "hawkeye.local")) {
		t.Error("same-origin should be allowed")
	}
	if upgrader.CheckOrigin(mk("http://evil.example", "hawkeye.local")) {
		t.Error("cross-origin should be rejected")
	}
	if upgrader.CheckOrigin(mk("null", "hawkeye.local")) {
		t.Error("origin 'null' should be rejected")
	}

	// 反向代理场景：Host 被改写成内网地址，原始域名在 X-Forwarded-Host 里。
	proxy := mk("https://hawkeye.example.com", "10.10.10.3:18325")
	if upgrader.CheckOrigin(proxy) {
		t.Error("origin should be rejected when only Host is considered")
	}
	proxy.Header.Set("X-Forwarded-Host", "hawkeye.example.com")
	if !upgrader.CheckOrigin(proxy) {
		t.Error("origin matching X-Forwarded-Host should be allowed")
	}
	proxy.Header.Set("X-Forwarded-Host", "other.example.com")
	if upgrader.CheckOrigin(proxy) {
		t.Error("origin not in X-Forwarded-Host should be rejected")
	}
}

// 验证 #16：qBittorrent 登录表单对特殊字符做编码。
func TestQbitLoginBodyEncoding(t *testing.T) {
	got := qbitLoginBody("admin", "p&ss w+rd%1")
	want := "password=p%26ss+w%2Brd%251&username=admin"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	return string(b)
}
