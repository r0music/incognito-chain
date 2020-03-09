package blockchain

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/pubsub"

	"github.com/incognitochain/incognito-chain/common"
	libp2p "github.com/libp2p/go-libp2p-peer"
	"github.com/patrickmn/go-cache"
)

type PeerState struct {
	Shard               map[byte]*ChainState
	Beacon              *ChainState
	ShardToBeaconPool   *map[byte][]uint64
	CrossShardPool      map[byte]*map[byte][]uint64
	PeerMiningPublicKey string
}

type ChainState struct {
	Timestamp     int64
	Height        uint64
	BlockHash     common.Hash
	BestStateHash common.Hash
}

type ReportedChainState struct {
	ClosestBeaconState ChainState
	ClosestShardsState map[byte]ChainState
	ShardToBeaconBlks  map[byte]map[string][]uint64
	CrossShardBlks     map[byte]map[string][]uint64
}

type Synker struct {
	Status struct {
		sync.Mutex
		Beacon            bool
		Shards            map[byte]struct{}
		CurrentlySyncBlks *cache.Cache
		IsLatest          struct {
			Beacon bool
			Shards map[byte]bool
			sync.RWMutex
		}
	}
	States struct {
		PeersState   map[string]*PeerState
		ClosestState struct {
			ClosestBeaconState uint64
			ClosestShardsState sync.Map
			ShardToBeaconPool  sync.Map
			CrossShardPool     sync.Map
		}
		PoolsState struct {
			BeaconPool        []uint64
			ShardToBeaconPool map[byte][]uint64
			CrossShardPool    map[byte][]uint64
			ShardsPool        map[byte][]uint64
			sync.Mutex
		}
		sync.Mutex
	}
	Event struct {
		requestSyncShardBlockByHashEvent    pubsub.EventChannel
		requestSyncShardBlockByHeightEvent  pubsub.EventChannel
		requestSyncBeaconBlockByHashEvent   pubsub.EventChannel
		requestSyncBeaconBlockByHeightEvent pubsub.EventChannel
	}
	blockchain    *BlockChain
	pubSubManager *pubsub.PubSubManager
	cQuit         chan struct{}
}

var currentInsert = struct {
	Beacon sync.Mutex
	Shards map[byte]*sync.Mutex
}{
	Shards: make(map[byte]*sync.Mutex),
}

func newSyncker(cQuit chan struct{}, blockchain *BlockChain, pubSubManager *pubsub.PubSubManager) Synker {
	s := Synker{
		blockchain:    blockchain,
		cQuit:         cQuit,
		pubSubManager: pubSubManager,
	}
	_, s.Event.requestSyncShardBlockByHashEvent, _ = pubSubManager.RegisterNewSubscriber(pubsub.RequestShardBlockByHashTopic)
	_, s.Event.requestSyncShardBlockByHeightEvent, _ = pubSubManager.RegisterNewSubscriber(pubsub.RequestShardBlockByHeightTopic)
	_, s.Event.requestSyncBeaconBlockByHashEvent, _ = pubSubManager.RegisterNewSubscriber(pubsub.RequestBeaconBlockByHashTopic)
	_, s.Event.requestSyncBeaconBlockByHeightEvent, _ = pubSubManager.RegisterNewSubscriber(pubsub.RequestBeaconBlockByHeightTopic)
	return s
}
func (synker *Synker) Start() {
	if synker.Status.Beacon {
		return
	}

	synker.Status.Beacon = true
	synker.Status.CurrentlySyncBlks = cache.New(DefaultMaxBlockSyncTime, DefaultCacheCleanupTime)
	synker.Status.Shards = make(map[byte]struct{})
	synker.Status.IsLatest.Shards = make(map[byte]bool)
	synker.States.PeersState = make(map[string]*PeerState)
	synker.States.ClosestState.ClosestShardsState = sync.Map{}
	synker.States.ClosestState.ShardToBeaconPool = sync.Map{}
	synker.States.ClosestState.CrossShardPool = sync.Map{}
	synker.States.PoolsState.ShardToBeaconPool = make(map[byte][]uint64)
	synker.States.PoolsState.CrossShardPool = make(map[byte][]uint64)
	synker.States.PoolsState.ShardsPool = make(map[byte][]uint64)
	for shardID := 0; shardID < common.MaxShardNumber; shardID++ {
		currentInsert.Shards[byte(shardID)] = &sync.Mutex{}
	}
	synker.Status.Lock()
	synker.startSyncRelayShards()
	synker.Status.Unlock()

	broadcastTicker := time.NewTicker(DefaultBroadcastStateTime)
	insertPoolTicker := time.NewTicker(1 * time.Second)
	updateStatesTicker := time.NewTicker(DefaultStateUpdateTime)
	defer func() {
		broadcastTicker.Stop()
		insertPoolTicker.Stop()
		updateStatesTicker.Stop()
	}()
	for {
		select {
		case <-synker.cQuit:
			return
		case <-insertPoolTicker.C:
			synker.InsertBlockFromPool()
		case <-broadcastTicker.C:
			err := synker.checkStateAndPublishState()
			if err != nil {
				Logger.log.Debugf("Check state and publish node state error: %v", err)
			}
		case <-updateStatesTicker.C:
			synker.UpdateState()
		case msg := <-synker.Event.requestSyncShardBlockByHashEvent:
			// Message Value: "[shardID],[BlockHash]"
			str, ok := msg.Value.(string)
			if !ok {
				continue
			}
			strs := strings.Split(str, ",")
			shardID, err := strconv.Atoi(strs[0])
			if err != nil {
				continue
			}
			hash, err := common.Hash{}.NewHashFromStr(strs[1])
			if err != nil {
				continue
			}
			synker.SyncBlkShard(byte(shardID), true, false, true, []common.Hash{*hash}, []uint64{}, 0, 0, "")
		case msg := <-synker.Event.requestSyncShardBlockByHeightEvent:
			// Message Value: "[shardID],[blockheight]"
			str, ok := msg.Value.(string)
			if !ok {
				continue
			}
			strs := strings.Split(str, ",")
			shardID, err := strconv.Atoi(strs[0])
			if err != nil {
				continue
			}
			height, err := strconv.Atoi(strs[1])
			if err != nil {
				continue
			}
			synker.SyncBlkShard(byte(shardID), false, true, true, []common.Hash{}, []uint64{uint64(height)}, uint64(height), uint64(height), "")
		case msg := <-synker.Event.requestSyncBeaconBlockByHashEvent:
			// Message Value: [BlockHash]
			hash, ok := msg.Value.(common.Hash)
			if !ok {
				continue
			}
			synker.SyncBlkBeacon(true, false, true, []common.Hash{hash}, []uint64{}, 0, 0, "")
		case msg := <-synker.Event.requestSyncBeaconBlockByHeightEvent:
			// Message Value: [blockheight]
			height, ok := msg.Value.(uint64)
			if !ok {
				continue
			}
			synker.SyncBlkBeacon(false, true, true, []common.Hash{}, []uint64{uint64(height)}, uint64(height), uint64(height), "")
		}
	}
}

