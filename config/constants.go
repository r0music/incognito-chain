package config

//Env variables key
const (
	NetworkKey        = "INCOGNITO_NETWORK_KEY"
	NetworkVersionKey = "INCOGNITO_NETWORK_VERSION_KEY"
	ConfigFileKey     = "INCOGNITO_CONFIG_FILE_KEY"
	ConfigDirKey      = "INCOGNITO_CONFIG_DIR_KEY"
	ConfigFileTypeKey = "INCOGNITO_CONFIG_FILE_TYPE_KEY"
	ConfigModeKey     = "INCOGNITO_CONFIG_MODE_KEY"
	ParamFileKey      = "INCOGNITO_PARAM_FILE_KEY"
)

// default config
const (
	DefaultDataDirname                 = "data"
	DefaultDatabaseDirname             = "block"
	DefaultMempoolDirname              = "mempool"
	DefaultLogLevel                    = "info"
	DefaultLogDirname                  = "logs"
	DefaultLogFilename                 = "log.log"
	DefaultMaxPeers                    = 1000
	DefaultMaxPeersSameShard           = 300
	DefaultMaxPeersOtherShard          = 600
	DefaultMaxPeersOther               = 300
	DefaultMaxPeersNoShard             = 200
	DefaultMaxPeersBeacon              = 500
	DefaultMaxRPCClients               = 500
	DefaultRPCLimitRequestPerDay       = 0 // 0: unlimited
	DefaultRPCLimitErrorRequestPerHour = 0 // 0: unlimited
	DefaultMaxRPCWsClients             = 200
	DefaultMetricUrl                   = ""
	SampleConfigFilename               = "sample-config.conf"
	DefaultDisableRpcTLS               = true
	DefaultFastStartup                 = true
	// DefaultNodeMode                    = common.NodeModeRelay
	DefaultEnableMining = true
	DefaultTxPoolTTL    = uint(15 * 60) // 15 minutes
	DefaultTxPoolMaxTx  = uint64(100000)
	DefaultLimitFee     = uint64(1) // 1 nano PRV = 10^-9 PRV
	//DefaultLimitFee = uint64(100000) // 100000 nano PRV = 100000 * 10^-9 PRV
	// For wallet
	DefaultWalletName     = "wallet"
	DefaultPersistMempool = false
	DefaultBtcClient      = 0
	DefaultBtcClientPort  = "8332"
	DefaultNetwork        = LocalNetwork
	DefaultConfigDir      = "config"
	DefaultConfigFile     = "config"
	DefaultConfigFileType = "yaml"
	DefaultParamFile      = "param"
)

const (
	LocalNetwork    = "local"
	TestNetNetwork  = "testnet"
	MainnetNetwork  = "mainnet"
	TestNetVersion1 = "1"
	TestNetVersion2 = "2"
	DefaultPort     = "9444"
	DefaultRPCPort  = "9344"
	DefaultWSPort   = "19444"
	FlagConfigMode  = "flag"
	FileConfigMode  = "file"
)

var (
	defaultDataDir     = DefaultDataDirname
	defaultRPCKeyFile  = "rpc.key"
	defaultRPCCertFile = "rpc.cert"
	defaultLogDir      = DefaultLogDirname
)
