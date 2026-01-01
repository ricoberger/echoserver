package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ricoberger/echoserver/pkg/httpserver"
	"github.com/ricoberger/echoserver/pkg/instrument"
	"github.com/ricoberger/echoserver/pkg/version"

	"github.com/alecthomas/kong"
)

type Cli struct {
	HTTPServer httpserver.Config `embed:"" prefix:"http-server." envprefix:"HTTP_SERVER_"`
}

func main() {
	var cli Cli

	ctx := kong.Parse(&cli, kong.Name("echoserver"))
	ctx.FatalIfErrorf(ctx.Error)
	ctx.FatalIfErrorf(cli.run())
}

func (c *Cli) run() error {
	instrumentClient, err := instrument.New(context.Background())
	if err != nil {
		return err
	}
	defer instrumentClient.Shutdown()

	version.Info()
	version.BuildContext()

	httpServer := httpserver.New(c.HTTPServer)
	go httpServer.Start()

	// All components should be terminated gracefully. For that we are listen
	// for the SIGINT and SIGTERM signals and try to gracefully shutdown the
	// started components. This ensures that established connections or tasks
	// are not interrupted.
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	slog.Info("Start listening for SIGINT and SIGTERM signal.")
	<-done
	slog.Info("Shutdown started.")

	httpServer.Stop()

	slog.Info("Shutdown done.")

	return nil
}
