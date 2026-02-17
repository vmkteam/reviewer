package app

import (
	"context"
	"time"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/rpc"
	"reviewsrv/pkg/vt"

	"github.com/go-pg/pg/v10"
	monitor "github.com/hypnoglow/go-pg-monitor"
	"github.com/labstack/echo/v4"
	"github.com/vmkteam/appkit"
	"github.com/vmkteam/embedlog"
	"github.com/vmkteam/rpcgen/v2"
	"github.com/vmkteam/rpcgen/v2/typescript"
	"github.com/vmkteam/zenrpc/v2"
)

type Config struct {
	Database *pg.Options
	Server   struct {
		Host      string
		Port      int
		IsDevel   bool
		EnableVFS bool
		BaseURL   string
	}
	Sentry struct {
		Environment string
		DSN         string
	}
}

type App struct {
	embedlog.Logger
	appName string
	version string
	cfg     Config
	db      db.DB
	dbc     *pg.DB
	mon     *monitor.Monitor
	echo    *echo.Echo
	vtsrv   *zenrpc.Server
	srv     *zenrpc.Server
}

func New(appName, version string, sl embedlog.Logger, cfg Config, db db.DB, dbc *pg.DB) *App {
	a := &App{
		appName: appName,
		version: version,
		cfg:     cfg,
		db:      db,
		dbc:     dbc,
		echo:    appkit.NewEcho(),
		Logger:  sl,
	}

	// add services
	a.vtsrv = vt.New(a.db, a.Logger, a.cfg.Server.IsDevel, a.cfg.Server.BaseURL)
	a.srv = rpc.New(a.db, a.Logger, a.cfg.Server.IsDevel, a.version)

	return a
}

// Run is a function that runs application.
func (a *App) Run(ctx context.Context) error {
	a.registerMetrics()
	a.registerHandlers()
	a.registerDebugHandlers()
	a.registerAPIHandlers()
	a.registerVTApiHandlers()
	if err := a.registerFrontendHandlers(); err != nil {
		return err
	}
	if err := a.registerVTFrontendHandlers(); err != nil {
		return err
	}
	a.registerMetadata()

	return a.runHTTPServer(ctx, a.cfg.Server.Host, a.cfg.Server.Port)
}

// TypeScriptClient returns TypeScript client for VT or RPC.
func (a *App) TypeScriptClient(client string) ([]byte, error) {
	gen := rpcgen.FromSMD(a.srv.SMD())
	if client == "vt" {
		gen = rpcgen.FromSMD(a.vtsrv.SMD())
	}

	tsSettings := typescript.Settings{ExcludedNamespace: []string{}, WithClasses: true}
	return gen.TSCustomClient(tsSettings).Generate()
}

// Shutdown is a function that gracefully stops HTTP server.
func (a *App) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	a.mon.Close()

	return a.echo.Shutdown(ctx)
}

// registerMetadata is a function that registers meta info from service. Must be updated.
func (a *App) registerMetadata() {
	opts := appkit.MetadataOpts{
		HasPublicAPI:  true,
		HasPrivateAPI: true,
		DBs: []appkit.DBMetadata{
			appkit.NewDBMetadata(a.cfg.Database.Database, a.cfg.Database.PoolSize, false),
		},
		Services: []appkit.ServiceMetadata{
			// NewServiceMetadata("srv", MetadataServiceTypeAsync),
		},
	}

	md := appkit.NewMetadataManager(opts)
	md.RegisterMetrics()

	a.echo.GET("/debug/metadata", md.Handler)
}