func (synker *Synker) checkStateAndPublishState() error {
	engine := synker.blockchain.config.ConsensusEngine
	userKey, _ := engine.GetCurrentMiningPublicKey()
	if userKey == "" {
		return errors.New("Can not load current mining key")
	}
	userLayer, userRole, shardID := engine.GetUserRole()
	if userRole == common.CommitteeRole {
		err := synker.blockchain.config.Server.PublishNodeState(userLayer, shardID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (synker *Synker) SyncShard(shardID byte) error {
	synker.Status.Lock()
	defer synker.Status.Unlock()
	return synker.syncShard(shardID)
}

func (synker *Synker) syncShard(shardID byte) error {
	if _, ok := synker.Status.Shards[shardID]; ok {
		return errors.New("Shard " + fmt.Sprintf("%d", shardID) + " synchronzation is already started")
	}
	Logger.log.Infof("*** Start syncing shard %+v ***", shardID)
	synker.Status.Shards[shardID] = struct{}{}
	return nil
}

func (synker *Synker) startSyncRelayShards() {
	for _, shardID := range synker.blockchain.config.RelayShards {
		if shardID > byte(synker.blockchain.GetBeaconBestState().ActiveShards-1) {
			break
		}
		synker.syncShard(shardID)
	}
}

func (synker *Synker) StopSyncUnnecessaryShard() {
	synker.Status.Lock()
	defer synker.Status.Unlock()
	synker.stopSyncUnnecessaryShard()
}

func (synker *Synker) stopSyncUnnecessaryShard() {
	for shardID := byte(0); shardID < common.MaxShardNumber; shardID++ {
		synker.stopSyncShard(shardID)
	}
}

func (synker *Synker) stopSyncShard(shardID byte) error {
	if synker.blockchain.config.NodeMode == common.NodeModeAuto || synker.blockchain.config.NodeMode == common.NodeModeShard {
		_, _, userShardIDInt := synker.blockchain.config.ConsensusEngine.GetUserRole()
		if userShardIDInt >= 0 {
			if byte(userShardIDInt) == shardID {
				return errors.New("Shard " + fmt.Sprintf("%d", shardID) + " synchronzation can't be stopped")
			}
		}
	}
	if _, ok := synker.Status.Shards[shardID]; ok {
		if common.IndexOfByte(shardID, synker.blockchain.config.RelayShards) < 0 {
			delete(synker.Status.Shards, shardID)
			return nil
		}
		return errors.New("Shard " + fmt.Sprintf("%d", shardID) + " synchronzation can't be stopped")
	}
	return errors.New("Shard " + fmt.Sprintf("%d", shardID) + " synchronzation is already stopped")
}

func (synker *Synker) UpdateState() {
	Logger.log.Debug("[updatestate] START update state")
	synker.Status.Lock()
	synker.States.Lock()
	Logger.log.Debug("[updatestate] Locked Status and States")
	synker.GetPoolsState()
	synker.Status.CurrentlySyncBlks.DeleteExpired()
	var shardsStateClone map[byte]ShardBestState
	shardsStateClone = make(map[byte]ShardBestState)
	var beaconStateClone BeaconBestState
	err := beaconStateClone.cloneBeaconBestStateFrom(synker.blockchain.GetBeaconBestState())
	if err != nil {
		panic(err)
	}
	var (
		userRole       string
		userLayer      string
		userShardRole  string
		userShardIDInt int
	)
	userKeyForCheckRole, _ := synker.blockchain.config.ConsensusEngine.GetCurrentMiningPublicKey()
	if userKeyForCheckRole != "" {
		userLayer, userRole, userShardIDInt = synker.blockchain.config.ConsensusEngine.GetUserRole()
		if userLayer == common.ShardRole && userRole != common.WaitingRole {
			synker.syncShard(byte(userShardIDInt))
			userShardRole = synker.blockchain.GetBestStateShard(byte(userShardIDInt)).GetPubkeyRole(userKeyForCheckRole, synker.blockchain.GetBestStateShard(byte(userShardIDInt)).BestBlock.Header.Round)
		}

	}
	synker.stopSyncUnnecessaryShard()

	synker.States.ClosestState.ClosestBeaconState = beaconStateClone.BeaconHeight
	for _, v := range synker.blockchain.ShardChain {
		synker.States.ClosestState.ClosestShardsState.Store(v.GetShardID(), v.GetBestState().ShardHeight)
	}

	for k, v := range synker.blockchain.config.ShardToBeaconPool.GetLatestValidPendingBlockHeight() {
		synker.States.ClosestState.ShardToBeaconPool.Store(k, v)
	}
	if userShardIDInt >= 0 {
		for k, v := range synker.blockchain.config.CrossShardPool[byte(userShardIDInt)].GetLatestValidBlockHeight() {
			synker.States.ClosestState.CrossShardPool.Store(k, v)
		}
	}

	RCS := ReportedChainState{
		ClosestBeaconState: ChainState{
			Height: beaconStateClone.BeaconHeight,
		},
		ClosestShardsState: make(map[byte]ChainState),
		ShardToBeaconBlks:  make(map[byte]map[string][]uint64),
		CrossShardBlks:     make(map[byte]map[string][]uint64),
	}

	bestShardsHeight := beaconStateClone.GetBestShardHeight()
	for shardID := byte(0); shardID < common.MaxShardNumber; shardID++ {
		RCS.ClosestShardsState[shardID] = ChainState{
			Height: bestShardsHeight[shardID],
		}
	}
	for shardID := range synker.Status.Shards {
		cloneState := ShardBestState{}
		err := cloneState.cloneShardBestStateFrom(synker.blockchain.GetBestStateShard(byte(shardID)))
		if err != nil {
			panic(err)
		}

		shardsStateClone[shardID] = cloneState
		RCS.ClosestShardsState[shardID] = ChainState{
			Height: shardsStateClone[shardID].ShardHeight,
		}
	}

	RCS = GetReportChainState(
		synker.States.PeersState,
		&RCS,
		synker.Status.Shards,
		&beaconStateClone,
		shardsStateClone,
	)

	for _, peerState := range synker.States.PeersState {
		// record pool state
		switch userLayer {
		case common.BeaconRole:
			if (synker.blockchain.config.NodeMode == common.NodeModeAuto || synker.blockchain.config.NodeMode == common.NodeModeBeacon) && userRole == common.CommitteeRole {
				if peerState.ShardToBeaconPool != nil {
					for shardID, blkHeights := range *peerState.ShardToBeaconPool {
						if len(synker.States.PoolsState.ShardToBeaconPool[shardID]) > 0 {
							if _, ok := RCS.ShardToBeaconBlks[shardID]; !ok {
								RCS.ShardToBeaconBlks[shardID] = make(map[string][]uint64)
							}
							RCS.ShardToBeaconBlks[shardID][peerState.PeerMiningPublicKey] = blkHeights
						}
					}
				}
				for shardID := byte(0); shardID < common.MaxShardNumber; shardID++ {
					if shardState, ok := peerState.Shard[shardID]; ok {
						if shardState.Height >= beaconStateClone.GetBestHeightOfShard(shardID) {
							if RCS.ClosestShardsState[shardID].Height == beaconStateClone.GetBestHeightOfShard(shardID) {
								RCS.ClosestShardsState[shardID] = *shardState
							} else {
								if shardState.Height < RCS.ClosestShardsState[shardID].Height {
									RCS.ClosestShardsState[shardID] = *shardState
								}
							}
						}
					}
				}
			}
		case common.ShardRole:
			if (synker.blockchain.config.NodeMode == common.NodeModeAuto || synker.blockchain.config.NodeMode == common.NodeModeShard) && (userShardRole == common.ProposerRole || userShardRole == common.ValidatorRole) {
				if pool, ok := peerState.CrossShardPool[byte(userShardIDInt)]; ok {
					for shardID, blkHeights := range *pool {
						if _, ok := RCS.CrossShardBlks[shardID]; !ok {
							RCS.CrossShardBlks[shardID] = make(map[string][]uint64)
						}
						RCS.CrossShardBlks[shardID][peerState.PeerMiningPublicKey] = blkHeights
					}
				}
			}
		}
	}

	synker.States.ClosestState.ClosestBeaconState = RCS.ClosestBeaconState.Height
	for shardID, state := range RCS.ClosestShardsState {
		synker.States.ClosestState.ClosestShardsState.Store(shardID, state.Height)
	}
	if len(synker.States.PeersState) > 0 {
		if userLayer != common.ShardRole {
			if RCS.ClosestBeaconState.Height == beaconStateClone.BeaconHeight {
				synker.SetChainState(false, 0, true)
			} else {
				Logger.log.Debugf("beacon not ready %v %v", userRole, RCS.ClosestBeaconState.Height)
				synker.SetChainState(false, 0, false)
			}
		}

		if userLayer == common.ShardRole && RCS.ClosestBeaconState.Height-1 <= beaconStateClone.BeaconHeight {
			if RCS.ClosestShardsState[byte(userShardIDInt)].Height == synker.blockchain.GetBestStateShard(byte(userShardIDInt)).ShardHeight && RCS.ClosestShardsState[byte(userShardIDInt)].Height >= beaconStateClone.GetBestHeightOfShard(byte(userShardIDInt)) {
				synker.SetChainState(false, 0, true)
				synker.SetChainState(true, byte(userShardIDInt), true)
			} else {
				Logger.log.Debugf("shard not ready", RCS.ClosestShardsState[byte(userShardIDInt)].Height)
				synker.SetChainState(false, 0, false)
				synker.SetChainState(true, byte(userShardIDInt), false)
			}
		}
	}

	// sync ShardToBeacon & CrossShard pool
	if synker.IsLatest(false, 0) {
		switch userLayer {
		case common.BeaconRole:
			if (synker.blockchain.config.NodeMode == common.NodeModeAuto || synker.blockchain.config.NodeMode == common.NodeModeBeacon) && userRole == common.CommitteeRole {
				for shardID, shardState := range RCS.ShardToBeaconBlks {
					for _, blks := range shardState {
						synker.SyncBlkShardToBeacon(shardID, false, true, false, nil, blks, 0, 0, libp2p.ID(""))
					}
				}
			}
		case common.ShardRole:
			if (synker.blockchain.config.NodeMode == common.NodeModeAuto || synker.blockchain.config.NodeMode == common.NodeModeShard) && (userShardRole == common.ProposerRole || userShardRole == common.ValidatorRole) {
				if synker.IsLatest(true, byte(userShardIDInt)) {
					for shardID, shardState := range RCS.CrossShardBlks {
						for _, blks := range shardState {
							synker.SyncBlkCrossShard(false, false, nil, blks, shardID, byte(userShardIDInt), libp2p.ID(""))
						}
					}
					blkMissing := GetMissingCrossShardBlock(
						synker.blockchain.GetDatabase(),
						beaconStateClone.LastCrossShardState,
						shardsStateClone[byte(userShardIDInt)].BestCrossShard,
						byte(userShardIDInt),
					)
					for shardID, blks := range blkMissing {
						synker.SyncBlkCrossShard(false, false, nil, blks, shardID, byte(userShardIDInt), libp2p.ID(""))
					}
				}
			}
		}
	}

	//Check beststate hash and sync if found some different beststate
	missingBestState := GetMissingBlockHashesFromPeersState(
		synker.States.PeersState,
		synker.Status.Shards,
		synker.blockchain.GetBeaconBestState,
		synker.blockchain.GetBestStateShard,
	)
	//remove hardcode later
	for cID, listBlks := range missingBestState {
		if cID == BEACON_ID {
			synker.SyncBlkBeacon(
				true,     // byHash
				false,    // bySpecific
				false,    // getFromPool
				listBlks, // []Hash
				nil,      // []heights
				0,        // from
				0,        // to
				libp2p.ID(""),
			)
		} else {
			synker.SyncBlkShard(
				byte(cID), // shardID
				true,      // byHash
				false,     // bySpecific
				false,     // getFromPool
				listBlks,  // []Hash
				nil,       // []heights
				0,         // from
				0,         // to
				libp2p.ID(""),
			)
		}
	}
	// sync beacon and missing block in beacon pool
	if RCS.ClosestBeaconState.Height-beaconStateClone.BeaconHeight > DefaultMaxBlkReqPerTime {
		RCS.ClosestBeaconState.Height = beaconStateClone.BeaconHeight + DefaultMaxBlkReqPerTime
	}
	synker.SyncBlkBeacon(
		false,                           // byHash
		false,                           // bySpecificHeights
		false,                           // getFromPool
		nil,                             // blksHash
		nil,                             // blkHeights
		beaconStateClone.BeaconHeight+1, // from
		RCS.ClosestBeaconState.Height,   // to
		libp2p.ID(""),
	)
	bcPool := synker.States.PoolsState.BeaconPool
	sort.Slice(bcPool, func(i, j int) bool {
		return bcPool[i] < bcPool[j]
	})
	heights := GetMissingBlockInPool(
		synker.blockchain.config.BeaconPool.GetBeaconState()+1,
		bcPool,
	)
	Logger.log.Debugf("[syncmissing] List block beacon needed to get %v", heights)
	synker.SyncBlkBeacon(
		false,   //byHash
		true,    // bySpecificHeights
		false,   //getFromPool
		nil,     //blksHash
		heights, // blkHeights
		0,       //from
		0,       //to
		libp2p.ID(""),
	)

	// sync shard and missing block in shard pool
	for shardID := range synker.Status.Shards {
		shardState := shardsStateClone[shardID]
		if RCS.ClosestShardsState[shardID].Height-shardState.ShardHeight > DefaultMaxBlkReqPerTime {
			RCS.ClosestShardsState[shardID] = ChainState{
				Height: shardState.ShardHeight + DefaultMaxBlkReqPerTime,
			}
		}
		synker.SyncBlkShard(
			shardID,                                // shardID
			false,                                  // byHash
			false,                                  // bySpecificHeights
			false,                                  // getFromPool
			nil,                                    // blksHash
			nil,                                    // blkHeights
			shardState.ShardHeight,                 // from
			RCS.ClosestShardsState[shardID].Height, // to
			libp2p.ID(""),
		)
		shPool := synker.States.PoolsState.ShardsPool[shardID]
		sort.Slice(shPool, func(i, j int) bool {
			return shPool[i] < shPool[j]
		})
		heights := GetMissingBlockInPool(
			synker.blockchain.config.ShardPool[shardID].GetLatestValidBlockHeight()+1,
			shPool,
		)
		Logger.log.Debugf("[syncmissing] List block shard %v needed to get %v", shardID, heights)
		synker.SyncBlkShard(
			shardID, // shardID
			false,   // byHash
			true,    // bySpecificHeights
			false,   // getFromPool
			nil,     // blksHash
			heights, // blkHeights
			0,       // from
			0,       // to
			libp2p.ID(""),
		)
	}

	beaconCommittee, _ := incognitokey.ExtractMiningPublickeysFromCommitteeKeyList(beaconStateClone.BeaconCommittee, beaconStateClone.ConsensusAlgorithm)
	shardCommittee := make(map[byte][]string)
	for shardID, committee := range beaconStateClone.GetShardCommittee() {
		shardCommittee[shardID], _ = incognitokey.ExtractMiningPublickeysFromCommitteeKeyList(committee, beaconStateClone.ShardConsensusAlgorithm[shardID])
	}
	userMiningKey, err := synker.blockchain.config.ConsensusEngine.GetMiningPublicKeyByConsensus(synker.blockchain.GetBeaconBestState().ConsensusAlgorithm)
	if err != nil {
		synker.Status.Unlock()
		synker.States.Unlock()
		panic(err)
	}
	if userLayer == common.ShardRole {
		shardID := byte(userShardIDInt)
		synker.blockchain.config.Server.UpdateConsensusState(userLayer, userMiningKey, &shardID, beaconCommittee, shardCommittee)
	} else {
		synker.blockchain.config.Server.UpdateConsensusState(userLayer, userMiningKey, nil, beaconCommittee, shardCommittee)
	}
	synker.States.PeersState = make(map[string]*PeerState)
	Logger.log.Debug("[updatestate] END update state")
	synker.Status.Unlock()
	synker.States.Unlock()
	Logger.log.Debug("[updatestate] Unlocked Status and States")
}

//SyncBlkBeacon Send a req to sync beacon block
/*
	- by Hash + blksHash: get by hash
	- from + to: get from main chain by height
	- GetFromPool: ignore mainchain, used only for hash
*/
func (synker *Synker) SyncBlkBeacon(byHash bool, bySpecificHeights bool, getFromPool bool, blksHash []common.Hash, blkHeights []uint64, from uint64, to uint64, peerID libp2p.ID) {
	cacheItems := synker.Status.CurrentlySyncBlks.Items()
	if byHash {
		//Sync block by hash
		prefix := getBlkPrefixSyncKey(true, BeaconBlk, 0, 0)
		blksNeedToGet := getBlkNeedToGetByHash(prefix, blksHash, cacheItems, peerID)
		if len(blksNeedToGet) > 0 {
			go synker.blockchain.config.Server.PushMessageGetBlockBeaconByHash(blksNeedToGet, getFromPool, peerID)
		}
		for _, blkHash := range blksNeedToGet {
			synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, blkHash.String()), time.Now().Unix(), DefaultMaxBlockSyncTime)
		}
	} else {
		//Sync by height
		prefix := getBlkPrefixSyncKey(false, BeaconBlk, 0, 0)
		if bySpecificHeights {
			blksNeedToGet := getBlkNeedToGetBySpecificHeight(prefix, blkHeights, cacheItems, synker.GetBeaconPoolStateByHeight())
			if len(blksNeedToGet) > 0 {
				go synker.blockchain.config.Server.PushMessageGetBlockBeaconBySpecificHeight(blksNeedToGet, getFromPool)
				for _, blkHeight := range blksNeedToGet {
					synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, blkHeight), time.Now().Unix(), DefaultMaxBlockSyncTime)
				}
			}
		} else {
			blkBatchsNeedToGet := getBlkNeedToGetByHeight(prefix, from, to, cacheItems, synker.GetBeaconPoolStateByHeight())
			if len(blkBatchsNeedToGet) > 0 {
				for fromHeight, toHeight := range blkBatchsNeedToGet {
					go synker.blockchain.config.Server.PushMessageGetBlockBeaconByHeight(fromHeight, toHeight)
					for height := fromHeight; height <= toHeight; height++ {
						synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, height), time.Now().Unix(), DefaultMaxBlockSyncTime)
					}
				}
			}
		}

	}
}

