package workbench_asset_cache_race_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"voice-clarity-acceptance/internal/store"
	"voice-clarity-acceptance/internal/web"
	"voice-clarity-acceptance/internal/workflow"
)

type requestGate struct {
	arrived chan struct{}
	release chan struct{}
}

type gatedResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	gate   *requestGate
	once   sync.Once
	status int
}

func (w *gatedResponseWriter) Header() http.Header {
	w.once.Do(func() {
		w.gate.arrived <- struct{}{}
		<-w.gate.release
	})
	return w.header
}

func (w *gatedResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *gatedResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func TestConcurrentWorkbenchAssetCacheIsRaceFree(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	handler := web.New(workflow.New(st))
	gate := &requestGate{arrived: make(chan struct{}, 2), release: make(chan struct{})}
	writers := []*gatedResponseWriter{
		{header: make(http.Header), gate: gate},
		{header: make(http.Header), gate: gate},
	}

	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			handler.ServeHTTP(writers[index], req)
		}(i)
	}

	<-gate.arrived
	<-gate.arrived
	close(gate.release)
	wg.Wait()

	for i, writer := range writers {
		if writer.status != http.StatusOK {
			t.Fatalf("第 %d 个并发请求状态码为 %d", i+1, writer.status)
		}
		if writer.body.Len() == 0 {
			t.Fatalf("第 %d 个并发请求未返回工作台页面", i+1)
		}
	}
}
