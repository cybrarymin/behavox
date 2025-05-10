/*
Copyright © 2025 ryan
*/
package cmd

import (
	"os"
	"time"

	"github.com/cybrarymin/behavox/api"
	observ "github.com/cybrarymin/behavox/api/observability"
	data "github.com/cybrarymin/behavox/internal/models"
	"github.com/cybrarymin/behavox/worker"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "behvox",
	Short: "A simple rest api for adding event to a queue of events",
	Long:  `A simple rest api for adding event to a queue of events`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	PreRun: func(cmd *cobra.Command, args []string) {
	},

	Run: func(cmd *cobra.Command, args []string) {
		api.Main()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&api.CmdLogLevelFlag, "log-level", "info", "loglevel. possible values are debug, info, warn, error, fatal, panic, and trace")
	rootCmd.PersistentFlags().StringVar(&api.CmdHTTPSrvListenAddr, "listen-addr", "http://0.0.0.0:80", "listen address for the http/https service")
	rootCmd.PersistentFlags().StringVar(&observ.CmdJaegerHostFlag, "jeager-host", "localhost", "Jaeger/jaeger-collector server address for sending opentelemetry traces")
	rootCmd.PersistentFlags().StringVar(&observ.CmdJaegerPortFlag, "jeager-port", "5317", "Jaeger/jaeger-collector server port for sending opentelemetry traces")
	rootCmd.PersistentFlags().DurationVar(&observ.CmdJaegerConnectionTimeout, "jeager-conn-timeout", time.Second*5, "connection will fail if it couldn't be established to jaeger host within this time")
	rootCmd.PersistentFlags().DurationVar(&observ.CmdSpanExportInterval, "jeager-trace-exporter-intervals", time.Second*5, "intervals which tracer batch exporter will send the traces to the jeager")
	rootCmd.Flags().DurationVar(&api.CmdHTTPSrvWriteTimeout, "srv-write-timeout", 3*time.Second, "http server response write timeout")
	rootCmd.Flags().DurationVar(&api.CmdHTTPSrvReadTimeout, "srv-read-timeout", 3*time.Second, "http server response write timeout")
	rootCmd.Flags().DurationVar(&api.CmdHTTPSrvIdleTimeout, "srv-idle-timeout", 1*time.Minute, "http server idle timeout")
	rootCmd.Flags().StringVar(&api.CmdTlsCertFile, "cert", "/etc/ssl/cert.pem", "certificate file for https serving")
	rootCmd.Flags().StringVar(&api.CmdTlsKeyFile, "cert-key", "/etc/ssl/key.pem", "key file for https serving")
	rootCmd.Flags().Int64Var(&api.CmdGlobalRateLimit, "global-request-rate-limit", 25, "used to apply rate limiting to total number of requests coming to the api server. 10% of the specified value will be considered as the burst limit for total number of requests")
	rootCmd.Flags().Int64Var(&api.CmdPerClientRateLimit, "per-client-rate-limit", 2, "used to apply rate limiting to per client number of requests coming to the api server. 10% of the specified value will be considered as the burst limit for total number of requests")
	rootCmd.Flags().BoolVar(&api.CmdEnableRateLimit, "enable-rate-limit", false, "enable rate limiting")
	rootCmd.Flags().StringVar(&api.CmdApiAdmin, "api-admin-user", "behavox-admin", "api admin user for basic authentication and token issueing")
	rootCmd.Flags().StringVar(&api.CmdApiAdminPass, "api-admin-pass", "behavox-pass", "api admin password for basic authentication and token issuing ")
	rootCmd.Flags().StringVar(&api.CmdJwtKey, "jwkey", "defaultJWTToken", "jwt key for signing and verifying the issued jwt token")
	rootCmd.Flags().Int64Var(&data.CmdEventQueueSize, "event-queue-size", 100, "event queue size")
	rootCmd.Flags().IntVar(&worker.CmdmaxWorkerGoroutines, "event-queue-max-worker-threads", 5, "number of threds worker is allowed to create to process the events")
	rootCmd.Flags().StringVar(&worker.CmdProcessedEventFile, "event-processor-file", "/tmp/events.json", "file path for the worker to persist the logs processing information in json format")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