//SyncBlkShard Send a req to sync shard block
/*
	- by Hash + blksHash: get by hash
	- from + to: get from main chain by height
	- GetFromPool: ignore mainchain, used only for hash
*/
func (synker *Synker) SyncBlkShard(shardID byte, byHash bool, bySpecificHeights bool, getFromPool bool, blksHash []common.Hash, blkHeights []uint64, from uint64, to uint64, peerID libp2p.ID) {
	cacheItems := synker.Status.CurrentlySyncBlks.Items()
	if byHash {
		//Sync block by hash
		prefix := getBlkPrefixSyncKey(true, ShardBlk, shardID, 0)
		blksNeedToGet := getBlkNeedToGetByHash(prefix, blksHash, cacheItems, peerID)
		if len(blksNeedToGet) > 0 {
			go synker.blockchain.config.Server.PushMessageGetBlockShardByHash(shardID, blksNeedToGet, getFromPool, peerID)
		}
		for _, blkHash := range blksNeedToGet {
			Logger.log.Criticalf("Block need to get hash=%+v", blkHash.String())
			synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, blkHash.String()), time.Now().Unix(), DefaultMaxBlockSyncTime)
		}
	} else {
		//Sync by height
		prefix := getBlkPrefixSyncKey(false, ShardBlk, shardID, 0)
		if bySpecificHeights {
			blksNeedToGet := getBlkNeedToGetBySpecificHeight(prefix, blkHeights, cacheItems, synker.GetShardPoolStateByHeight(shardID))
			if len(blksNeedToGet) > 0 {
				go synker.blockchain.config.Server.PushMessageGetBlockShardBySpecificHeight(shardID, blksNeedToGet, getFromPool)
				for _, blkHeight := range blksNeedToGet {
					synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, blkHeight), time.Now().Unix(), DefaultMaxBlockSyncTime)
				}
			}
		} else {
			blkBatchsNeedToGet := getBlkNeedToGetByHeight(prefix, from, to, cacheItems, synker.GetShardPoolStateByHeight(shardID))
			Logger.log.Debug("SyncBlkShard", from, to, blkBatchsNeedToGet)
			if len(blkBatchsNeedToGet) > 0 {
				for fromHeight, toHeight := range blkBatchsNeedToGet {
					Logger.log.Debug("SyncBlkShard", shardID, fromHeight, toHeight, peerID)
					go synker.blockchain.config.Server.PushMessageGetBlockShardByHeight(shardID, fromHeight, toHeight)
					for height := fromHeight; height <= toHeight; height++ {
						synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, height), time.Now().Unix(), DefaultMaxBlockSyncTime)
					}
				}
			}
		}
	}
}

