package log

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
)

var (
	logCh = make(chan Event)
	_     = NewObservable[Event](logCh)
	level = INFO
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:             true,
		TimestampFormat:           "2006-01-02T15:04:05.999999999Z07:00",
		EnvironmentOverrideColors: true,
	})
}

type Event struct {
	LogLevel LogLevel
	Payload  string
}

func (e *Event) Type() string {
	return e.LogLevel.String()
}

func Infoln(format string, v ...any) {
	event := newLog(INFO, format, v...)
	logCh <- event
	print(event)
}

func Warnln(format string, v ...any) {
	event := newLog(WARNING, format, v...)
	logCh <- event
	print(event)
}

func Errorln(format string, v ...any) {
	event := newLog(ERROR, format, v...)
	logCh <- event
	print(event)
}

func Debugln(format string, v ...any) {
	event := newLog(DEBUG, format, v...)
	logCh <- event
	print(event)
}

func Fatalln(format string, v ...any) {
	log.Fatalf(format, v...)
}

func print(data Event) {
	if data.LogLevel < level {
		return
	}

	switch data.LogLevel {
	case INFO:
		log.Infoln(data.Payload)
	case WARNING:
		log.Warnln(data.Payload)
	case ERROR:
		log.Errorln(data.Payload)
	case DEBUG:
		log.Debugln(data.Payload)
	}
}

func newLog(logLevel LogLevel, format string, v ...any) Event {
	return Event{
		LogLevel: logLevel,
		Payload:  fmt.Sprintf(format, v...),
	}
}
