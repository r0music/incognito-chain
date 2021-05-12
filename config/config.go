package config

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/spf13/viper"
)

var c *config

func Config() *config {
	return c
}

type config struct {
	//Basic config
	DataDir     string `yaml:"data_dir" short:"d" long:"datadir" description:"Directory to store data"`
	DatabaseDir string `yaml:"database_dir" short:"d" long:"datapre" description:"Database dir"`
	MempoolDir  string `yaml:"mempool_dir" short:"m" long:"mempooldir" description:"Mempool Directory"`
	LogDir      string `yaml:"log_dir" short:"l" long:"logdir" description:"Directory to log output."`
	LogLevel    string `yaml:"log_level" long:"loglevel" description:"Logging level for all subsystems {trace, debug, info, warn, error, critical} -- You may also specify <subsystem>=<level>,<subsystem2>=<level>,... to set the log level for individual subsystems -- Use show to list available subsystems"`
	LogFileName string `yaml:"log_file_name" long:"logfilename" description:"log file name"`

	//Peer Config
	AddPeers             []string `yaml:"add_peers" short:"a" long:"addpeer" description:"Add a peer to connect with at startup"`
	ConnectPeers         []string `yaml:"connect_peers" short:"c" long:"connect" description:"Connect only to the specified peers at startup"`
	DisableListen        bool     `yaml:"disable_listen" long:"nolisten" description:"Disable listening for incoming connections -- NOTE: Listening is automatically disabled if the --connect or --proxy options are used without also specifying listen interfaces via --listen"`
	Listener             string   `yaml:"listener" long:"listen" description:"Add an interface/port to listen for connections (default all interfaces port: 9333, testnet: 9444)"`
	MaxPeers             int      `yaml:"max_peers" long:"maxpeers" description:"Max number of inbound and outbound peers"`
	MaxOutPeers          int      `yaml:"max_out_peers" long:"maxoutpeers" description:"Max number of outbound peers"`
	MaxInPeers           int      `yaml:"max_in_peers" long:"maxinpeers" description:"Max number of inbound peers"`
	DiscoverPeers        bool     `yaml:"discover_peers" long:"discoverpeers" description:"Enable discover peers"`
	DiscoverPeersAddress string   `yaml:"discover_peers_address" long:"discoverpeersaddress" description:"Url to connect discover peers server"`
	MaxPeersSameShard    int      `yaml:"max_peers_same_shard" long:"maxpeersameshard" description:"Max peers in same shard for connection"`
	MaxPeersOtherShard   int      `yaml:"max_pmax_peers_other_shard" long:"maxpeerothershard" description:"Max peers in other shard for connection"`
	MaxPeersOther        int      `yaml:"max_peers_other" long:"maxpeerother" description:"Max peers in other for connection"`
	MaxPeersNoShard      int      `yaml:"max_peers_no_shard" long:"maxpeernoshard" description:"Max peers in no shard for connection"`
	MaxPeersBeacon       int      `yaml:"max_peers_beacon" long:"maxpeerbeacon" description:"Max peers in beacon for connection"`

	//Rpc Config
	ExternalAddress             string   `yaml:"external_address" long:"externaladdress" description:"External address"`
	RPCDisableAuth              bool     `yaml:"rpc_disable_auth" long:"norpcauth" description:"Disable RPC authorization by username/password"`
	RPCUser                     string   `yaml:"rpc_user" short:"u" long:"rpcuser" description:"Username for RPC connections"`
	RPCPass                     string   `yaml:"rpc_pass" short:"P" long:"rpcpass" default-mask:"-" description:"Password for RPC connections"`
	RPCLimitUser                string   `yaml:"rpc_limit_user" long:"rpclimituser" description:"Username for limited RPC connections"`
	RPCLimitPass                string   `yaml:"rpc_limit_pass" long:"rpclimitpass" default-mask:"-" description:"Password for limited RPC connections"`
	RPCListeners                []string `yaml:"rpc_listeners" long:"rpclisten" description:"Add an interface/port to listen for RPC connections (default port: 9334, testnet: 9334)"`
	RPCWSListeners              []string `yaml:"rpc_ws_listeners" long:"rpcwslisten" description:"Add an interface/port to listen for RPC Websocket connections (default port: 19334, testnet: 19334)"`
	RPCCert                     string   `yaml:"rpc_cert" long:"rpccert" description:"File containing the certificate file"`
	RPCKey                      string   `yaml:"rpc_key" long:"rpckey" description:"File containing the certificate key"`
	RPCLimitRequestPerDay       int      `yaml:"rpc_limit_request_per_day" long:"rpclimitrequestperday" description:"Max request per day by remote address"`
	RPCLimitRequestErrorPerHour int      `yaml:"rpc_limit_request_error_per_hour" long:"rpclimitrequesterrorperhour" description:"Max request error per hour by remote address"`
	RPCMaxClients               int      `yaml:"rpc_max_clients" long:"rpcmaxclients" description:"Max number of RPC clients for standard connections"`
	RPCMaxWSClients             int      `yaml:"rpc_max_ws_clients" long:"rpcmaxwsclients" description:"Max number of RPC clients for standard connections"`
	RPCQuirks                   bool     `yaml:"rpc_quirks" long:"rpcquirks" description:"Mirror some JSON-RPC quirks of coin Core -- NOTE: Discouraged unless interoperability issues need to be worked around"`
	DisableRPC                  bool     `yaml:"disable_rpc" long:"norpc" description:"Disable built-in RPC server -- NOTE: The RPC server is disabled by default if no rpcuser/rpcpass or rpclimituser/rpclimitpass is specified"`
	DisableTLS                  bool     `yaml:"disable_tls" long:"notls" description:"Disable TLS for the RPC server -- NOTE: This is only allowed if the RPC server is bound to localhost"`
	Proxy                       string   `yaml:"proxy" long:"proxy" description:"Connect via SOCKS5 proxy (eg. 127.0.0.1:9050)"`

	//Network Config
	IsLocal        bool `long:"local" description:"Use the local network"`
	IsTestNet      bool `long:"istestnet" description:"Use the testnet network"`
	IsMainNet      bool `long:"ismainnet" description:"Use the mainnet network"`
	TestNetVersion int  `long:"testnetversion" description:"Use the test network"`

	RelayShards string `yaml:"relay_shards" long:"relayshards" description:"set relay shards of this node when in 'relay' mode if noderole is auto then it only sync shard data when user is a shard producer/validator"`
	// For Wallet
	EnableWallet     bool   `yaml:"enable_wallet" long:"enablewallet" description:"Enable wallet"`
	WalletName       string `yaml:"wallet_name" long:"wallet" description:"Wallet Database Name file, default is 'wallet'"`
	WalletPassphrase string `yaml:"wallet_passphrase" long:"walletpassphrase" description:"Wallet passphrase"`
	WalletAutoInit   bool   `yaml:"wallet_auto_init" long:"walletautoinit" description:"Init wallet automatically if not exist"`
	WalletShardID    int    `yaml:"wallet_shard_id" long:"walletshardid" description:"ShardID which wallet use to create account"`

	//Fast start up config
	FastStartup bool `yaml:"fast_start_up" long:"faststartup" description:"Load existed shard/chain dependencies instead of rebuild from block data"`

	//Txpool config
	TxPoolTTL   uint   `yaml:"tx_pool_ttl" long:"txpoolttl" description:"Set Time To Live (TTL) Value for transaction that enter pool"`
	TxPoolMaxTx uint64 `yaml:"tx_pool_max_tx" long:"txpoolmaxtx" description:"Set Maximum number of transaction in pool"`
	LimitFee    uint64 `yaml:"limit_fee" long:"limitfee" description:"Limited fee for tx(per Kb data), default is 0.00 PRV"`

	//Mempool config
	IsLoadFromMempool bool `yaml:"is_load_from_mem_pool" long:"loadmempool" description:"Load transactions from Mempool database"`
	IsPersistMempool  bool `yaml:"is_persist_mem_pool" long:"persistmempool" description:"Persistence transaction in memepool database"`

	//Mining config
	EnableMining bool   `yaml:"enable_mining" long:"mining" description:"enable mining"`
	MiningKeys   string `yaml:"mining_keys" long:"miningkeys" description:"keys used for different consensus algorigthm"`
	PrivateKey   string `yaml:"private_key" long:"privatekey" description:"your wallet privatekey"`
	Accelerator  bool   `yaml:"accelerator" long:"accelerator" description:"Relay Node Configuration For Consensus"`

	// Highway
	Libp2pPrivateKey string `yaml:"p2p_private_key" long:"libp2pprivatekey" description:"Private key used to create node's PeerID, empty to generate random key each run"`

	//backup
	PreloadAddress string `yaml:"preload_address" long:"preloadaddress" description:"Endpoint of fullnode to download backup database"`
	ForceBackup    bool   `yaml:"force_backup" long:"forcebackup" description:"Force node to backup"`
}