func (synker *Synker) getMissingBlockInPool(
	isBeacon bool,
	shardID int,
) []uint64 {
	// Logger.log.Infof("[sync] syncMissingBlockInPool")
	listPendingBlks := []uint64{}
	listBlkToSync := []uint64{}
	// Start can not be "1" because genesis block height is 1 and all of the validators have it.
	start := uint64(2)
	if isBeacon {
		start = synker.blockchain.config.BeaconPool.GetBeaconState() + 1
		listPendingBlks = synker.blockchain.config.BeaconPool.GetPendingBlockHeight()
	} else {
		start = synker.blockchain.config.ShardPool[byte(shardID)].GetLatestValidBlockHeight() + 1
		listPendingBlks = synker.blockchain.config.ShardPool[byte(shardID)].GetPendingBlockHeight()
	}

	for _, blkHeight := range listPendingBlks {
		for blkNeedToSync := start; blkNeedToSync < blkHeight; blkNeedToSync++ {
			listBlkToSync = append(listBlkToSync, blkNeedToSync)
			if len(listBlkToSync) >= DefaultMaxBlkReqPerPeer {
				return listBlkToSync
			}
		}
		start = blkHeight + 1
	}

	if len(listBlkToSync) == 0 {
		Logger.log.Infof("[sync] %v Don't have missing blocks", shardID)
		return nil
	}

	return listBlkToSync
}

