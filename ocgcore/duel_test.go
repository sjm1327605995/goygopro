package ocgcore

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// OnMessage 的日志分级：默认写 stderr；SetLogOutput 重定向；
// SetDebugLogging(false) 静默（引擎脚本消息量大有噪音，需可关）。
func TestLogEngineMessage(t *testing.T) {
	savedDebug := debugLog
	t.Cleanup(func() {
		logWriter = os.Stderr
		debugLog = savedDebug
	})

	var buf bytes.Buffer
	SetLogOutput(&buf)
	SetDebugLogging(true)
	logEngineMessage("hello engine")

	if !strings.Contains(buf.String(), "hello engine") {
		t.Fatalf("expected message in redirected log, got %q", buf.String())
	}

	buf.Reset()
	SetDebugLogging(false)
	logEngineMessage("noisy script warning")
	if buf.Len() != 0 {
		t.Fatalf("debug off must silence messages, got %q", buf.String())
	}

	SetLogOutput(nil)
	if logWriter != os.Stderr {
		t.Fatal("SetLogOutput(nil) must restore stderr")
	}
}
