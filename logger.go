package cdkey

import "fmt"

type Logger interface {
	Info(v ...interface{})
	Error(v ...interface{})
}

var logger Logger

func SetLogger(l Logger) {
	logger = l
}

func info_log(v ...interface{}) {
	if logger != nil {
		logger.Info(v...)
	}
}

func error_log(v ...interface{}) {
	if logger != nil {
		logger.Error(v...)
	}
}

func info_logf(format string, v ...interface{}) {
	if logger != nil {
		logger.Info(fmt.Sprintf(format, v...))
	}
}

func error_logf(format string, v ...interface{}) {
	if logger != nil {
		logger.Error(fmt.Sprintf(format, v...))
	}
}
