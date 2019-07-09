package connmanager

import (
	"fmt"
	"math"
	"net"
	"net/rpc"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/incognitochain/incognito-chain/bootnode/server"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/peer"
	"github.com/incognitochain/incognito-chain/wire"
	libpeer "github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

type ConnManager struct {
	start int32
	stop  int32
	// Discover Peers
	//discoveredPeers     map[string]*DiscoverPeerInfo
	discoverPeerAddress string
	// channel
	cQuit            chan struct{}
	cDiscoveredPeers chan struct{}

	Config Config

	ListeningPeer *peer.Peer

	randShards []byte
}

type Config struct {
	ExternalAddress    string
	MaxPeersSameShard  int
	MaxPeersOtherShard int
	MaxPeersOther      int
	MaxPeersNoShard    int
	MaxPeersBeacon     int
	// ListenerPeers defines a slice of listeners for which the connection
	// manager will take ownership of and accept connections.  When a
	// connection is accepted, the OnAccept handler will be invoked with the
	// connection.  Since the connection manager takes ownership of these
	// listeners, they will be closed when the connection manager is
	// stopped.
	//
	// This field will not have any effect if the OnAccept field is not
	// also specified.  It may be nil if the caller does not wish to listen
	// for incoming connections.
	ListenerPeer *peer.Peer

	// OnInboundAccept is a callback that is fired when an inbound connection is accepted
	OnInboundAccept func(peerConn *peer.PeerConn)

	//OnOutboundConnection is a callback that is fired when an outbound connection is established
	OnOutboundConnection func(peerConn *peer.PeerConn)

	//OnOutboundDisconnection is a callback that is fired when an outbound connection is disconnected
	OnOutboundDisconnection func(peerConn *peer.PeerConn)

	DiscoverPeers        bool
	DiscoverPeersAddress string
	ConsensusState       *ConsensusState
}

type DiscoverPeerInfo struct {
	PublicKey  string
	RawAddress string
	PeerID     libpeer.ID
}

func (connManager *ConnManager) UpdateConsensusState(role string, userPbk string, currentShard *byte, beaconCommittee []string, shardCommittee map[byte][]string) {
	connManager.Config.ConsensusState.Lock()
	defer connManager.Config.ConsensusState.Unlock()

	bChange := false
	if connManager.Config.ConsensusState.Role != role {
		connManager.Config.ConsensusState.Role = role
		bChange = true
	}
	if (connManager.Config.ConsensusState.CurrentShard != nil && currentShard == nil) ||
		(connManager.Config.ConsensusState.CurrentShard == nil && currentShard != nil) ||
		(connManager.Config.ConsensusState.CurrentShard != nil && currentShard != nil && *connManager.Config.ConsensusState.CurrentShard != *currentShard) {
		connManager.Config.ConsensusState.CurrentShard = currentShard
		bChange = true
	}
	if !common.CompareStringArray(connManager.Config.ConsensusState.BeaconCommittee, beaconCommittee) {
		connManager.Config.ConsensusState.BeaconCommittee = make([]string, len(beaconCommittee))
		copy(connManager.Config.ConsensusState.BeaconCommittee, beaconCommittee)
		bChange = true
	}
	if len(connManager.Config.ConsensusState.CommitteeByShard) != len(shardCommittee) {
		for shardID, _ := range connManager.Config.ConsensusState.CommitteeByShard {
			_, ok := shardCommittee[shardID]
			if !ok {
				delete(connManager.Config.ConsensusState.CommitteeByShard, shardID)
			}
		}
		bChange = true
	}
	if connManager.Config.ConsensusState.CommitteeByShard == nil {
		connManager.Config.ConsensusState.CommitteeByShard = make(map[byte][]string)
	}
	for shardID, committee := range shardCommittee {
		_, ok := connManager.Config.ConsensusState.CommitteeByShard[shardID]
		if ok {
			if !common.CompareStringArray(connManager.Config.ConsensusState.CommitteeByShard[shardID], committee) {
				connManager.Config.ConsensusState.CommitteeByShard[shardID] = make([]string, len(committee))
				copy(connManager.Config.ConsensusState.CommitteeByShard[shardID], committee)
				bChange = true
			}
		} else {
			connManager.Config.ConsensusState.CommitteeByShard[shardID] = make([]string, len(committee))
			copy(connManager.Config.ConsensusState.CommitteeByShard[shardID], committee)
			bChange = true
		}
	}
	if connManager.Config.ConsensusState.UserPublicKey != userPbk {
		connManager.Config.ConsensusState.UserPublicKey = userPbk
		bChange = true
	}

	// update peer connection
	if bChange {
		connManager.Config.ConsensusState.rebuild()
		go connManager.processDiscoverPeers()
	}

	return
}

// Stop gracefully shuts down the connection manager.
func (connManager *ConnManager) Stop() {
	if atomic.AddInt32(&connManager.stop, 1) != 1 {
		Logger.log.Error("Connection manager already stopped")
		return
	}
	Logger.log.Warn("Stopping connection manager")

	// Stop all the listeners.  There will not be any listeners if
	// listening is disabled.
	listener := connManager.Config.ListenerPeer
	if listener != nil {
		listener.Stop()
	}

	if connManager.cDiscoveredPeers != nil {
		close(connManager.cDiscoveredPeers)
	}

	if connManager.cQuit != nil {
		close(connManager.cQuit)
	}
	Logger.log.Warn("Connection manager stopped")
}

func (connManager ConnManager) New(cfg *Config) *ConnManager {
	connManager.Config = *cfg
	connManager.cQuit = make(chan struct{})
	//connManager.discoveredPeers = make(map[string]*DiscoverPeerInfo)
	connManager.ListeningPeer = nil
	connManager.Config.ConsensusState = &ConsensusState{}
	connManager.cDiscoveredPeers = make(chan struct{})
	return &connManager
}

func (connManager *ConnManager) GetPeerId(addr string) (string, error) {
	ipfsAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		Logger.log.Error(err)
		return common.EmptyString, err
	}
	pid, err := ipfsAddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		Logger.log.Error(err)
		return common.EmptyString, err
	}
	peerId, err := libpeer.IDB58Decode(pid)
	if err != nil {
		Logger.log.Error(err)
		return common.EmptyString, err
	}
	return peerId.Pretty(), nil
}

