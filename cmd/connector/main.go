package main

import (
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/fatih/color"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/cmd/connector/provider"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/statusHandler/presenter"
	"github.com/klever-io/klever-go/statusHandler/view/termuic"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type config struct {
	logWithCorrelation bool
	logWithLoggerName  bool
	useWss             bool
	interval           int
	address            string
	logLevel           string
}

var (
	log        = logger.GetOrCreate("connector")
	appVersion = core.UnVersionedAppString
	g          = color.New(color.FgGreen).SprintFunc()
)

var (
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "connector",
	Short:        "Klever Connector ",
	SilenceUsage: true,
	Version:      appVersion,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if verbose {
			log.SetLevel(logger.LogTrace)
		}

		return nil
	},
	Long: fmt.Sprintf(`
CONNECTOR interface to Klever Blockchain

%s`, g("type 'connector --help' for details")),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var conf config

func init() {
	rootCmd.PersistentFlags().StringVarP(&conf.address, "address", "a", "127.0.0.1:8080", "Address and port number on which the application will try to connect to the klever node")
	rootCmd.PersistentFlags().StringVarP(&conf.logLevel, "log-level", "l", "*:"+logger.LogInfo.String(), "This flag specifies the logger `level(s)`. It can contain multiple comma-separated value. For example"+
		", if set to *:INFO the logs for all packages will have the INFO level. However, if set to *:INFO,api:DEBUG"+
		" the logs for all packages will have the INFO level, excepting the api package which will receive a DEBUG"+
		" log level.")
	rootCmd.PersistentFlags().BoolVarP(&conf.logWithCorrelation, "log-correlation", "c", false, "Boolean option for enabling log correlation elements.")
	rootCmd.PersistentFlags().BoolVarP(&conf.logWithLoggerName, "log-logger-name", "n", false, "Boolean option for logger name in the logs.")
	rootCmd.PersistentFlags().IntVarP(&conf.interval, "interval", "i", 1000, "This flag specifies the duration in milliseconds until new data is fetched from the node")
	rootCmd.PersistentFlags().BoolVarP(&conf.useWss, "use-wss", "w", false, "Will use wss instead of ws when creating the web socket")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		errMsg := errors.Wrapf(err, "commit: %s, error", appVersion).Error()
		fmt.Fprintf(os.Stderr, errMsg+"\n")
		fmt.Fprintf(os.Stderr, "try adding a `--help` flag\n")
		os.Exit(1)
	}
}

func main() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stderr,
				"Klever Connector. %v version %s\n",
				path.Base(os.Args[0]), appVersion)
			os.Exit(0)
			return nil
		},
	}, &cobra.Command{
		Use:   "node",
		Short: "Connect to Node",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return startConnectorViewer(&conf)
		},
	})
	Execute()
}

func startConnectorViewer(argsConfig *config) error {
	nodeAddress := argsConfig.address
	fetchIntervalFlagValue := argsConfig.interval

	chanNodeIsStarting := make(chan struct{})

	presenterStatusHandler := presenter.NewPresenterStatusHandler()
	statusMetricsProvider, err := provider.NewStatusMetricsProvider(presenterStatusHandler, nodeAddress, fetchIntervalFlagValue)
	if err != nil {
		return err
	}

	connectorConsole, err := termuic.NewConnectorConsole(presenterStatusHandler, fetchIntervalFlagValue, chanNodeIsStarting)
	if err != nil {
		return err
	}

	statusMetricsProvider.StartUpdatingData()

	loggerProfile := &logger.Profile{
		LogLevelPatterns: argsConfig.logLevel,
		WithCorrelation:  argsConfig.logWithCorrelation,
		WithLoggerName:   argsConfig.logWithLoggerName,
	}
	customLogProfile := argsConfig.logLevel != "*:"+logger.LogInfo.String() || argsConfig.logWithCorrelation || argsConfig.logWithLoggerName
	if customLogProfile {
		log.LogIfError(err)
	}

	argsLogHandler := provider.LogHandlerArgs{
		Presenter:          presenterStatusHandler,
		NodeURL:            nodeAddress,
		Profile:            loggerProfile,
		ChanNodeIsStarting: chanNodeIsStarting,
		UseWss:             argsConfig.useWss,
		CustomLogProfile:   customLogProfile,
	}
	err = provider.InitLogHandler(argsLogHandler)
	if err != nil {
		return err
	}

	err = connectorConsole.Start(chanNodeIsStarting)
	if err != nil {
		return err
	}

	chanNodeIsStarting <- struct{}{}
	waitForUserToTerminateApp()

	return nil
}

func waitForUserToTerminateApp() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs

	log.Info("terminating terminal ui app at user's signal...")

	provider.StopWebSocket()
}
