package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	apiObserv "github.com/cybrarymin/behavox/api/observability"
	helpers "github.com/cybrarymin/behavox/internal"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

var (
	CmdLogLevelFlag        string
	CmdHTTPSrvListenAddr   string
	CmdHTTPSrvReadTimeout  time.Duration
	CmdHTTPSrvWriteTimeout time.Duration
	CmdHTTPSrvIdleTimeout  time.Duration
	CmdTlsCertFile         string
	CmdTlsKeyFile          string
	CmdGlobalRateLimit     int64
	CmdPerClientRateLimit  int64
	CmdEnableRateLimit     bool
)

func Main() {
	// initializing the logger with respect to the specified loglevel option
	var nlogger zerolog.Logger
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	if zerolog.LevelTraceValue == CmdLogLevelFlag {
		nlogger = zerolog.New(os.Stdout).With().Stack().Timestamp().Logger().Level(zerolog.TraceLevel)
	} else {
		loglvl, _ := zerolog.ParseLevel(CmdLogLevelFlag)
		nlogger = zerolog.New(os.Stdout).With().Timestamp().Logger().Level(loglvl)
	}

	ctx := context.Background()

	// initialize opentelemetry
	otelShut, err := apiObserv.SetupOTelSDK(ctx, apiObserv.CmdJaegerHostFlag, apiObserv.CmdJaegerPortFlag, apiObserv.CmdJaegerConnectionTimeout, apiObserv.CmdSpanExportInterval)
	if err != nil {
		nlogger.Error().Err(err).Msg("failed to initialize the opentelemetry sdk")
		return
	}

	// initialize the prometheus
	apiObserv.PromInit()

	// initializing new validator to be used for input validation of cmdOptions
	nVal := helpers.NewValidator()

	// parsing the listen address
	url, err := url.Parse(CmdHTTPSrvListenAddr)
	if err != nil {
		nlogger.Error().Err(err).Send()
		return
	}

	nApiCfg := NewApiServerCfg(url, CmdTlsCertFile,
		CmdTlsKeyFile,
		CmdEnableRateLimit,
		CmdGlobalRateLimit,
		CmdPerClientRateLimit,
		CmdHTTPSrvReadTimeout,
		CmdHTTPSrvIdleTimeout,
		CmdHTTPSrvWriteTimeout)
	if !nApiCfg.validation(*nVal).Valid() {
		for key, err := range nVal.Errors {
			err := fmt.Errorf("%s is invalid: %s", key, err)
			nlogger.Error().Err(err).Send()
		}
		return
	}

	nApi := NewApiServer(nApiCfg, &nlogger)
	nSrv := http.Server{
		Addr:         nApi.Cfg.ListenAddr.Host,
		Handler:      nApi.routes(),
		ReadTimeout:  nApi.Cfg.ServerReadTimeout,
		WriteTimeout: nApi.Cfg.ServerWriteTimeout,
		IdleTimeout:  nApi.Cfg.ServerIdleTimeout,
		ErrorLog:     log.New(nApi.Logger, "", 0),
	}

	shutdownChan := make(chan error)
	go nApi.gracefulShutdown(nApi.Logger, shutdownChan, nSrv.Shutdown, otelShut)

	if nApi.Cfg.ListenAddr.Scheme == "https" {
		nlogger.Info().Msgf("starting the server on %s over %s", nApi.Cfg.ListenAddr.Host, nApi.Cfg.ListenAddr.Scheme)
		err := nSrv.ListenAndServeTLS(nApi.Cfg.TlsCertFile, nApi.Cfg.TlsKeyFile)
		if err != nil && err != http.ErrServerClosed {
			nlogger.Error().Err(err).Send()
			return
		}
	} else {
		nlogger.Info().Msgf("starting the server on %s over %s", nApi.Cfg.ListenAddr.Host, nApi.Cfg.ListenAddr.Scheme)
		err := nSrv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			nlogger.Error().Err(err).Send()
			return
		}
	}

	err = <-shutdownChan
	if err != nil {
		nlogger.Error().Err(err).Send()
	}
}

// gracefulShitdown catches the terminate, quit, interrupt signals and closes the connection gracefully
func (api *ApiServer) gracefulShutdown(logger *zerolog.Logger, shutdownChan chan error, shutdownFuncs ...func(context.Context) error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	s := <-sigChan

	// log the signal catched
	logger.Warn().Msgf("catched os signal %s", s)

	// gracefully shutdown the services
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	for _, shutdownFunc := range shutdownFuncs {
		err := shutdownFunc(ctx)
		if err != nil {
			shutdownChan <- err
		}
	}

	// waiting for the background tasks to finish
	logger.Info().Msg("waiting for background tasks to finish")
	api.Wg.Wait()
	shutdownChan <- nil

	logger.Info().Msg("stopped the server")
}
