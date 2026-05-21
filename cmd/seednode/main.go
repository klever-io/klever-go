package main

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/cmd/seednode/api"
	"github.com/klever-io/klever-go/common/facade"
	"github.com/klever-io/klever-go/config"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/network/p2p"
	"github.com/klever-io/klever-go/network/p2p/libp2p"
	"github.com/klever-io/klever-go/tools"
	"github.com/klever-io/klever-go/tools/check"
	"github.com/klever-io/klever-go/tools/display"
	"github.com/klever-io/klever-go/tools/logging"
	"github.com/klever-io/klever-go/tools/marshal"
	factoryMarshalizer "github.com/klever-io/klever-go/tools/marshal/factory"
	"github.com/urfave/cli"
)

const (
	defaultLogsPath     = "logs"
	logFilePrefix       = "klever-seed"
	filePathPlaceholder = "[path]"
)

var (
	seedNodeHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}
USAGE:
   {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}
   {{if len .Authors}}
AUTHOR:
   {{range .Authors}}{{ . }}{{end}}
   {{end}}{{if .Commands}}
GLOBAL OPTIONS:
   {{range .VisibleFlags}}{{.}}
   {{end}}
VERSION:
   {{.Version}}
   {{end}}
`
	// port defines a flag for setting the port on which the node will listen for connections
	port = cli.StringFlag{
		Name: "port",
		Usage: "The `[p2p port]` number on which the application will start. Can use single values such as " +
			"`0, 10230, 15670` or range of ports such as `5000-10000`",
		Value: "10000",
	}
	// restAPIInterfaceFlag defines a flag for the interface on which the rest API will try to bind with
	restAPIInterfaceFlag = cli.StringFlag{
		Name: "rest-api-interface",
		Usage: "The interface `address and port` to which the REST API will attempt to bind. " +
			"To bind to all available interfaces, set this flag to :8080. If set to `off` then the API won't be available",
		Value: facade.DefaultRestInterface,
	}
	// p2pSeed defines a flag to be used as a seed when generating P2P credentials. Useful for seed nodes.
	p2pSeed = cli.StringFlag{
		Name:  "p2p-seed",
		Usage: "P2P seed will be used when generating credentials for p2p component. Can be any string.",
		Value: "seed",
	}
	// logLevel defines the logger level
	logLevel = cli.StringFlag{
		Name: "log-level",
		Usage: "This flag specifies the logger `level(s)`. It can contain multiple comma-separated value. For example" +
			", if set to *:INFO the logs for all packages will have the INFO level. However, if set to *:INFO,api:DEBUG" +
			" the logs for all packages will have the INFO level, excepting the api package which will receive a DEBUG" +
			" log level.",
		Value: "*:" + logger.LogInfo.String(),
	}
	//logFile is used when the log output needs to be logged in a file
	logSaveFile = cli.BoolFlag{
		Name:  "log-save",
		Usage: "Boolean option for enabling log saving. If set, it will automatically save all the logs into a file.",
	}
	// configurationFile defines a flag for the path to the main toml configuration file
	configurationFile = cli.StringFlag{
		Name: "config",
		Usage: "The `" + filePathPlaceholder + "` for the main configuration file. This YAML file contain the main " +
			"configurations such as the marshalizer type",
		Value: "./config/seednode/config.yaml",
	}
)

var log = logger.GetOrCreate("main")

var appVersion = core.UnVersionedAppString

func main() {
	app := cli.NewApp()
	cli.AppHelpTemplate = seedNodeHelpTemplate
	app.Name = "SeedNode CLI App"
	app.Usage = "This is the entry point for starting a new seed node - the app will help bootnodes connect to the network"
	app.Flags = []cli.Flag{
		port,
		restAPIInterfaceFlag,
		p2pSeed,
		logLevel,
		logSaveFile,
		configurationFile,
	}
	app.Version = appVersion
	app.Authors = []cli.Author{
		{
			Name:  "The Klever Team",
			Email: "contact@klever.org",
		},
	}

	app.Action = func(c *cli.Context) error {
		return startNode(c)
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func startNode(ctx *cli.Context) error {
	startTime := time.Now()

	logLevelFlagValue := ctx.GlobalString(logLevel.Name)
	err := logger.SetLogLevel(logLevelFlagValue)
	if err != nil {
		return err
	}

	configurationFileName := ctx.GlobalString(configurationFile.Name)
	cfg, err := config.LoadFromPath(configurationFileName)
	if err != nil {
		return err
	}

	internalMarshalizer, err := factoryMarshalizer.NewMarshalizer(cfg.Marshalizer.Type)
	if err != nil {
		return fmt.Errorf("error creating marshalizer (internal): %s", err.Error())
	}

	withLogFile := ctx.GlobalBool(logSaveFile.Name)
	var fileLogging tools.FileLoggingHandler
	if withLogFile {
		workingDir := getWorkingDir(log)
		fileLogging, err = logging.NewFileLogging(workingDir, defaultLogsPath, logFilePrefix)
		if err != nil {
			return fmt.Errorf("%w creating a log file", err)
		}

		logFileLifeSpan := time.Second * time.Duration(cfg.Logs.LogFileLifeSpanInSec)
		logFileSizeSpan := time.Second * time.Duration(cfg.Logs.LogFileSizeSpanInSec)

		err = fileLogging.SetFileRotation(logFileLifeSpan, logFileSizeSpan, cfg.Logs.MaxBackups, cfg.Logs.MaxFileSize)
		if err != nil {
			return err
		}
	}

	log.Info("starting seednode...")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	if ctx.IsSet(port.Name) {
		cfg.P2P.Node.Port = ctx.GlobalString(port.Name)
	}
	if ctx.IsSet(p2pSeed.Name) {
		cfg.P2P.Node.Seed = ctx.GlobalString(p2pSeed.Name)
	}

	err = checkExpectedPeerCount(cfg.P2P)
	if err != nil {
		return err
	}

	messenger, err := createNode(cfg.P2P, internalMarshalizer)
	if err != nil {
		return err
	}

	err = messenger.Bootstrap()
	if err != nil {
		return err
	}

	startRestServices(ctx, internalMarshalizer, messenger, startTime)

	log.Info("application is now running...")
	mainLoop(messenger, sigs)

	log.Debug("closing seednode")
	if !check.IfNil(fileLogging) {
		err = fileLogging.Close()
		log.LogIfError(err)
	}

	return nil
}

const peerStatusTickInterval = 30 * time.Second

func mainLoop(messenger p2p.Messenger, stop chan os.Signal) {
	displayStartupAddresses(messenger)

	prevConnected := make(map[string]struct{})
	emitPeerStatus(messenger, prevConnected)

	ticker := time.NewTicker(peerStatusTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			log.Info("terminating at user's signal...")
			return
		case <-ticker.C:
			emitPeerStatus(messenger, prevConnected)
		}
	}
}

func createNode(p2pConfig config.P2PConfig, marshalizer marshal.Marshalizer) (p2p.Messenger, error) {
	arg := libp2p.ArgsNetworkMessenger{
		Marshalizer:   marshalizer,
		ListenAddress: libp2p.ListenAddrWithIp4AndTcp,
		P2pConfig:     p2pConfig,
		SyncTimer:     &libp2p.LocalSyncTimer{},
		IsSeedNode:    true,
	}

	return libp2p.NewNetworkMessenger(arg)
}

func displayStartupAddresses(messenger p2p.Messenger) {
	addresses := make([]*display.LineData, 0)
	for _, address := range messenger.Addresses() {
		addresses = append(addresses, display.NewLineData(false, []string{address}))
	}

	tbl, _ := display.CreateTableString([]string{"Seednode addresses:"}, addresses)
	log.Info("\n" + tbl)
}

func emitPeerStatus(messenger p2p.Messenger, prevConnected map[string]struct{}) {
	connected := messenger.ConnectedAddresses()
	sort.Strings(connected)

	known := len(messenger.Peers())

	curConnected := make(map[string]struct{}, len(connected))
	for _, addr := range connected {
		curConnected[addr] = struct{}{}
	}

	var gained, lost int
	for addr := range curConnected {
		if _, ok := prevConnected[addr]; !ok {
			gained++
		}
	}
	for addr := range prevConnected {
		if _, ok := curConnected[addr]; !ok {
			lost++
		}
	}

	listen := pickReportableListenAddress(messenger.Addresses())

	log.Info("seednode status",
		"connected", len(connected),
		"known", known,
		"gained", gained,
		"lost", lost,
		"listen", listen,
	)

	if log.GetLevel() <= logger.LogDebug {
		headerConnectedAddresses := []string{fmt.Sprintf("Seednode is connected to %d peers:", len(connected))}
		rows := make([]*display.LineData, len(connected))
		for idx, address := range connected {
			rows[idx] = display.NewLineData(false, []string{address})
		}
		tbl, _ := display.CreateTableString(headerConnectedAddresses, rows)
		log.Debug("\n" + tbl)
	}

	for addr := range prevConnected {
		delete(prevConnected, addr)
	}
	for addr := range curConnected {
		prevConnected[addr] = struct{}{}
	}
}

// pickReportableListenAddress returns the first multiaddr useful to a human
// reading the single-line status log: skip loopback and the Docker default
// bridge (172.17.x), which appears on every container-hosted seed but never
// helps operators find the node. Falls back to the first address so the log
// line is never empty.
func pickReportableListenAddress(addresses []string) string {
	for _, addr := range addresses {
		if strings.HasPrefix(addr, "/ip4/127.") || strings.HasPrefix(addr, "/ip6/::1/") {
			continue
		}
		if strings.HasPrefix(addr, "/ip4/172.17.") {
			continue
		}
		return addr
	}
	if len(addresses) > 0 {
		return addresses[0]
	}
	return ""
}

func getWorkingDir(log logger.Logger) string {
	workingDir, err := os.Getwd()
	if err != nil {
		log.LogIfError(err)
		workingDir = ""
	}

	log.Trace("working directory", "path", workingDir)

	return workingDir
}

func checkExpectedPeerCount(p2pConfig config.P2PConfig) error {
	maxExpectedPeerCount := p2pConfig.Node.MaximumExpectedPeerCount

	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return fmt.Errorf("%w while getting RLimits", err)
	}

	log.Info("file limits",
		"current", rLimit.Cur,
		"max", rLimit.Max,
		"expected", maxExpectedPeerCount,
	)

	if maxExpectedPeerCount > rLimit.Cur {
		return fmt.Errorf("provided maxExpectedPeerCount is less than the current OS configured value")
	}

	return nil
}

func startRestServices(ctx *cli.Context, marshalizer marshal.Marshalizer, messenger p2p.Messenger, startTime time.Time) {
	restAPIInterface := ctx.GlobalString(restAPIInterfaceFlag.Name)
	if restAPIInterface != facade.DefaultRestPortOff {
		go startGinServer(restAPIInterface, marshalizer, messenger, startTime)
	} else {
		log.Info("rest api is disabled")
	}
}

func startGinServer(restAPIInterface string, marshalizer marshal.Marshalizer, messenger p2p.Messenger, startTime time.Time) {
	err := api.Start(restAPIInterface, marshalizer, messenger, appVersion, startTime)
	if err != nil {
		log.LogIfError(err)
	}
}