// Connect assigns an id and dials a connection to the address of the
// connection request.
func (connManager *ConnManager) Connect(addr string, publicKey string, cConn chan *peer.PeerConn) {
	if atomic.LoadInt32(&connManager.stop) != 0 {
		return
	}
	// The following code extracts target's peer Id from the
	// given multiaddress
	ipfsAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		Logger.log.Error(err)
		return
	}

	// decode to a peerID from ipfs address
	pid, err := ipfsAddr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		Logger.log.Error(err)
		return
	}
	peerId, err := libpeer.IDB58Decode(pid)
	if err != nil {
		Logger.log.Error(err)
		return
	}

	// Decapsulate the /ipfs/<peerID> part from the target
	// /ip4/<a.b.c.d>/ipfs/<peer> becomes /ip4/<a.b.c.d>
	// Create a Peer object
	targetPeerAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/ipfs/%s", libpeer.IDB58Encode(peerId)))
	targetAddr := ipfsAddr.Decapsulate(targetPeerAddr)

	listeningPeer := connManager.Config.ListenerPeer
	listeningPeer.HandleConnected = connManager.handleConnected
	listeningPeer.HandleDisconnected = connManager.handleDisconnected
	listeningPeer.HandleFailed = connManager.handleFailed

	peer := peer.Peer{
		TargetAddress:      targetAddr,
		PeerID:             peerId,
		RawAddress:         addr,
		Config:             listeningPeer.Config,
		PeerConns:          make(map[string]*peer.PeerConn),
		PendingPeers:       make(map[string]*peer.Peer),
		HandleConnected:    connManager.handleConnected,
		HandleDisconnected: connManager.handleDisconnected,
		HandleFailed:       connManager.handleFailed,
	}

	// if we can get an pubbic key from params?
	if publicKey != common.EmptyString {
		// use public key to detect role in network
		peer.PublicKey = publicKey
	}

	// add remote address peer into our listening node peer
	listeningPeer.Host.Peerstore().AddAddr(peer.PeerID, peer.TargetAddress, pstore.PermanentAddrTTL)
	Logger.log.Info("DEBUG Connect to RemotePeer", peer.PublicKey)
	Logger.log.Info(listeningPeer.Host.Peerstore().Addrs(peer.PeerID))
	listeningPeer.PushConn(&peer, cConn)
}