// normalizeAddresses returns a new slice with all the passed peer addresses
// normalized with the given default port, and all duplicates removed.
func normalizeAddresses(addrs []string, defaultPort string) []string {
	for i, addr := range addrs {
		addrs[i] = normalizeAddress(addr, defaultPort)
	}

	return removeDuplicateAddresses(addrs)
}

// normalizeAddress returns addr with the passed default port appended if
// there is not already a port specified.
func normalizeAddress(addr, defaultPort string) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}

// removeDuplicateAddresses returns a new slice with all duplicate entries in
// addrs removed.
func removeDuplicateAddresses(addrs []string) []string {
	result := make([]string, 0, len(addrs))
	seen := map[string]struct{}{}
	for _, val := range addrs {
		if _, ok := seen[val]; !ok {
			result = append(result, val)
			seen[val] = struct{}{}
		}
	}
	return result
}

func (c *config) loadNetwork() string {
	res := ""
	switch common.GetEnv(NetworkKey, LocalNetwork) {
	case LocalNetwork:
		res = LocalNetwork
		c.IsLocal = true
	case TestNetNetwork:
		res = TestNetNetwork
		c.IsTestNet = true
		testnetVersion := common.GetEnv(NetworkVersionKey, TestNetVersion1)
		version, err := strconv.Atoi(testnetVersion)
		if err != nil {
			panic(err)
		}
		res += testnetVersion
		c.TestNetVersion = version
	case MainnetNetwork:
		res = MainnetNetwork
		c.IsMainNet = true
	}
	return res
}

