package app

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	_ "net/http/pprof"

	"reviewsrv/frontend"
	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/rpc"
	"reviewsrv/pkg/slack"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/vmkteam/appkit"
	"github.com/vmkteam/rpcgen/v2"
	"github.com/vmkteam/rpcgen/v2/typescript"
	"github.com/vmkteam/zenrpc/v2"
)

// runHTTPServer is a function that starts http listener using labstack/echo.
func (a *App) runHTTPServer(ctx context.Context, host string, port int) error {
	listenAddress := fmt.Sprintf("%s:%d", host, port)
	addr := "http://" + listenAddress
	a.Print(ctx, "starting http listener", "url", addr, "smdbox", addr+"/v1/rpc/doc/")

	return a.echo.Start(listenAddress)
}

// registerHandlers register echo handlers.
func (a *App) registerHandlers() {
	a.echo.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.PUT, echo.POST, echo.DELETE},
		AllowHeaders: []string{"Authorization", "Authorization2", "Origin", "X-Requested-With", "Content-Type", "Accept", "Platform", "Version"},
	}), middleware.BodyLimit("2M"))

	lg := middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:    true,
		LogURI:       true,
		LogError:     true,
		HandleError:  true,
		LogLatency:   true,
		LogRemoteIP:  true,
		LogRequestID: true,
		LogUserAgent: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			attrs := []slog.Attr{
				slog.String("ip", v.RemoteIP),
				slog.String("uri", v.URI),
				slog.Int("status", v.Status),
				slog.String("userAgent", v.UserAgent),
				slog.String("duration", v.Latency.String()),
				slog.String("xRequestId", v.RequestID),
			}

			if v.Error == nil {
				a.Log().LogAttrs(context.Background(), slog.LevelInfo, "http request", attrs...)
			} else {
				a.Log().LogAttrs(context.Background(), slog.LevelError, "http request error", append(attrs, slog.String("err", v.Error.Error()))...)
			}
			return nil
		},
	})

	h := rest.NewHandler(a.db, slack.NewNotifier(a.Logger), a.cfg.Server.BaseURL)

	a.echo.GET("/v1/prompt/:projectKey/", h.GetPrompt, lg)
	a.echo.GET("/v1/upload/upload.js", h.GetUploadScript, lg)
	a.echo.POST("/v1/upload/:projectKey/", h.CreateReview, lg)
	a.echo.POST("/v1/upload/:projectKey/:reviewId/:reviewType/", h.UploadReviewFile, lg)
}

// registerDebugHandlers adds /debug/pprof handlers into a.echo instance.
func (a *App) registerDebugHandlers() {
	dbg := a.echo.Group("/debug")

	// add pprof integration
	dbg.Any("/pprof/*", appkit.PprofHandler)

	// add healthcheck
	a.echo.GET("/status", func(c echo.Context) error {
		// test postgresql connection
		err := a.db.Ping(c.Request().Context())
		if err != nil {
			a.Error(c.Request().Context(), "failed to check db connection", "err", err)
			return c.String(http.StatusInternalServerError, "DB error")
		}
		return c.String(http.StatusOK, "OK")
	})

	// show all routes in devel mode
	if a.cfg.Server.IsDevel {
		a.echo.GET("/", appkit.RenderRoutes(a.appName, a.echo))
	} else {
		a.echo.GET("/", func(c echo.Context) error {
			return c.Redirect(http.StatusFound, "/reviews/")
		})
	}
}

// registerAPIHandlers registers main rpc server.
func (a *App) registerAPIHandlers() {
	srv := rpc.New(a.db, a.Logger, a.cfg.Server.IsDevel)
	gen := rpcgen.FromSMD(srv.SMD())

	a.echo.Any("/v1/rpc/", appkit.EchoHandler(appkit.XRequestID(srv)))
	a.echo.Any("/v1/rpc/doc/", appkit.EchoHandlerFunc(zenrpc.SMDBoxHandler))
	a.echo.Any("/v1/rpc/openrpc.json", appkit.EchoHandlerFunc(rpcgen.Handler(gen.OpenRPC("reviewsrv", "http://localhost:8075/v1/rpc"))))
	a.echo.Any("/v1/rpc/api.ts", appkit.EchoHandlerFunc(rpcgen.Handler(gen.TSClient(nil))))
}

// registerSPAHandlers serves an embedded SPA at the given prefix.
// indexFile is the name of the HTML entry point inside distFS (e.g. "index.html", "vt.html").
func (a *App) registerSPAHandlers(distFS fs.FS, prefix, indexFile string) {
	fileServer := http.FileServer(http.FS(distFS))

	// serve static assets
	a.echo.GET(prefix+"assets/*", echo.WrapHandler(http.StripPrefix(prefix, fileServer)))
	a.echo.GET(prefix+"favicon.svg", echo.WrapHandler(http.StripPrefix(prefix, fileServer)))

	// SPA fallback
	a.echo.GET(prefix+"*", func(c echo.Context) error {
		index, err := fs.ReadFile(distFS, indexFile)
		if err != nil {
			return c.String(http.StatusInternalServerError, "frontend not found")
		}
		return c.HTMLBlob(http.StatusOK, index)
	})
	a.echo.GET(prefix[:len(prefix)-1], func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, prefix)
	})
}

// registerFrontendHandlers serves the embedded SPA frontend at /reviews/.
func (a *App) registerFrontendHandlers() error {
	distFS, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("frontend fs: %w", err)
	}
	a.registerSPAHandlers(distFS, "/reviews/", "index.html")
	return nil
}

// registerVTApiHandlers registers vt rpc server.
func (a *App) registerVTApiHandlers() {
	gen := rpcgen.FromSMD(a.vtsrv.SMD())
	tsSettings := typescript.Settings{ExcludedNamespace: []string{}, WithClasses: true}

	a.echo.Any("/v1/vt/", appkit.EchoHandler(appkit.XRequestID(a.vtsrv)))
	a.echo.Any("/v1/vt/doc/", appkit.EchoHandlerFunc(zenrpc.SMDBoxHandler))
	a.echo.Any("/v1/vt/api.ts", appkit.EchoHandlerFunc(rpcgen.Handler(gen.TSCustomClient(tsSettings))))
}

// registerVTFrontendHandlers serves the embedded VT admin SPA at /vt/.
func (a *App) registerVTFrontendHandlers() error {
	distFS, err := fs.Sub(frontend.DistVTFS, "dist-vt")
	if err != nil {
		return fmt.Errorf("vt frontend fs: %w", err)
	}
	a.registerSPAHandlers(distFS, "/vt/", "vt.html")
	return nil
}