func (connManager *ConnManager) Start(discoverPeerAddress string) {
	// Already started?
	if atomic.AddInt32(&connManager.start, 1) != 1 {
		return
	}

	Logger.log.Info("Connection manager started")
	// Start handler to listen channel from connection peer
	//go connManager.connHandler()

	// Start all the listeners so long as the caller requested them and
	// provided a callback to be invoked when connections are accepted.
	if connManager.Config.OnInboundAccept != nil {
		listner := connManager.Config.ListenerPeer
		listner.HandleConnected = connManager.handleConnected
		listner.HandleDisconnected = connManager.handleDisconnected
		listner.HandleFailed = connManager.handleFailed
		go connManager.listenHandler(listner)
		connManager.ListeningPeer = listner

		if connManager.Config.DiscoverPeers && connManager.Config.DiscoverPeersAddress != common.EmptyString {
			Logger.log.Infof("DiscoverPeers: true\n----------------------------------------------------------------"+
				"\n|               Discover peer url: %s               |"+
				"\n----------------------------------------------------------------",
				connManager.Config.DiscoverPeersAddress)
			go connManager.DiscoverPeers(discoverPeerAddress)
		}
	}
}

// listenHandler accepts incoming connections on a given listener.  It must be
// run as a goroutine.
func (connManager *ConnManager) listenHandler(listen *peer.Peer) {
	listen.Start()
}

func (connManager *ConnManager) handleConnected(peerConn *peer.PeerConn) {
	Logger.log.Infof("handleConnected %s", peerConn.RemotePeerID.Pretty())
	if peerConn.GetIsOutbound() {
		Logger.log.Infof("handleConnected OUTBOUND %s", peerConn.RemotePeerID.Pretty())

		if connManager.Config.OnOutboundConnection != nil {
			connManager.Config.OnOutboundConnection(peerConn)
		}

	} else {
		Logger.log.Infof("handleConnected INBOUND %s", peerConn.RemotePeerID.Pretty())
	}
}

func (connManager *ConnManager) handleDisconnected(peerConn *peer.PeerConn) {
	Logger.log.Infof("handleDisconnected %s", peerConn.RemotePeerID.Pretty())
}

func (connManager *ConnManager) handleFailed(peerConn *peer.PeerConn) {
	Logger.log.Infof("handleFailed %s", peerConn.RemotePeerID.Pretty())
}

// DiscoverPeers - connect to bootnode
// create a rpc client to ping to bootnode
//
func (connManager *ConnManager) DiscoverPeers(discoverPeerAddress string) {
	Logger.log.Infof("Start Discover Peers : %s", discoverPeerAddress)
	connManager.randShards = connManager.makeRandShards(common.MAX_SHARD_NUMBER)
	connManager.discoverPeerAddress = discoverPeerAddress
	for {
		// main process of discover peer
		// connect RPC server of boot node
		// -> get response of peers
		// -> use to make peer connection
		err := connManager.processDiscoverPeers()
		if err != nil {
			continue
		}
		select {
		case <-connManager.cDiscoveredPeers:
			// receive channel stop
			Logger.log.Info("Stop Discover Peers")
			return
		case <-time.NewTimer(IntervalDiscoverPeer * time.Second).C:
			// every IntervalDiscoverPeer, (const = 60 second)
			// call processDiscoverPeers func to reconnect RPC server of boot node
			// and process data
			continue
		}
	}
}

