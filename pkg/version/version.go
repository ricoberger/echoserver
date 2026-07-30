package version

import (
	"log/slog"
	"runtime"
)

// Build information. Populated at build-time.
var (
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
	GoVersion = runtime.Version()
)

// Info returns version, branch and revision information.
func Info() {
	slog.Info("Version information.", slog.String("version", Version), slog.String("branch", Branch), slog.String("revision", Revision))
}

// BuildContext returns goVersion, buildUser and buildDate information.
func BuildContext() {
	slog.Info("Build information.", slog.String("go", GoVersion), slog.String("user", BuildUser), slog.String("date", BuildDate))
}
