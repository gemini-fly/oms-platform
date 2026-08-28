package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gemini-fly/oms-platform/internal/apptree"
	"github.com/gemini-fly/oms-platform/internal/audit"
	"github.com/gemini-fly/oms-platform/internal/buildinfo"
	"github.com/gemini-fly/oms-platform/internal/cicd"
	"github.com/gemini-fly/oms-platform/internal/cmdb"
	"github.com/gemini-fly/oms-platform/internal/deploy"
	"github.com/gemini-fly/oms-platform/internal/iam"
	"github.com/gemini-fly/oms-platform/internal/itsm"
	"github.com/gemini-fly/oms-platform/internal/notify"
	"github.com/gemini-fly/oms-platform/internal/org"
	"github.com/gemini-fly/oms-platform/internal/platform"
	"github.com/gemini-fly/oms-platform/internal/web"
)

func main() {
	addr := flag.String("addr", env("HTTP_ADDR", "127.0.0.1:8080"), "HTTP listen address")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(buildinfo.String())
		return
	}

	store := platform.NewStore()
	if dsn := platform.DBDSNFromEnv(); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := store.ConnectPostgres(ctx, dsn); err != nil {
			cancel()
			log.Fatalf("connect local database failed: %v", err)
		}
		cancel()
		defer store.Close()
		log.Printf("postgres persistence enabled")
	}

	server := platform.NewServer(store)

	platform.RegisterSettings(server)
	iam.Register(server)
	org.Register(server)
	apptree.Register(server)
	itsm.Register(server)
	cmdb.Register(server)
	cicd.Register(server)
	deploy.Register(server)
	notify.Register(server)
	audit.Register(server)
	web.Register(server)

	log.Printf("oms-platform api listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, platform.Logging(server.Handler())))
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