func (c *config) Network() string {
	res := common.GetEnv(NetworkKey, LocalNetwork)
	if res == TestNetNetwork {
		res += common.GetEnv(NetworkVersionKey, TestNetVersion1)
	}
	return res
}

func (c *config) verify(network string) {
	// Multiple networks can't be selected simultaneously.
	numNets := 0
	if c.IsLocal {
		numNets++
	}
	if c.IsTestNet {
		numNets++
	}
	if c.IsMainNet {
		numNets++
	}

	if numNets > 1 {
		log.Println("The network can not be used together -- choose one of them")
		os.Exit(common.ExitCodeUnknow)
	}

	// Append the network type to the data directory so it is "namespaced"
	// per network.  In addition to the block database, there are other
	// pieces of data that are saved to disk such as address manager state.
	// All data is specific to a network, so namespacing the data directory
	// means each individual piece of serialized data does not have to
	// worry about changing names per network and such.
	c.DataDir = filepath.Join(c.DataDir, network)

	// Append the network type to the log directory so it is "namespaced"
	// per network in the same fashion as the data directory.
	c.LogDir = filepath.Join(c.LogDir, network)
	c.LogFileName = filepath.Join(c.LogDir, c.LogFileName)

	/*// Initialize log rotation.  After log rotation has been initialized, the*/
	//// logger variables may be used.
	//initLogRotator(filepath.Join(cfg.LogDir, DefaultLogFilename))

	//// Parse, validate, and set debug log level(s).
	//if err := parseAndSetDebugLevels(cfg.LogLevel); err != nil {
	//err := fmt.Errorf("%s: %v", funcName, err.Error())
	//fmt.Fprintln(os.Stderr, err)
	//fmt.Fprintln(os.Stderr, usageMessage)
	//return nil, nil, err
	/*}*/

	// --addPeer and --connect do not mix.
	if len(c.AddPeers) > 0 && len(c.ConnectPeers) > 0 {
		str := "%s: the --addpeer and --connect options can not be mixed"
		fmt.Fprintln(os.Stderr, errors.New(str))
		panic(str)
	}

	// --proxy or --connect without --listen disables listening.
	if (c.Proxy != common.EmptyString || len(c.ConnectPeers) > 0) &&
		len(c.Listener) == 0 {
		c.DisableListen = true
	}

	// Add the default listener if none were specified. The default
	// listener is all addresses on the listen port for the network
	// we are to connect to.
	if len(c.Listener) == 0 {
		c.Listener = net.JoinHostPort("", DefaultPort)
	}

	if !c.RPCDisableAuth {
		if c.RPCUser == c.RPCLimitUser && c.RPCUser != "" {
			str := "%s: --rpcuser and --rpclimituser must not specify the same username"
			fmt.Fprintln(os.Stderr, errors.New(str))
			panic(str)
		}

		// Check to make sure limited and admin users don't have the same password
		if c.RPCPass == c.RPCLimitPass && c.RPCPass != "" {
			str := "%s: --rpcpass and --rpclimitpass must not specify the same password"
			fmt.Fprintln(os.Stderr, errors.New(str))
			panic(str)
		}

		// The RPC server is disabled if no username or password is provided.
		if (c.RPCUser == "" || c.RPCPass == "") &&
			(c.RPCLimitUser == "" || c.RPCLimitPass == "") {
			log.Println("The RPC server is disabled if no username or password is provided.")
			c.DisableRPC = true
		}
	}

	if c.DisableRPC {
		log.Println("RPC service is disabled")
	}

	// Default RPC to listen on localhost only.
	if !c.DisableRPC && len(c.RPCListeners) == 0 {
		addrs, err := net.LookupHost("0.0.0.0")
		if err != nil {
			panic(err)
		}
		// Get address from env
		externalAddress := os.Getenv("EXTERNAL_ADDRESS")
		if externalAddress != "" {
			host, _, err := net.SplitHostPort(externalAddress)
			if err == nil && host != "" {
				addrs = []string{host}
			}
		}
		//Logger.log.Info(externalAddress, addrs)
		c.RPCListeners = make([]string, 0, len(addrs))
		for _, addr := range addrs {
			addr = net.JoinHostPort(addr, DefaultRPCPort)
			c.RPCListeners = append(c.RPCListeners, addr)
		}
	}

	// Default RPC Ws to listen on localhost only.
	if !c.DisableRPC && len(c.RPCWSListeners) == 0 {
		addrs, err := net.LookupHost("0.0.0.0")
		if err != nil {
			panic(err)
		}
		// Get address from env
		externalAddress := os.Getenv("EXTERNAL_ADDRESS")
		if externalAddress != "" {
			host, _, err := net.SplitHostPort(externalAddress)
			if err == nil && host != "" {
				addrs = []string{host}
			}
		}
		//Logger.log.Info(externalAddress, addrs)
		c.RPCWSListeners = make([]string, 0, len(addrs))
		for _, addr := range addrs {
			addr = net.JoinHostPort(addr, DefaultWSPort)
			c.RPCWSListeners = append(c.RPCWSListeners, addr)
		}
	}

	// Add default port to all listener addresses if needed and remove
	// duplicate addresses.
	c.Listener = normalizeAddress(c.Listener, DefaultPort)

	// Add default port to all rpc listener addresses if needed and remove
	// duplicate addresses.
	c.RPCListeners = normalizeAddresses(c.RPCListeners, DefaultRPCPort)
	// Add default port to all rpc listener addresses if needed and remove
	// duplicate addresses.
	c.RPCWSListeners = normalizeAddresses(c.RPCWSListeners, DefaultWSPort)

	// Only allow TLS to be disabled if the RPC is bound to localhost
	// addresses.
	if !c.DisableRPC && c.DisableTLS {
		allowedTLSListeners := map[string]struct{}{
			"localhost": {},
			"127.0.0.1": {},
			"::1":       {},
			"0.0.0.0":   {},
		}

		for _, addr := range c.RPCListeners {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				str := "%s: RPC listen interface '%s' is " +
					"invalid: %v"
				fmt.Fprintln(os.Stderr, errors.New(str))
				panic(str)
			}
			if _, ok := allowedTLSListeners[host]; !ok {
				str := "%s: the --notls option may not be used when binding RPC to non localhost addresses: %s"
				fmt.Fprintln(os.Stderr, errors.New(str))
				panic(str)
			}
		}
		for _, addr := range c.RPCWSListeners {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				str := "%s: WS RPC listen interface '%s' is " +
					"invalid: %v"
				fmt.Fprintln(os.Stderr, errors.New(str))
				panic(str)
			}
			if _, ok := allowedTLSListeners[host]; !ok {
				str := "%s: the --notls option may not be used when binding WS RPC to non localhost addresses: %s"
				fmt.Fprintln(os.Stderr, errors.New(str))
				panic(str)
			}
		}
	}

	if c.DiscoverPeers {
		if c.DiscoverPeersAddress == "" {
			err := errors.New("discover peers server is empty")
			panic(err)
		}
	}
}