//SyncBlkShardToBeacon Send a req to sync shardToBeacon block
/*
	- by Hash + blksHash: get by hash
	- from + to: get from main chain by height
	- GetFromPool: ignore mainchain, used only for hash
*/
func (synker *Synker) SyncBlkShardToBeacon(shardID byte, byHash bool, bySpecificHeights bool, getFromPool bool, blksHash []common.Hash, blkHeights []uint64, from uint64, to uint64, peerID libp2p.ID) {
	cacheItems := synker.Status.CurrentlySyncBlks.Items()
	if byHash {
		Logger.log.Infof("[sync] REQUEST SYNC S2B byHash %v From %v To %v shardID %v ", blksHash, from, to, shardID)
		//Sync block by hash
		prefix := getBlkPrefixSyncKey(true, ShardToBeaconBlk, shardID, 0)
		blksNeedToGet := getBlkNeedToGetByHash(prefix, blksHash, cacheItems, peerID)
		if len(blksNeedToGet) > 0 {
			go synker.blockchain.config.Server.PushMessageGetBlockShardToBeaconByHash(shardID, blksNeedToGet, getFromPool, peerID)
		}
		for _, blkHash := range blksNeedToGet {
			synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, blkHash.String()), time.Now().Unix(), DefaultMaxBlockSyncTime)
		}
	} else {
		//Sync by height
		prefix := getBlkPrefixSyncKey(false, ShardToBeaconBlk, shardID, 0)
		if bySpecificHeights {
			Logger.log.Infof("[sync] REQUEST SYNC S2B bySpecificHeights %v %v %v %v", blkHeights, from, to, shardID)
			blksNeedToGet := getBlkNeedToGetBySpecificHeight(
				prefix,
				blkHeights,
				cacheItems,
				synker.GetShardToBeaconPoolStateByHeight(shardID),
			)
			Logger.log.Infof("[sync] Blks need to get %v", blksNeedToGet)
			if len(blksNeedToGet) > 0 {
				go synker.blockchain.config.Server.PushMessageGetBlockShardToBeaconBySpecificHeight(shardID, blksNeedToGet, getFromPool, peerID)
				for _, blkHeight := range blksNeedToGet {
					synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, blkHeight), time.Now().Unix(), DefaultMaxBlockSyncTime)
				}
			}
		} else {
			Logger.log.Infof("[sync] REQUEST SYNC S2B BlkHeights %v From %v To %v shardID %v ", blkHeights, from, to, shardID)
			blkBatchsNeedToGet := getBlkNeedToGetByHeight(prefix, from, to, cacheItems, synker.GetShardToBeaconPoolStateByHeight(shardID))
			Logger.log.Infof("[sync] Blks need to get %v", blkBatchsNeedToGet)
			if len(blkBatchsNeedToGet) > 0 {
				for fromHeight, toHeight := range blkBatchsNeedToGet {
					go synker.blockchain.config.Server.PushMessageGetBlockShardToBeaconByHeight(shardID, fromHeight, toHeight)
					for height := fromHeight; height <= toHeight; height++ {
						synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, height), time.Now().Unix(), DefaultMaxBlockSyncTime)
					}
				}
			}

		}
	}
}

