package platform_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hironow/phonewave/internal/platform"
)

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Info("hello %s", "world")

	got := buf.String()
	if !strings.Contains(got, "INFO hello world") {
		t.Errorf("expected INFO prefix, got %q", got)
	}
	if !regexp.MustCompile(`\[\d{2}:\d{2}:\d{2}\]`).MatchString(got) {
		t.Errorf("expected timestamp, got %q", got)
	}
}

func TestLogger_OK(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.OK("done")
	if !strings.Contains(buf.String(), " OK  done") {
		t.Errorf("expected OK prefix, got %q", buf.String())
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Warn("careful")
	if !strings.Contains(buf.String(), "WARN careful") {
		t.Errorf("expected WARN prefix, got %q", buf.String())
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Error("bad")
	if !strings.Contains(buf.String(), " ERR bad") {
		t.Errorf("expected ERR prefix, got %q", buf.String())
	}
}

func TestLogger_Verbose_Suppressed(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Debug("hidden")
	if buf.Len() != 0 {
		t.Errorf("expected no output in non-verbose mode, got %q", buf.String())
	}
}

func TestLogger_Verbose_Shown(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, true)
	logger.Debug("shown")
	if !strings.Contains(buf.String(), "DBUG shown") {
		t.Errorf("expected DBUG prefix, got %q", buf.String())
	}
}

func TestLogger_Writer(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	if logger.Writer() != &buf {
		t.Error("Writer() should return the configured writer")
	}
}

func TestLogger_SetExtraWriter_DualWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer f.Close()

	logger.SetExtraWriter(f)

	logger.Info("dual")

	// Check buffer output
	if !strings.Contains(buf.String(), "INFO dual") {
		t.Errorf("expected buffer output, got %q", buf.String())
	}

	// Check file output
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "INFO dual") {
		t.Errorf("expected file output, got %q", string(data))
	}
}

func TestLogger_SetExtraWriter_Nil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	logger.SetExtraWriter(f)
	f.Close()

	// Disconnect extra writer
	logger.SetExtraWriter(nil)

	// After disconnect, should write only to buf, not crash
	logger.Info("after disconnect")
	if !strings.Contains(buf.String(), "INFO after disconnect") {
		t.Errorf("expected output after disconnect, got %q", buf.String())
	}
}

func TestLogger_NoColorWhenNotTerminal(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Info("no terminal")
	if strings.Contains(buf.String(), "\033[") {
		t.Errorf("expected no ANSI codes for non-terminal writer, got %q", buf.String())
	}
}

func TestLogger_ColorWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.SetNoColor(false)
	logger.Info("colored")
	if !strings.Contains(buf.String(), "\033[") {
		t.Errorf("expected ANSI codes when color enabled, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "\033[0m") {
		t.Errorf("expected reset code, got %q", buf.String())
	}
}

func TestLogger_SetNoColor(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.SetNoColor(false)
	logger.Info("on")
	colored := buf.String()

	buf.Reset()
	logger.SetNoColor(true)
	logger.Info("off")
	plain := buf.String()

	if !strings.Contains(colored, "\033[") {
		t.Errorf("expected color when on, got %q", colored)
	}
	if strings.Contains(plain, "\033[") {
		t.Errorf("expected no color when off, got %q", plain)
	}
}

func TestLogger_NoColorEnvVar(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Info("env test")
	if strings.Contains(buf.String(), "\033[") {
		t.Errorf("NO_COLOR=1 should disable color, got %q", buf.String())
	}
}

func TestLogger_ExtraWriterPlainText(t *testing.T) {
	var primary bytes.Buffer
	logger := platform.NewLogger(&primary, false)
	logger.SetNoColor(false)

	var extra bytes.Buffer
	logger.SetExtraWriter(&extra)

	logger.Info("dual")

	if !strings.Contains(primary.String(), "\033[") {
		t.Errorf("primary should have ANSI codes, got %q", primary.String())
	}
	if strings.Contains(extra.String(), "\033[") {
		t.Errorf("extra writer should be plain text, got %q", extra.String())
	}
}

func TestRace_Logger_ConcurrentLog(t *testing.T) {
	logger := platform.NewLogger(nil, false)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.Info("concurrent log %d", id)
		}(i)
	}
	wg.Wait()
}

func TestLogger_ConcurrentSetExtraWriterAndWrite(t *testing.T) {
	logger := platform.NewLogger(io.Discard, false)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			var buf bytes.Buffer
			logger.SetExtraWriter(&buf)
		}()
		go func(n int) {
			defer wg.Done()
			logger.Info("race test info %d", n)
			logger.Warn("race test warn %d", n)
		}(i)
		go func() {
			defer wg.Done()
			logger.SetExtraWriter(nil)
		}()
	}
	wg.Wait()

	// Clean up
	logger.SetExtraWriter(nil)
}