// processDiscoverPeers - create a connection to
// RPC server of bootnode with golang RPC client
// after receive a response which contains data
// of peers(connectable peers) from bootnode
// conneManager should use this data to make connections with
// node peers are beacon committee
// node peers are shard commttee
// other role of other peers
func (connManager *ConnManager) processDiscoverPeers() error {
	discoverPeerAddress := connManager.discoverPeerAddress
	if discoverPeerAddress == "" {
		// we dont have config to make discover peer
		// so we dont need to do anything here
		return nil
	}

	// create a rpc client object,
	// connect to boot node with URL
	// get from discoverPeerAddress
	// in conf of our node
	client, err := rpc.Dial("tcp", discoverPeerAddress)
	if err != nil {
		// can not create connection to rpc server with
		// provided "discover peer address" in config
		Logger.log.Error("[Exchange Peers] re-connect:")
		Logger.log.Error(err)
		return err
	}
	if client != nil {
		defer client.Close()

		// get data about our current node peer
		listener := connManager.Config.ListenerPeer
		var response []wire.RawPeer

		externalAddress := connManager.Config.ExternalAddress
		Logger.log.Info("Start Process Discover Peers ExternalAddress", externalAddress)

		// remove later
		rawAddress := listener.RawAddress
		rawPort := listener.Port
		if externalAddress == common.EmptyString {
			externalAddress = os.Getenv("EXTERNAL_ADDRESS")
		}
		if externalAddress != common.EmptyString {
			host, port, err := net.SplitHostPort(externalAddress)
			if err == nil && host != common.EmptyString {
				rawAddress = strings.Replace(rawAddress, "127.0.0.1", host, 1)
				rawAddress = strings.Replace(rawAddress, "0.0.0.0", host, 1)
				rawAddress = strings.Replace(rawAddress, "localhost", host, 1)
				rawAddress = strings.Replace(rawAddress, fmt.Sprintf("/%s/", rawPort), fmt.Sprintf("/%s/", port), 1)
			}
		} else {
			rawAddress = ""
		}

		// In case WE run a node look like  committee of shard or beacon
		// we need TO Generate a signature with base58check format string
		// and send to boot node like a notice from us that
		// we live and we send info about us to bootnode(peerID, node rol, ...)
		publicKeyInBase58CheckEncode := ""
		signDataInBase58CheckEncode := ""
		if listener.Config.UserKeySet != nil {
			publicKeyInBase58CheckEncode = listener.Config.UserKeySet.GetPublicKeyB58()
			Logger.log.Info("Start Process Discover Peers", publicKeyInBase58CheckEncode)
			// sign data
			signDataInBase58CheckEncode, err = listener.Config.UserKeySet.SignDataB58([]byte(rawAddress))
			if err != nil {
				Logger.log.Error(err)
			}
		}

		// packing in a object PingArgs
		args := &server.PingArgs{
			RawAddress: rawAddress,
			PublicKey:  publicKeyInBase58CheckEncode,
			SignData:   signDataInBase58CheckEncode,
		}
		Logger.log.Infof("[Exchange Peers] Ping %+v", args)

		// Write more log to debug
		//Logger.log.Info("Dump PeerConns", len(listener.PeerConns))
		//for pubK, info := range connManager.discoveredPeers {
		//	var result []string
		//	for _, peerConn := range listener.PeerConns {
		//		if peerConn.RemotePeer.PublicKey == pubK {
		//			result = append(result, peerConn.RemotePeer.PeerID.Pretty())
		//		}
		//	}
		//	Logger.log.Infof("Public PubKey %s, %s, %s", pubK, info.PeerID.Pretty(), result)

		//for _, peerConn := range listener.PeerConns {
		//	Logger.log.Info("PeerConn state %s %s %s", peerConn.ConnState(), peerConn.GetIsOutbound(), peerConn.RemotePeerID.Pretty(), peerConn.RemotePeer.RawAddress)
		//}

		err := client.Call("Handler.Ping", args, &response)
		if err != nil {
			// can not call method PING to rpc server of boot node
			Logger.log.Error("[Exchange Peers] Ping:")
			Logger.log.Error(err)
			client = nil
			return err
		}
		// make models
		responsePeers := make(map[string]*wire.RawPeer)
		for _, rawPeer := range response {
			p := rawPeer
			responsePeers[rawPeer.PublicKey] = &p
		}
		// connect to relay nodes
		connManager.handleRelayNode(responsePeers)
		// connect to beacon peers
		connManager.handleRandPeersOfBeacon(connManager.Config.MaxPeersBeacon, responsePeers)
		// connect to same shard peers
		connManager.handleRandPeersOfShard(connManager.Config.ConsensusState.CurrentShard, connManager.Config.MaxPeersSameShard, responsePeers)
		// connect to other shard peers
		connManager.handleRandPeersOfOtherShard(connManager.Config.ConsensusState.CurrentShard, connManager.Config.MaxPeersOtherShard, connManager.Config.MaxPeersOther, responsePeers)
		// connect to no shard peers
		connManager.handleRandPeersOfNoShard(connManager.Config.MaxPeersNoShard, responsePeers)
	}
	return nil
}