//SyncBlkCrossShard Send a req to sync crossShard block
/*
	From Shard: shard creates cross shard block
	To  Shard: shard receive cross shard block
*/
func (synker *Synker) SyncBlkCrossShard(getFromPool bool, byHash bool, blksHash []common.Hash, blksHeight []uint64, fromShard byte, toShard byte, peerID libp2p.ID) {
	Logger.log.Infof("[sync] START Shard %+v request CrossShardBlock with Height %+v from shard %+v \n", fromShard, blksHeight, toShard)
	defer Logger.log.Infof("[sync] END   Shard %+v request CrossShardBlock with Height %+v from shard %+v \n", fromShard, blksHeight, toShard)
	cacheItems := synker.Status.CurrentlySyncBlks.Items()
	if byHash {
		Logger.log.Infof("[sync] NOOOOOOOOOOOOOOOOO Request by hash!!!!!!!!!!!!!!!!!")
		prefix := getBlkPrefixSyncKey(true, CrossShardBlk, toShard, fromShard)
		blksNeedToGet := getBlkNeedToGetByHash(prefix, blksHash, cacheItems, peerID)
		if len(blksNeedToGet) > 0 {
			go synker.blockchain.config.Server.PushMessageGetBlockCrossShardByHash(fromShard, toShard, blksNeedToGet, getFromPool, peerID)
		}
		for _, blkHash := range blksNeedToGet {
			synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, blkHash.String()), time.Now().Unix(), DefaultMaxBlockSyncTime)
		}
	} else {
		//Sync by specific heights
		prefix := getBlkPrefixSyncKey(false, CrossShardBlk, toShard, fromShard)
		var tempBlksHeight []uint64
		for _, value := range blksHeight {
			if value != 0 {
				tempBlksHeight = append(tempBlksHeight, value)
			}
		}
		blksHeight = tempBlksHeight
		if len(blksHeight) == 0 {
			return
		}
		Logger.log.Infof("[sync] REQUEST SYNC S2B Blk Cross Shard %v", blksHeight, fromShard, toShard)
		blksNeedToGet := getBlkNeedToGetBySpecificHeight(prefix, blksHeight, cacheItems, synker.GetCrossShardPoolStateByHeight(fromShard))
		Logger.log.Infof("[sync] Oke, request block %v", blksNeedToGet)
		if len(blksNeedToGet) > 0 {
			go synker.blockchain.config.Server.PushMessageGetBlockCrossShardBySpecificHeight(fromShard, toShard, blksNeedToGet, getFromPool, peerID)
			for _, blkHeight := range blksNeedToGet {
				synker.Status.CurrentlySyncBlks.Add(fmt.Sprintf("%v%v", prefix, blkHeight), time.Now().Unix(), DefaultMaxBlockSyncTime)
			}
		}

	}
}

