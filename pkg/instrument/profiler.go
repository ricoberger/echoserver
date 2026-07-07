package instrument

import (
	"fmt"
	"log/slog"

	"github.com/grafana/pyroscope-go"
)

type pyroscopeLogger struct{}

func (l *pyroscopeLogger) Debugf(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...))
}

func (l *pyroscopeLogger) Infof(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

func (l *pyroscopeLogger) Errorf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
}

func newPyroscopeLogger() pyroscope.Logger {
	return &pyroscopeLogger{}
}
