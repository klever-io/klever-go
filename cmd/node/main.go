package main

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/denisbrodbeck/machineid"
	logger "github.com/klever-io/klever-go-logger"
	"github.com/klever-io/klever-go/common/facade"
	"github.com/klever-io/klever-go/core"
	"github.com/urfave/cli"
)

const (
	defaultStatsPath             = "stats"
	defaultLogsPath              = "logs"
	logFilePrefix                = "klever-go"
	metachainName                = "metachain"
	secondsToWaitForP2PBootstrap = 5
	maxTimeToClose               = 10 * time.Second
	maxMachineIDLen              = 10
	defaultSwaggerURL            = "node.testnet.klever.finance/"
	emptyNamePrefix              = "klever"
)

var (
	nodeHelpTemplate = `NAME:
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
	filePathPlaceholder = "[path]"
	// genesisFile defines a flag for the path of the bootstrapping file.
	genesisFile = cli.StringFlag{
		Name: "genesis-file",
		Usage: "The `" + filePathPlaceholder + "` for the genesis file. This JSON file contains initial data to " +
			"bootstrap from, such as initial balances for accounts.",
		Value: "./config/node/genesis.json",
	}
	// nodesFile defines a flag for the path of the initial nodes file.
	nodesFile = cli.StringFlag{
		Name: "nodes-setup-file",
		Usage: "The `" + filePathPlaceholder + "` for the nodes setup. This JSON file contains initial nodes info, " +
			"such as consensus group size, slot duration, validators public keys and so on.",
		Value: "./config/node/nodesSetup.json",
	}
	// configurationFile defines a flag for the path to the main yaml configuration file
	configurationFile = cli.StringFlag{
		Name: "config",
		Usage: "The `" + filePathPlaceholder + "` for the main configuration file. This YAML file contain the main " +
			"configurations such as storage setups, epoch duration and so on.",
		Value: "./config/node/config.yaml",
	}
	// configurationAPIFile defines a flag for the path to the api routes yaml configuration file
	configurationAPIFile = cli.StringFlag{
		Name: "config-api",
		Usage: "The `" + filePathPlaceholder + "` for the api configuration file. This YAML file contains " +
			"all available routes for Rest API and options to enable or disable them.",
		Value: "./config/node/api.yaml",
	}
	// configurationEnableEpochsFile defines a flag for the path to the enable epochs yaml configuration file
	configurationEnableEpochsFile = cli.StringFlag{
		Name: "config-epochs",
		Usage: "The `" + filePathPlaceholder + "` for the enable epochs configuration file. This YAML file contains " +
			"all tags initialization with correspondent epoch to be enabled.",
		Value: "./config/node/enableEpochs.yaml",
	}
	// configurationGasScheduleFile defines a flag for the path to the gas schedule yaml configuration file
	configurationGasScheduleFile = cli.StringFlag{
		Name: "config-gas-schedule",
		Usage: "The `" + filePathPlaceholder + "` for the gas schedule configuration file. This YAML file contains " +
			"all the gas costs.",
		Value: "./config/node/gasSchedule.yaml",
	}
	// externalConfigFile defines a flag for the path to the external yaml configuration file
	externalConfigFile = cli.StringFlag{
		Name: "config-external",
		Usage: "The `" + filePathPlaceholder + "` for the external configuration file. This YAML file contains" +
			" external configurations such as ElasticSearch's URL and login information",
		Value: "./config/node/external.yaml",
	}
	// port defines a flag for setting the port on which the node will listen for connections
	port = cli.StringFlag{
		Name: "port",
		Usage: "The `[p2p port]` number on which the application will start. Can use single values such as " +
			"`0, 10230, 15670` or range of ports such as `5000-10000`",
		Value: "0",
	}
	// profileMode defines a flag for profiling the binary
	// If enabled, it will open the pprof routes over the default gin rest webserver.
	// There are several routes that will be available for profiling (profiling can be analyzed with: go tool pprof):
	//  /debug/pprof/ (can be accessed in the browser, will list the available options)
	//  /debug/pprof/goroutine
	//  /debug/pprof/heap
	//  /debug/pprof/threadcreate
	//  /debug/pprof/block
	//  /debug/pprof/mutex
	//  /debug/pprof/profile (CPU profile)
	//  /debug/pprof/trace?seconds=5 (CPU trace) -> being a trace, can be analyzed with: go tool trace
	// Usage: go tool pprof http(s)://ip.of.the.server/debug/pprof/xxxxx
	profileMode = cli.BoolFlag{
		Name: "profile-mode",
		Usage: "Boolean option for enabling the profiling mode. If set, the /debug/pprof routes will be available " +
			"on the node for profiling the application.",
	}
	// useHealthService is used to enable the health service
	useHealthService = cli.BoolFlag{
		Name:  "use-health-service",
		Usage: "Boolean option for enabling the health service.",
	}
	// validatorKeyIndex defines a flag that specifies the 0-th based index of the private key to be used from validatorKey.pem file
	validatorKeyIndex = cli.IntFlag{
		Name:  "sk-index",
		Usage: "The index in the PEM file of the private key to be used by the node.",
		Value: 0,
	}
	// gopsEn used to enable diagnosis of running go processes
	gopsEn = cli.BoolFlag{
		Name:  "gops-enable",
		Usage: "Boolean option for enabling gops over the process. If set, stack can be viewed by calling 'gops stack <pid>'.",
	}
	// storageCleanup defines a flag for choosing the option of starting the node from scratch. If it is not set (false)
	// it starts from the last state stored on disk
	storageCleanup = cli.BoolFlag{
		Name: "storage-cleanup",
		Usage: "Boolean option for starting the node with clean storage. If set, the Node will empty its storage " +
			"before starting, otherwise it will start from the last state stored on disk..",
	}

	// restAPIInterface defines a flag for the interface on which the rest API will try to bind with
	restAPIInterface = cli.StringFlag{
		Name: "rest-api-interface",
		Usage: "The interface `address and port` to which the REST API will attempt to bind. " +
			"To bind to all available interfaces, set this flag to :8080",
		Value: facade.DefaultRestInterface,
	}

	// p2pSeed defines a flag to be used as a seed when generating P2P credentials. Useful for seed nodes.
	p2pSeed = cli.StringFlag{
		Name:  "p2p-seed",
		Usage: "P2P seed will be used when generating credentials for p2p component. Can be any string.",
		Value: "",
	}

	// restAPIDebug defines a flag for starting the rest API engine in debug mode
	restAPIDebug = cli.BoolFlag{
		Name:  "rest-api-debug",
		Usage: "Boolean option for starting the Rest API in debug mode.",
	}

	// nodeDisplayName defines the friendly name used by a node in the public monitoring tools. If set, will override
	// the NodeDisplayName from config
	nodeDisplayName = cli.StringFlag{
		Name: "display-name",
		Usage: "The user-friendly name for the node, appearing in the public monitoring tools. Will override the " +
			"name set in the preferences YAML file.",
		Value: "",
	}
	// identityFlagName defines the keybase's identity. If set, will override the identity config
	identityFlagName = cli.StringFlag{
		Name:  "keybase-identity",
		Usage: "The keybase's identity. If set, will override the one set in the preferences YAML file.",
		Value: "",
	}

	//useLogView is used when connector interface is not needed.
	useLogView = cli.BoolFlag{
		Name: "use-log-view",
		Usage: "Boolean option for enabling the simple node's interface. If set, the node will not enable the " +
			"user-friendly terminal view of the node.",
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

	swaggerUrl = cli.StringFlag{
		Name:  "swagger-url",
		Value: defaultSwaggerURL,
		Usage: "String option for change swagger base URL",
	}

	bugsnagKey = cli.StringFlag{
		Name:  "bugsnag-key",
		Usage: "String option to use bugsnag",
	}

	bugsnagStage = cli.StringFlag{
		Name:  "bugsnag-stage",
		Usage: "ReleaseStage to bugsnag",
		Value: "production",
	}

	//logFile is used when the log output needs to be logged in a file
	logSaveFile = cli.BoolFlag{
		Name:  "log-save",
		Usage: "Boolean option for enabling log saving. If set, it will automatically save all the logs into a file.",
	}
	//logWithCorrelation is used to enable log correlation elements
	logWithCorrelation = cli.BoolFlag{
		Name:  "log-correlation",
		Usage: "Boolean option for enabling log correlation elements.",
	}
	//logWithLoggerName is used to enable log correlation elements
	logWithLoggerName = cli.BoolFlag{
		Name:  "log-logger-name",
		Usage: "Boolean option for logger name in the logs.",
	}
	// disableAnsiColor defines if the logger subsystem should prevent displaying ANSI colors
	disableAnsiColor = cli.BoolFlag{
		Name:  "disable-ansi-color",
		Usage: "Boolean option for disabling ANSI colors in the logging system.",
	}
	// bootstrapSlotIndex defines a flag that specifies the slot index from which node should bootstrap from storage
	bootstrapSlotIndex = cli.Uint64Flag{
		Name:  "bootstrap-slot-index",
		Usage: "This flag specifies the slot `index` from which node should bootstrap from storage.",
		Value: math.MaxUint64,
	}

	// validatorKeyPemFile defines a flag for the path to the validator key used in block signing
	validatorKeyPemFile = cli.StringFlag{
		Name:  "validator-key-pem-file",
		Usage: "The `filepath` for the PEM file which contains the secret keys for the validator key.",
		Value: "./config/node/validatorKey.pem",
	}

	// workingDirectory defines a flag for the path for the working directory.
	workingDirectory = cli.StringFlag{
		Name:  "working-directory",
		Usage: "This flag specifies the `directory` where the node will store databases, logs and statistics.",
		Value: "",
	}

	keepOldEpochsData = cli.BoolFlag{
		Name: "keep-old-epochs-data",
		Usage: "Boolean option for enabling a node to keep old epochs data. If set, the node won't remove any database " +
			"and will have a full history over epochs.",
	}

	// importDbDirectory defines a flag for the optional import DB directory on which the node will re-check the blockchain against
	importDbDirectory = cli.StringFlag{
		Name: "import-db",
		Usage: "This flag, if set, will make the node start the import process using the provided data path. Will re-check" +
			"and re-process everything",
		Value: "",
	}
	// importDbNoSigCheck defines a flag for the optional import DB no signature check option
	importDbNoSigCheck = cli.BoolFlag{
		Name:  "import-db-no-sig-check",
		Usage: "This flag, if set, will cause the signature checks on headers to be skipped. Can be used only if the import-db was previously set",
	}

	// importDbStartInEpoch defines a flag for an optional flag that can specify the start in epoch value when executing the import-db process
	importDbStartInEpoch = cli.Uint64Flag{
		Name:  "import-db-start-epoch",
		Value: 0,
		Usage: "This flag will specify the start in epoch value in import-db process",
	}

	// redundancyLevel defines a flag that specifies the level of redundancy used by the current instance for the node (-1 = disabled, 0 = main instance (default), 1 = first backup, 2 = second backup, etc.)
	redundancyLevel = cli.Int64Flag{
		Name:  "redundancy-level",
		Usage: "This flag specifies the level of redundancy used by the current instance for the node (-1 = disabled, 0 = main instance (default), 1 = first backup, 2 = second backup, etc.)",
		Value: 0,
	}

	startWithInSync = cli.BoolFlag{
		Name:  "start-in-sync",
		Usage: "Boolean option for enabling a node to start as `InSync` state.Used for network bootstrap.",
	}

	startInEpoch = cli.BoolFlag{
		Name: "start-in-epoch",
		Usage: "Boolean option for enabling a node the fast bootstrap mechanism from the network." +
			"Should be enabled if data is not available in local disk.",
	}

	syncUntil = cli.Uint64Flag{
		Name:  "sync-until",
		Usage: "This flag specifies to run until stopping the node. If 0 the node runs normally. for backtest reasons only",
		Value: 0,
	}

	WSConnectionUrl = cli.StringFlag{
		Name:  "ws-conn-url",
		Usage: "String option for enabling the AWS post connection websocket. If set together with ws-conn-api-key, the websocket will send posts to AWS WS",
	}

	WSConnectionApiKey = cli.StringFlag{
		Name:  "ws-conn-api-key",
		Usage: "String option for the WS post connection api key. If set together with ws-conn-url, the websocket will send posts to AWS WS",
	}
)

// appVersion should be populated at build time using ldflags
// Usage examples:
// linux/mac:
//
//	go build -i -v -ldflags="-X main.appVersion=$(git describe --tags --long --dirty)"
//
// windows:
//
//	for /f %i in ('git describe --tags --long --dirty') do set VERS=%i
//	go build -i -v -ldflags="-X main.appVersion=%VERS%"
var appVersion = core.UnVersionedAppString

// @title KleverChain Node
// @version 1.0
// @description This is a sample of the KleverChain Node Server.
func main() {
	_ = logger.SetDisplayByteSlice(logger.ToHexShort)
	log := logger.GetOrCreate("main")

	app := cli.NewApp()
	cli.AppHelpTemplate = nodeHelpTemplate
	app.Name = "Klever Node CLI App"
	machineID, err := machineid.ProtectedID(app.Name)
	if err != nil {
		log.Warn("error fetching machine id", "error", err)
		machineID = "unknown"
	}
	if len(machineID) > maxMachineIDLen {
		machineID = machineID[:maxMachineIDLen]
	}

	app.Version = fmt.Sprintf("%s/%s/%s-%s/%s", appVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH, machineID)
	app.Usage = "This is the entry point for starting a new Klever node - the app will start after the genesis timestamp"
	app.Flags = []cli.Flag{
		genesisFile,
		nodesFile,
		configurationFile,
		configurationAPIFile,
		configurationEnableEpochsFile,
		configurationGasScheduleFile,
		externalConfigFile,
		validatorKeyIndex,
		validatorKeyPemFile,
		port,
		profileMode,
		useHealthService,
		storageCleanup,
		gopsEn,
		nodeDisplayName,
		identityFlagName,
		restAPIInterface,
		p2pSeed,
		restAPIDebug,
		disableAnsiColor,
		bootstrapSlotIndex,
		logLevel,
		logSaveFile,
		logWithCorrelation,
		logWithLoggerName,
		useLogView,
		workingDirectory,
		keepOldEpochsData,
		redundancyLevel,
		startInEpoch,
		swaggerUrl,
		importDbDirectory,
		importDbNoSigCheck,
		importDbStartInEpoch,
		bugsnagKey,
		bugsnagStage,
		syncUntil,
		startWithInSync,
		WSConnectionApiKey,
		WSConnectionUrl,
	}

	app.Authors = []cli.Author{
		{
			Name:  "The Klever Team",
			Email: "contact@klever.io",
		},
	}

	app.Action = func(c *cli.Context) error {
		return startNode(c, log, app.Version)
	}

	err = app.Run(os.Args)
	if err != nil {
		_ = bugsnag.Notify(err)
		log.Error(err.Error())
		os.Exit(1)
	}
}