func (synker *Synker) SetChainState(shard bool, shardID byte, ready bool) {
	synker.Status.IsLatest.Lock()
	defer synker.Status.IsLatest.Unlock()
	if shard {
		synker.Status.IsLatest.Shards[shardID] = ready
	} else {
		synker.Status.IsLatest.Beacon = ready
	}
}

func (synker *Synker) IsLatest(shard bool, shardID byte) bool {
	synker.Status.IsLatest.RLock()
	defer synker.Status.IsLatest.RUnlock()
	if shard {
		if _, ok := synker.Status.IsLatest.Shards[shardID]; !ok {
			return false
		}
		return synker.Status.IsLatest.Shards[shardID]
	}
	return synker.Status.IsLatest.Beacon
}

func (synker *Synker) GetPoolsState() {

	var (
		userRole      string
		userShardID   byte
		userShardRole string
		userPK        string
	)
	userPK, _ = synker.blockchain.config.ConsensusEngine.GetCurrentMiningPublicKey()

	if userPK != "" {
		userRole, userShardID = synker.blockchain.GetBeaconBestState().GetPubkeyRole(userPK, synker.blockchain.GetBeaconBestState().BestBlock.Header.Round)
		userShardRole = synker.blockchain.GetBestStateShard(userShardID).GetPubkeyRole(userPK, synker.blockchain.GetBestStateShard(userShardID).BestBlock.Header.Round)
	}

	synker.States.PoolsState.Lock()
	defer synker.States.PoolsState.Unlock()

	synker.States.PoolsState.BeaconPool = synker.blockchain.config.BeaconPool.GetAllBlockHeight()

	for shardID := range synker.Status.Shards {
		synker.States.PoolsState.ShardsPool[shardID] = synker.blockchain.config.ShardPool[shardID].GetAllBlockHeight()
	}

	if userRole == common.ProposerRole || userRole == common.ValidatorRole {
		synker.States.PoolsState.ShardToBeaconPool = synker.blockchain.config.ShardToBeaconPool.GetAllBlockHeight()
	}

	if userShardRole == common.ProposerRole || userShardRole == common.ValidatorRole {
		synker.States.PoolsState.CrossShardPool = synker.blockchain.config.CrossShardPool[userShardID].GetAllBlockHeight()
	}
}

func (synker *Synker) GetBeaconPoolStateByHeight() []uint64 {
	synker.States.PoolsState.Lock()
	defer synker.States.PoolsState.Unlock()
	result := make([]uint64, len(synker.States.PoolsState.BeaconPool))
	copy(result, synker.States.PoolsState.BeaconPool)
	return result
}

func (synker *Synker) GetShardPoolStateByHeight(shardID byte) []uint64 {
	synker.States.PoolsState.Lock()
	defer synker.States.PoolsState.Unlock()
	result := make([]uint64, len(synker.States.PoolsState.ShardsPool[shardID]))
	copy(result, synker.States.PoolsState.ShardsPool[shardID])
	return result
}

func (synker *Synker) GetShardToBeaconPoolStateByHeight(shardID byte) []uint64 {
	synker.States.PoolsState.Lock()
	defer synker.States.PoolsState.Unlock()
	if blks, ok := synker.States.PoolsState.ShardToBeaconPool[shardID]; ok {
		result := make([]uint64, len(blks))
		copy(result, blks)
		return result
	}
	return nil
}

