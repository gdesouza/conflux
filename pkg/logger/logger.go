package logger

import (
	"fmt"
	"log"
	"os"
)

type Logger struct {
	verbose bool
	logger  *log.Logger
}

func New(verbose bool) *Logger {
	return &Logger{
		verbose: verbose,
		logger:  log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.logger.Printf("[INFO] "+format, args...)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if l.verbose {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+format, args...)
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.logger.Printf("[FATAL] "+format, args...)
	os.Exit(1)
}

func (l *Logger) Print(v ...interface{}) {
	fmt.Print(v...)
}

func (l *Logger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (l *Logger) Println(v ...interface{}) {
	fmt.Println(v...)
}
