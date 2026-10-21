// Command superping is a portable network monitoring tool. It pings the
// configured addresses, shows the round trip time in real time and keeps a
// daily rotated log of every probe.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/superping/superping/internal/config"
	"github.com/superping/superping/internal/history"
	"github.com/superping/superping/internal/logstore"
	"github.com/superping/superping/internal/model"
	"github.com/superping/superping/internal/monitor"
	"github.com/superping/superping/internal/ui"
)

// version is the semantic version of the application.
var version = "0.1.1"

func main() {
	var (
		configPath  = flag.String("config", "", "path of the configuration file")
		dataDir     = flag.String("data-dir", "", "directory of the persistent data")
		showVersion = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("superping %s\n", version)
		return
	}

	path := *configPath
	if path == "" {
		path = config.DefaultPath()
	}

	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "superping: %v\n", err)
		os.Exit(1)
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "superping: %v\n", err)
		os.Exit(1)
	}

	store, err := logstore.New(cfg.LogDir(), time.Duration(cfg.LogRetentionDays)*24*time.Hour)
	if err != nil {
		fmt.Fprintf(os.Stderr, "superping: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	hist := history.New(cfg.GraphWindow.Duration())
	mon := monitor.New()
	mon.SetICMode(cfg.ICMode)
	mon.SetSink(func(s model.Sample) {
		hist.Add(s)
		if err := store.Write(s); err != nil {
			fmt.Fprintf(os.Stderr, "superping: %v\n", err)
		}
	})
	for _, target := range cfg.Targets {
		mon.Upsert(target)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mon.Start(ctx)
	defer mon.Stop()

	application := ui.New(&cfg, path, mon, hist, store)
	application.Run()
}