func LoadConfig() *config {
	c = &config{
		LogLevel:                    DefaultLogLevel,
		MaxOutPeers:                 DefaultMaxPeers,
		MaxInPeers:                  DefaultMaxPeers,
		MaxPeers:                    DefaultMaxPeers,
		MaxPeersSameShard:           DefaultMaxPeersSameShard,
		MaxPeersOtherShard:          DefaultMaxPeersOtherShard,
		MaxPeersOther:               DefaultMaxPeersOther,
		MaxPeersNoShard:             DefaultMaxPeersNoShard,
		MaxPeersBeacon:              DefaultMaxPeersBeacon,
		RPCMaxClients:               DefaultMaxRPCClients,
		RPCMaxWSClients:             DefaultMaxRPCWsClients,
		RPCLimitRequestPerDay:       DefaultRPCLimitRequestPerDay,
		RPCLimitRequestErrorPerHour: DefaultRPCLimitErrorRequestPerHour,
		DataDir:                     defaultDataDir,
		DatabaseDir:                 DefaultDatabaseDirname,
		MempoolDir:                  DefaultMempoolDirname,
		LogDir:                      defaultLogDir,
		RPCKey:                      defaultRPCKeyFile,
		RPCCert:                     defaultRPCCertFile,
		WalletShardID:               -1,
		WalletName:                  DefaultWalletName,
		DisableTLS:                  DefaultDisableRpcTLS,
		DisableRPC:                  false,
		RPCDisableAuth:              false,
		DiscoverPeers:               true,
		DiscoverPeersAddress:        "127.0.0.1:9330", //"35.230.8.182:9339",
		MiningKeys:                  common.EmptyString,
		PrivateKey:                  common.EmptyString,
		FastStartup:                 DefaultFastStartup,
		TxPoolTTL:                   DefaultTxPoolTTL,
		TxPoolMaxTx:                 DefaultTxPoolMaxTx,
		IsPersistMempool:            DefaultPersistMempool,
		LimitFee:                    DefaultLimitFee,
		EnableMining:                DefaultEnableMining,
	}

	//get network
	network := c.loadNetwork()
	//load config from file
	c.loadConfig(network)
	//verify config
	c.verify(network)

	return c
}

func (c *config) loadConfig(network string) {
	//read config from file
	viper.SetConfigName(common.GetEnv(ConfigFileKey, DefaultConfigFile))         // name of config file (without extension)
	viper.SetConfigType(common.GetEnv(ConfigFileTypeKey, DefaultConfigFileType)) // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(
		filepath.Join(
			common.GetEnv(ConfigDirKey, DefaultConfigDir), network,
		) + "/",
	) // optionally look for config in the working directory
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			panic(err)
		}
	}
}
