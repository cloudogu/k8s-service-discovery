package logging

import (
	"fmt"
	"os"

	"github.com/bombsimon/logrusr/v2"
	"github.com/cloudogu/cesapp-lib/core"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
)

const namespaceLogLevel = "LOG_LEVEL"

const (
	errorLevel int = iota
	warningLevel
	infoLevel
	debugLevel
)

type logSink interface {
	logr.LogSink
}

type libraryLogger struct {
	logger logSink
	name   string
}

func (l *libraryLogger) log(level int, args ...interface{}) {
	l.logger.Info(level, fmt.Sprintf("[%s] %s", l.name, fmt.Sprint(args...)))
}

func (l *libraryLogger) logf(level int, format string, args ...interface{}) {
	l.logger.Info(level, fmt.Sprintf("[%s] %s", l.name, fmt.Sprintf(format, args...)))
}

func (l *libraryLogger) Debug(args ...interface{}) {
	l.log(debugLevel, args...)
}

func (l *libraryLogger) Info(args ...interface{}) {
	l.log(infoLevel, args...)
}

func (l *libraryLogger) Warning(args ...interface{}) {
	l.log(warningLevel, args...)
}

func (l *libraryLogger) Error(args ...interface{}) {
	l.log(errorLevel, args...)
}

func (l *libraryLogger) Debugf(format string, args ...interface{}) {
	l.logf(debugLevel, format, args...)
}

func (l *libraryLogger) Infof(format string, args ...interface{}) {
	l.logf(infoLevel, format, args...)
}

func (l *libraryLogger) Warningf(format string, args ...interface{}) {
	l.logf(warningLevel, format, args...)
}

func (l *libraryLogger) Errorf(format string, args ...interface{}) {
	l.logf(errorLevel, format, args...)
}

func getLogLevelFromEnv() (logrus.Level, error) {
	logLevel, found := os.LookupEnv(namespaceLogLevel)
	if !found {
		return logrus.ErrorLevel, nil
	}

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return logrus.ErrorLevel, fmt.Errorf("value of log environment variable [%s] is not a valid log level: %w", namespaceLogLevel, err)
	}

	return level, nil
}

func ConfigureLogger() error {
	level, err := getLogLevelFromEnv()
	if err != nil {
		return err
	}

	// create logrus logger that can be styled and formatted
	logrusLog := logrus.New()
	logrusLog.SetFormatter(&logrus.TextFormatter{})
	logrusLog.SetLevel(level)

	// convert logrus logger to logr logger
	logrusLogrLogger := logrusr.New(logrusLog)

	// set logr logger as controller logger
	ctrl.SetLogger(logrusLogrLogger)

	// set custom logger implementation to cesapp-lib logger
	cesappLibLogger := libraryLogger{name: "cesapp-lib", logger: logrusLogrLogger.GetSink()}
	core.GetLogger = func() core.Logger {
		return &cesappLibLogger
	}

	return nil
}
