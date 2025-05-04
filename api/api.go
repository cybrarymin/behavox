package api

import (
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	helpers "github.com/cybrarymin/behavox/internal"
	"github.com/rs/zerolog"
)

var (
	Version   string
	BuildTime string
)

type ApiServerCfg struct {
	ListenAddr         *url.URL      // http server listen address url
	ServerReadTimeout  time.Duration // amount of time allowed to read a request body otherwise server will return an error
	ServerWriteTimeout time.Duration // amount of time allowed to write a response for the client
	ServerIdleTimeout  time.Duration // amount of time in idle mode before closing the connection with client
	TlsCertFile        string        // Tls certificate file for https serving
	TlsKeyFile         string        // Tls key file https serving
	RateLimit          struct {
		GlobalRateLimit    int64
		perClientRateLimit int64
		Enabled            bool
	}
}

func NewApiServerCfg(listenAddr *url.URL, tlsCertFile string, tlsKeyFile string, rateLimitEnabled bool, globalRateLimit int64, perCleintRateLimit int64, srvReadTimeout, srvIdleTimeout, srvWriteTimeout time.Duration) *ApiServerCfg {
	return &ApiServerCfg{
		ListenAddr:         listenAddr,
		ServerReadTimeout:  srvReadTimeout,
		ServerWriteTimeout: srvWriteTimeout,
		ServerIdleTimeout:  srvIdleTimeout,
		TlsCertFile:        tlsCertFile,
		TlsKeyFile:         tlsKeyFile,
		RateLimit: struct {
			GlobalRateLimit    int64
			perClientRateLimit int64
			Enabled            bool
		}{
			GlobalRateLimit:    globalRateLimit,
			Enabled:            rateLimitEnabled,
			perClientRateLimit: perCleintRateLimit,
		},
	}
}

func (cfg *ApiServerCfg) validation(nVal helpers.Validator) *helpers.Validator {
	nVal.Check(cfg.ListenAddr.Scheme == "http" || cfg.ListenAddr.Scheme == "https", "listen-addr", "invalid schema")
	if cfg.ListenAddr.Scheme == "https" {
		_, err := os.Stat(cfg.TlsCertFile)
		nVal.Check(err == nil, "tls-certfile", fmt.Sprintf("%s doesn't exists", cfg.TlsCertFile))
		_, err = os.Stat(cfg.TlsKeyFile)
		nVal.Check(err == nil, "tls-key", fmt.Sprintf("%s doesn't exists", cfg.TlsKeyFile))
	}
	return &nVal
}

type ApiServer struct {
	Cfg    *ApiServerCfg
	Logger *zerolog.Logger
	Wg     sync.WaitGroup
	mu     sync.RWMutex
}

func NewApiServer(cfg *ApiServerCfg, logger *zerolog.Logger) *ApiServer {
	return &ApiServer{
		Cfg:    cfg,
		Logger: logger,
	}
}