func (connManager *ConnManager) getPeerIdsFromPbk(pbk string) []libpeer.ID {
	result := make([]libpeer.ID, 0)
	listener := connManager.Config.ListenerPeer
	allPeers := listener.GetPeerConnOfAll()
	for _, peerConn := range allPeers {
		// Logger.log.Info("Test PeerConn", peerConn.RemotePeer.PaymentAddress)
		if peerConn.RemotePeer.PublicKey == pbk {
			exist := false
			for _, item := range result {
				if item.Pretty() == peerConn.RemotePeer.PeerID.Pretty() {
					exist = true
				}
			}
			if !exist {
				result = append(result, peerConn.RemotePeer.PeerID)
			}
		}
	}
	return result
}

func (connManager *ConnManager) getPeerConnOfShard(shard *byte) []*peer.PeerConn {
	c := make([]*peer.PeerConn, 0)
	listener := connManager.Config.ListenerPeer
	allPeers := listener.GetPeerConnOfAll()
	for _, peerConn := range allPeers {
		sh := connManager.getShardOfPublicKey(peerConn.RemotePeer.PublicKey)
		if (shard == nil && sh == nil) || (sh != nil && shard != nil && *sh == *shard) {
			c = append(c, peerConn)
		}
	}
	return c
}

func (connManager *ConnManager) countPeerConnOfShard(shard *byte) int {
	c := 0
	listener := connManager.Config.ListenerPeer
	allPeers := listener.GetPeerConnOfAll()
	for _, peerConn := range allPeers {
		sh := connManager.getShardOfPublicKey(peerConn.RemotePeer.PublicKey)
		if (shard == nil && sh == nil) || (sh != nil && shard != nil && *sh == *shard) {
			c++
		}
	}
	return c
}

func (connManager *ConnManager) checkPeerConnOfPublicKey(publicKey string) bool {
	listener := connManager.Config.ListenerPeer
	pcs := listener.GetPeerConnOfAll()
	for _, peerConn := range pcs {
		if peerConn.RemotePeer.PublicKey == publicKey {
			return true
		}
	}
	return false
}

// checkBeaconOfPbk - check a public key is beacon committee?
func (connManager *ConnManager) checkBeaconOfPbk(pbk string) bool {
	beaconCommittee := connManager.Config.ConsensusState.getBeaconCommittee()
	if pbk != "" && common.IndexOfStr(pbk, beaconCommittee) >= 0 {
		return true
	}
	return false
}

func (connManager *ConnManager) closePeerConnOfShard(shard byte) {
	cPeers := connManager.getPeerConnOfShard(&shard)
	for _, p := range cPeers {
		p.ForceClose()
	}
}

func (connManager *ConnManager) handleRandPeersOfShard(shard *byte, maxPeers int, mPeers map[string]*wire.RawPeer) int {
	if shard == nil {
		return 0
	}
	//Logger.log.Info("handleRandPeersOfShard", *shard)
	countPeerShard := connManager.countPeerConnOfShard(shard)
	if countPeerShard >= maxPeers {
		// close if over max conn
		if countPeerShard > maxPeers {
			cPeers := connManager.getPeerConnOfShard(shard)
			lPeers := len(cPeers)
			for idx := maxPeers; idx < lPeers; idx++ {
				cPeers[idx].ForceClose()
			}
		}
		return maxPeers
	}
	pBKs := connManager.Config.ConsensusState.getCommitteeByShard(*shard)
	for len(pBKs) > 0 {
		randN := common.RandInt() % len(pBKs)
		pbk := pBKs[randN]
		pBKs = append(pBKs[:randN], pBKs[randN+1:]...)
		peerI, ok := mPeers[pbk]
		if ok {
			cPbk := connManager.Config.ConsensusState.UserPublicKey
			// if existed conn then not append to array
			if cPbk != pbk && !connManager.checkPeerConnOfPublicKey(pbk) {
				go connManager.Connect(peerI.RawAddress, peerI.PublicKey, nil)
				countPeerShard++
			}
			if countPeerShard >= maxPeers {
				return countPeerShard
			}
		}
	}
	return countPeerShard
}