func (synker *Synker) GetCrossShardPoolStateByHeight(fromShard byte) []uint64 {
	synker.States.PoolsState.Lock()
	defer synker.States.PoolsState.Unlock()
	if blks, ok := synker.States.PoolsState.CrossShardPool[fromShard]; ok {
		result := make([]uint64, len(blks))
		copy(result, blks)
		return result
	}
	return nil
}

func (synker *Synker) GetCurrentSyncShards() []byte {
	synker.Status.Lock()
	defer synker.Status.Unlock()
	var currentSyncShards []byte
	for shardID := range synker.Status.Shards {
		currentSyncShards = append(currentSyncShards, shardID)
	}
	return currentSyncShards
}

func (synker *Synker) InsertBlockFromPool() {
	go func() {
		if !synker.blockchain.config.ConsensusEngine.IsOngoing(common.BeaconChainKey) {
			synker.InsertBeaconBlockFromPool()
		}
	}()

	synker.Status.Lock()
	for shardID := range synker.Status.Shards {
		if !synker.blockchain.config.ConsensusEngine.IsOngoing(common.GetShardChainKey(shardID)) {
			go func(shardID byte) {
				synker.InsertShardBlockFromPool(shardID)
			}(shardID)
		}
	}
	synker.Status.Unlock()
}

func (synker *Synker) InsertBeaconBlockFromPool() {
	currentInsert.Beacon.Lock()
	defer currentInsert.Beacon.Unlock()
	blocks := synker.blockchain.config.BeaconPool.GetValidBlock()
	if len(blocks) > 0 {
		Logger.log.Debugf("InsertBeaconBlockFromPool %d", len(blocks))
	}

	chain := synker.blockchain.Chains[common.BeaconChainKey]
	bestState := &BeaconBestState{}
	bestState.cloneBeaconBestStateFrom(synker.blockchain.GetBeaconBestState())
	curEpoch := bestState.Epoch
	sameCommitteeBlock := blocks
	for i, v := range blocks {
		if v.GetCurrentEpoch() == curEpoch+1 {
			sameCommitteeBlock = blocks[:i+1]
			break
		}
	}

	for i, blk := range sameCommitteeBlock {
		if i == len(sameCommitteeBlock)-1 {
			break
		}
		if blk.Header.Height != sameCommitteeBlock[i+1].Header.Height-1 {
			sameCommitteeBlock = blocks[:i+1]
			break
		}
	}

	for i := len(sameCommitteeBlock) - 1; i >= 0; i-- {
		if err := chain.ValidateBlockSignatures(sameCommitteeBlock[i], bestState.BeaconCommittee); err != nil {
			sameCommitteeBlock = sameCommitteeBlock[:i]
			//TODO: remove invalid block
		} else {
			break
		}
	}

	if len(sameCommitteeBlock) > 0 {
		if sameCommitteeBlock[0].Header.Height-1 != bestState.BeaconHeight {
			return
		}
	}

	for _, v := range sameCommitteeBlock {
		err := chain.InsertBlk(v)
		if err != nil {
			Logger.log.Error(err)
			break
		}
	}
}

func (synker *Synker) InsertShardBlockFromPool(shardID byte) {
	Logger.log.Debug("InsertShardBlockFromPool start")
	currentInsert.Shards[shardID].Lock()
	defer currentInsert.Shards[shardID].Unlock()

	blocks := synker.blockchain.config.ShardPool[shardID].GetValidBlock()
	if len(blocks) > 0 {
		Logger.log.Debugf("InsertShardBlockFromPool %d blocks", len(blocks))
	}

	chain := synker.blockchain.Chains[common.GetShardChainKey(shardID)]
	bestState := &ShardBestState{}
	sbs, _ := synker.blockchain.GetShardBestState(shardID)
	bestState.cloneShardBestStateFrom(sbs)

	curEpoch := bestState.Epoch
	sameCommitteeBlock := blocks

	for i, v := range blocks {
		if v.GetCurrentEpoch() == curEpoch+1 {
			sameCommitteeBlock = blocks[:i+1]
			break
		}
	}

	for i, blk := range sameCommitteeBlock {
		if i == len(sameCommitteeBlock)-1 {
			break
		}
		if blk.Header.Height != sameCommitteeBlock[i+1].Header.Height-1 {
			sameCommitteeBlock = blocks[:i+1]
			break
		}
	}

	for i := len(sameCommitteeBlock) - 1; i >= 0; i-- {
		if err := chain.ValidateBlockSignatures(sameCommitteeBlock[i], chain.GetCommittee()); err != nil {
			sameCommitteeBlock = sameCommitteeBlock[:i]
			//TODO: remove invalid block
		} else {
			break
		}
	}

	if len(sameCommitteeBlock) > 0 {
		if sameCommitteeBlock[0].Header.Height-1 != bestState.ShardHeight {
			return
		}
	}

	for _, v := range sameCommitteeBlock {
		err := chain.InsertBlk(v)
		if err != nil {
			Logger.log.Error(err)
			break
		}
	}

}

func (synker *Synker) GetClosestShardToBeaconPoolState() map[byte]uint64 {
	result := make(map[byte]uint64)
	synker.States.ClosestState.ShardToBeaconPool.Range(func(k interface{}, v interface{}) bool {
		shardID := k.(byte)
		height := v.(uint64)
		result[shardID] = height
		return true
	})
	return result
}

func (synker *Synker) GetClosestCrossShardPoolState() map[byte]uint64 {
	result := make(map[byte]uint64)
	synker.States.ClosestState.CrossShardPool.Range(func(k interface{}, v interface{}) bool {
		shardID := k.(byte)
		height := v.(uint64)
		result[shardID] = height
		return true
	})
	return result
}
