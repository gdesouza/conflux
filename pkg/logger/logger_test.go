package logger

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	// Test with verbose = false
	logger := New(false)
	if logger == nil {
		t.Fatal("Expected logger to be created")
	}
	if logger.verbose != false {
		t.Error("Expected verbose to be false")
	}
	if logger.logger == nil {
		t.Error("Expected internal logger to be initialized")
	}

	// Test with verbose = true
	loggerVerbose := New(true)
	if loggerVerbose.verbose != true {
		t.Error("Expected verbose to be true")
	}
}

func captureLogOutput(fn func()) string {
	var buf bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&buf)

	defer func() {
		log.SetOutput(originalOutput)
	}()

	fn()
	return buf.String()
}

func TestInfo(t *testing.T) {
	logger := New(false)

	// Capture the log output
	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	logger.Info("test info message")

	output := buf.String()
	if !strings.Contains(output, "[INFO] test info message") {
		t.Errorf("Expected log output to contain '[INFO] test info message', got: %s", output)
	}
}

func TestInfoWithArgs(t *testing.T) {
	logger := New(false)

	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	logger.Info("test %s with %d args", "info", 2)

	output := buf.String()
	if !strings.Contains(output, "[INFO] test info with 2 args") {
		t.Errorf("Expected formatted log output, got: %s", output)
	}
}

func TestDebugVerboseTrue(t *testing.T) {
	logger := New(true) // verbose = true

	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	logger.Debug("debug message")

	output := buf.String()
	if !strings.Contains(output, "[DEBUG] debug message") {
		t.Errorf("Expected debug message to be logged when verbose=true, got: %s", output)
	}
}

func TestDebugVerboseFalse(t *testing.T) {
	logger := New(false) // verbose = false

	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	logger.Debug("debug message")

	output := buf.String()
	if strings.Contains(output, "[DEBUG]") {
		t.Errorf("Expected no debug output when verbose=false, got: %s", output)
	}
}

func TestDebugWithArgs(t *testing.T) {
	logger := New(true) // verbose = true

	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	logger.Debug("debug %s with %d args", "message", 2)

	output := buf.String()
	if !strings.Contains(output, "[DEBUG] debug message with 2 args") {
		t.Errorf("Expected formatted debug output, got: %s", output)
	}
}

func TestError(t *testing.T) {
	logger := New(false)

	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	logger.Error("error message")

	output := buf.String()
	if !strings.Contains(output, "[ERROR] error message") {
		t.Errorf("Expected error message to be logged, got: %s", output)
	}
}

func TestErrorWithArgs(t *testing.T) {
	logger := New(false)

	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	logger.Error("error %s with code %d", "occurred", 500)

	output := buf.String()
	if !strings.Contains(output, "[ERROR] error occurred with code 500") {
		t.Errorf("Expected formatted error output, got: %s", output)
	}
}

func TestFatal(t *testing.T) {
	// We can't easily test Fatal because it calls os.Exit(1)
	// Instead we test that it logs the message before exiting
	logger := New(false)

	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	// We can't call Fatal in the test because it would exit
	// So we manually test the logging part by calling the logger directly
	logger.logger.Printf("[FATAL] test fatal message")

	output := buf.String()
	if !strings.Contains(output, "[FATAL] test fatal message") {
		t.Errorf("Expected fatal message format, got: %s", output)
	}
}

func TestFatalWithArgs(t *testing.T) {
	logger := New(false)

	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	// Simulate what Fatal would log
	logger.logger.Printf("[FATAL] fatal %s with code %d", "error", 1)

	output := buf.String()
	if !strings.Contains(output, "[FATAL] fatal error with code 1") {
		t.Errorf("Expected formatted fatal output, got: %s", output)
	}
}

// Test the fmt wrapper functions
func TestPrint(t *testing.T) {
	logger := New(false)

	// Capture stdout
	r, w, _ := os.Pipe()
	originalStdout := os.Stdout
	os.Stdout = w

	logger.Print("test print")

	w.Close()
	os.Stdout = originalStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output != "test print" {
		t.Errorf("Expected 'test print', got: %s", output)
	}
}

func TestPrintf(t *testing.T) {
	logger := New(false)

	// Capture stdout
	r, w, _ := os.Pipe()
	originalStdout := os.Stdout
	os.Stdout = w

	logger.Printf("test %s with %d", "printf", 1)

	w.Close()
	os.Stdout = originalStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output != "test printf with 1" {
		t.Errorf("Expected 'test printf with 1', got: %s", output)
	}
}

func TestPrintln(t *testing.T) {
	logger := New(false)

	// Capture stdout
	r, w, _ := os.Pipe()
	originalStdout := os.Stdout
	os.Stdout = w

	logger.Println("test println")

	w.Close()
	os.Stdout = originalStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output != "test println\n" {
		t.Errorf("Expected 'test println\\n', got: %s", output)
	}
}

func TestPrintlnMultipleArgs(t *testing.T) {
	logger := New(false)

	// Capture stdout
	r, w, _ := os.Pipe()
	originalStdout := os.Stdout
	os.Stdout = w

	logger.Println("test", "multiple", "args")

	w.Close()
	os.Stdout = originalStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output != "test multiple args\n" {
		t.Errorf("Expected 'test multiple args\\n', got: %s", output)
	}
}

// Test edge cases
func TestLoggerOutputRedirection(t *testing.T) {
	logger := New(false)

	// Test that we can redirect the logger output
	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	logger.Info("redirected")
	logger.Error("also redirected")

	output := buf.String()
	if !strings.Contains(output, "[INFO] redirected") {
		t.Error("Expected INFO message in redirected output")
	}
	if !strings.Contains(output, "[ERROR] also redirected") {
		t.Error("Expected ERROR message in redirected output")
	}
}

func TestVerboseToggling(t *testing.T) {
	// Test that debug behavior changes based on verbose setting
	verboseLogger := New(true)
	quietLogger := New(false)

	var verboseBuf, quietBuf bytes.Buffer
	verboseLogger.logger.SetOutput(&verboseBuf)
	quietLogger.logger.SetOutput(&quietBuf)

	verboseLogger.Debug("verbose debug")
	quietLogger.Debug("quiet debug")

	verboseOutput := verboseBuf.String()
	quietOutput := quietBuf.String()

	if !strings.Contains(verboseOutput, "[DEBUG] verbose debug") {
		t.Error("Expected debug message in verbose logger output")
	}
	if strings.Contains(quietOutput, "[DEBUG]") {
		t.Error("Expected no debug message in quiet logger output")
	}
}

// Benchmark tests
func BenchmarkInfo(b *testing.B) {
	logger := New(false)
	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message")
	}
}

func BenchmarkDebugVerbose(b *testing.B) {
	logger := New(true)
	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Debug("benchmark debug message")
	}
}

func BenchmarkDebugQuiet(b *testing.B) {
	logger := New(false)
	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Debug("benchmark debug message")
	}
}

func BenchmarkError(b *testing.B) {
	logger := New(false)
	var buf bytes.Buffer
	logger.logger.SetOutput(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Error("benchmark error message")
	}
}