func (connManager *ConnManager) handleRandPeersOfOtherShard(cShard *byte, maxShardPeers int, maxPeers int, mPeers map[string]*wire.RawPeer) int {
	//Logger.log.Info("handleRandPeersOfOtherShard", maxShardPeers, maxPeers)
	countPeers := 0
	for _, shard := range connManager.randShards {
		if cShard == nil || (cShard != nil && *cShard != shard) {
			if countPeers < maxPeers {
				mP := int(math.Min(float64(maxShardPeers), float64(maxPeers-countPeers)))
				cPeer := connManager.handleRandPeersOfShard(&shard, mP, mPeers)
				countPeers += cPeer
				if countPeers >= maxPeers {
					continue
				}
			}
			if countPeers >= maxPeers {
				connManager.closePeerConnOfShard(shard)
			}
		}
	}
	return countPeers
}

func (connManager *ConnManager) handleRandPeersOfBeacon(maxBeaconPeers int, mPeers map[string]*wire.RawPeer) int {
	Logger.log.Info("handleRandPeersOfBeacon")
	countPeerShard := 0
	pBKs := connManager.Config.ConsensusState.getBeaconCommittee()
	for len(pBKs) > 0 {
		randN := common.RandInt() % len(pBKs)
		pbk := pBKs[randN]
		pBKs = append(pBKs[:randN], pBKs[randN+1:]...)
		peerI, ok := mPeers[pbk]
		if ok {
			cPbk := connManager.Config.ConsensusState.UserPublicKey
			// if existed conn then not append to array
			if cPbk != pbk && !connManager.checkPeerConnOfPublicKey(pbk) {
				go connManager.Connect(peerI.RawAddress, peerI.PublicKey, nil)
			}
			countPeerShard++
			if countPeerShard >= maxBeaconPeers {
				return countPeerShard
			}
		}
	}
	return countPeerShard
}

func (connManager *ConnManager) handleRandPeersOfNoShard(maxPeers int, mPeers map[string]*wire.RawPeer) int {
	countPeers := 0
	shardByCommittee := connManager.Config.ConsensusState.getShardByCommittee()
	for _, peer := range mPeers {
		publicKey := peer.PublicKey
		if !connManager.checkPeerConnOfPublicKey(publicKey) {
			pBKs := connManager.Config.ConsensusState.getBeaconCommittee()
			if common.IndexOfStr(publicKey, pBKs) >= 0 {
				continue
			}
			_, ok := shardByCommittee[publicKey]
			if ok {
				continue
			}
			go connManager.Connect(peer.RawAddress, peer.PublicKey, nil)
			countPeers++
			if countPeers >= maxPeers {
				return countPeers
			}
		}
	}
	return countPeers
}

func (connManager *ConnManager) makeRandShards(maxShards int) []byte {
	shardBytes := make([]byte, 0)
	for i := 0; i < common.MAX_SHARD_NUMBER; i++ {
		shardBytes = append(shardBytes, byte(i))
	}
	shardsRet := make([]byte, 0)
	for len(shardsRet) < maxShards && len(shardBytes) > 0 {
		randN := common.RandInt() % len(shardBytes)
		shardV := shardBytes[randN]
		shardBytes = append(shardBytes[:randN], shardBytes[randN+1:]...)
		shardsRet = append(shardsRet, shardV)
	}
	return shardsRet
}

// CheckForAcceptConn - return true if our connection manager can accept a new connection from new peer
func (connManager *ConnManager) CheckForAcceptConn(peerConn *peer.PeerConn) bool {
	if peerConn == nil {
		return false
	}
	// check max shard conn
	shardID := connManager.getShardOfPublicKey(peerConn.RemotePeer.PublicKey)
	currentShard := connManager.Config.ConsensusState.CurrentShard
	if shardID != nil && currentShard != nil && *shardID == *currentShard {
		//	same shard
		countPeerShard := connManager.countPeerConnOfShard(shardID)
		if countPeerShard > connManager.Config.MaxPeersSameShard {
			return false
		}
	} else if shardID != nil {
		//	other shard
		countPeerShard := connManager.countPeerConnOfShard(shardID)
		if countPeerShard > connManager.Config.MaxPeersOtherShard {
			return false
		}
	} else if shardID == nil {
		// none shard
		countPeerShard := connManager.countPeerConnOfShard(nil)
		if countPeerShard > connManager.Config.MaxPeersNoShard {
			return false
		}
	}
	return true
}

//getShardOfPublicKey - return shardID of public key of peer connection
func (connManager *ConnManager) getShardOfPublicKey(publicKey string) *byte {
	shard, ok := connManager.Config.ConsensusState.ShardByCommittee[publicKey]
	if ok {
		return &shard
	}
	return nil
}

// GetCurrentRoleShard - return current role in shard of connected peer
func (connManager *ConnManager) GetCurrentRoleShard() (string, *byte) {
	return connManager.Config.ConsensusState.Role, connManager.Config.ConsensusState.CurrentShard
}

func (connManager *ConnManager) GetPeerConnOfShard(shard byte) []*peer.PeerConn {
	peerConns := make([]*peer.PeerConn, 0)
	listener := connManager.Config.ListenerPeer
	if listener != nil {
		allPeers := listener.GetPeerConnOfAll()
		for _, peerConn := range allPeers {
			shardT := connManager.getShardOfPublicKey(peerConn.RemotePeer.PublicKey)
			if shardT != nil && *shardT == shard {
				peerConns = append(peerConns, peerConn)
			}
		}
	}
	return peerConns
}

// GetPeerConnOfBeacon - return peer connection of nodes which are beacon committee
func (connManager *ConnManager) GetPeerConnOfBeacon() []*peer.PeerConn {
	peerConns := make([]*peer.PeerConn, 0)
	listener := connManager.Config.ListenerPeer
	if listener != nil {
		allPeers := listener.GetPeerConnOfAll()
		for _, peerConn := range allPeers {
			pbk := peerConn.RemotePeer.PublicKey
			if pbk != "" && connManager.checkBeaconOfPbk(pbk) {
				peerConns = append(peerConns, peerConn)
			}
		}
	}
	return peerConns
}

// GetPeerConnOfPublicKey - return PeerConn from public key
func (connManager *ConnManager) GetPeerConnOfPublicKey(publicKey string) []*peer.PeerConn {
	peerConns := make([]*peer.PeerConn, 0)
	if publicKey == "" {
		return peerConns
	}
	listener := connManager.Config.ListenerPeer
	if listener != nil {
		allPeers := listener.GetPeerConnOfAll()
		for _, peerConn := range allPeers {
			if publicKey == peerConn.RemotePeer.PublicKey {
				peerConns = append(peerConns, peerConn)
			}
		}
	}
	return peerConns
}

// GetPeerConnOfAll - return all Peer connection of node
func (connManager *ConnManager) GetPeerConnOfAll() []*peer.PeerConn {
	peerConns := make([]*peer.PeerConn, 0)
	listener := connManager.Config.ListenerPeer
	if listener != nil {
		peerConns = append(peerConns, listener.GetPeerConnOfAll()...)
	}
	return peerConns
}

// GetConnOfRelayNode - return connection of relay nodes
func (connManager *ConnManager) GetConnOfRelayNode() []*peer.PeerConn {
	peerConns := make([]*peer.PeerConn, 0)
	listener := connManager.Config.ListenerPeer
	if listener != nil {
		allPeers := listener.GetPeerConnOfAll()
		for _, peerConn := range allPeers {
			pbk := peerConn.RemotePeer.PublicKey
			if pbk != "" && common.IndexOfStr(pbk, peer.RelayNode) != -1 {
				peerConns = append(peerConns, peerConn)
			}
		}
	}
	return peerConns
}

// handle connect to relay node
func (connManager *ConnManager) handleRelayNode(mPeers map[string]*wire.RawPeer) {
	for _, p := range mPeers {
		publicKey := p.PublicKey
		if common.IndexOfStr(publicKey, peer.RelayNode) == -1 || connManager.checkPeerConnOfPublicKey(publicKey) || common.IndexOfStr(publicKey, protocol.RoundData.Committee) == -1 {
			continue
		}

		go connManager.Connect(p.RawAddress, p.PublicKey, nil)
	}
}
