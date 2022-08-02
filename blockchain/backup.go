package blockchain

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/config"
	"github.com/incognitochain/incognito-chain/dataaccessobject/flatfile"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"
)

type checkpointInfo struct {
	Height                     uint64
	Hash                       string
	ConsensusStateDBRootHash   common.Hash
	TransactionStateDBRootHash common.Hash
	FeatureStateDBRootHash     common.Hash
	RewardStateDBRootHash      common.Hash
	SlashStateDBRootHash       common.Hash
}
type BackupProcess struct {
	CheckpointName string
	ChainInfo      map[int]checkpointInfo
}

type BackupManager struct {
	blockchain       *BlockChain
	lastBootStrap    *BackupProcess
	runningBootStrap *BackupProcess
}

type StateDBData struct {
	K []byte
	V []byte
}

func NewBackupManager(bc *BlockChain) *BackupManager {
	//read bootstrap dir and load lastBootstrap
	cfg := config.LoadConfig()
	fd, _ := os.OpenFile(path.Join(path.Join(cfg.DataDir, cfg.DatabaseDir), "backupinfo"), os.O_RDONLY, 0666)
	jsonStr, _ := ioutil.ReadAll(fd)
	lastBackup := &BackupProcess{}
	json.Unmarshal(jsonStr, &lastBackup)
	return &BackupManager{bc, lastBackup, nil}
}

func (s *BackupManager) GetLastestBootstrap() BackupProcess {
	return *s.lastBootStrap
}

func (s *BackupManager) Start() {
	shardBestView := map[int]*ShardBestState{}
	beaconBestView := s.blockchain.GetBeaconBestState()
	checkPoint := time.Now().Format(time.RFC3339)
	defer func() {
		s.runningBootStrap = nil
	}()
	for i := 0; i < s.blockchain.GetActiveShardNumber(); i++ {
		shardBestView[i] = s.blockchain.GetBestStateShard(byte(i))
	}

	//update current status
	bootstrapInfo := &BackupProcess{
		CheckpointName: checkPoint,
		ChainInfo:      make(map[int]checkpointInfo),
	}
	bootstrapInfo.ChainInfo[-1] = checkpointInfo{beaconBestView.GetHeight(), beaconBestView.BestBlock.Hash().String(),
		beaconBestView.ConsensusStateDBRootHash, common.Hash{},
		beaconBestView.FeatureStateDBRootHash, beaconBestView.RewardStateDBRootHash, common.Hash{}}
	s.runningBootStrap = bootstrapInfo

	//backup beacon then shard
	cfg := config.LoadConfig()
	s.backupBeacon(path.Join(cfg.DataDir, cfg.DatabaseDir, checkPoint), beaconBestView)
	for i := 0; i < s.blockchain.GetActiveShardNumber(); i++ {
		s.backupShard(path.Join(cfg.DataDir, cfg.DatabaseDir, checkPoint), shardBestView[i])
		bootstrapInfo.ChainInfo[i] = checkpointInfo{shardBestView[i].GetHeight(), shardBestView[i].BestBlock.Hash().String(),
			shardBestView[i].ConsensusStateDBRootHash, shardBestView[i].TransactionStateDBRootHash,
			shardBestView[i].FeatureStateDBRootHash, shardBestView[i].RewardStateDBRootHash, shardBestView[i].SlashStateDBRootHash}
	}

	//update final status
	s.lastBootStrap = bootstrapInfo
	fd, _ := os.OpenFile(path.Join(path.Join(cfg.DataDir, cfg.DatabaseDir), "backupinfo"), os.O_RDWR, 0666)
	jsonStr, _ := json.Marshal(bootstrapInfo)
	fd.Write(jsonStr)
	fd.Close()

	fmt.Println("update lastBootStrap", bootstrapInfo)
}

const (
	BeaconConsensus = 1
	BeaconFeature   = 2
	BeaconReward    = 3
	BeaconSlash     = 4
	ShardConsensus  = 5
	ShardTransacton = 6
	ShardFeature    = 7
	ShardReward     = 8
)

type CheckpointInfo struct {
	Hash   string
	Height int64
}

func (s *BackupManager) GetBackupReader(checkpoint string, cid int, dbType int) *flatfile.FlatFileManager {
	cfg := config.LoadConfig()
	dbLoc := path.Join(cfg.DataDir, cfg.DatabaseDir, checkpoint)
	switch dbType {
	case BeaconConsensus:
		dbLoc = path.Join(dbLoc, "beacon", "consensus")
	case BeaconFeature:
		dbLoc = path.Join(dbLoc, "beacon", "feature")
	case BeaconReward:
		dbLoc = path.Join(dbLoc, "beacon", "reward")
	case BeaconSlash:
		dbLoc = path.Join(dbLoc, "beacon", "slash")
	case ShardConsensus:
		dbLoc = path.Join(dbLoc, fmt.Sprintf("shard%v", cid), "consensus")
	case ShardTransacton:
		dbLoc = path.Join(dbLoc, fmt.Sprintf("shard%v", cid), "transaction")
	case ShardFeature:
		dbLoc = path.Join(dbLoc, fmt.Sprintf("shard%v", cid), "feature")
	case ShardReward:
		dbLoc = path.Join(dbLoc, fmt.Sprintf("shard%v", cid), "reward")
	}
	fmt.Println("GetBackupReader", dbLoc)
	ff, _ := flatfile.NewFlatFile(dbLoc, 5000)
	return ff
}

func (s *BackupManager) backupShard(name string, bestView *ShardBestState) {
	consensusDB := bestView.GetCopiedConsensusStateDB()
	txDB := bestView.GetCopiedTransactionStateDB()
	featureDB := bestView.GetCopiedFeatureStateDB()
	rewardDB := bestView.GetShardRewardStateDB()

	consensusFF, _ := flatfile.NewFlatFile(path.Join(name, fmt.Sprintf("shard%v", bestView.ShardID), "consensus"), 5000)
	featureFF, _ := flatfile.NewFlatFile(path.Join(name, fmt.Sprintf("shard%v", bestView.ShardID), "feature"), 5000)
	txFF, _ := flatfile.NewFlatFile(path.Join(name, fmt.Sprintf("shard%v", bestView.ShardID), "tx"), 5000)
	rewardFF, _ := flatfile.NewFlatFile(path.Join(name, fmt.Sprintf("shard%v", bestView.ShardID), "reward"), 5000)

	wg := sync.WaitGroup{}
	wg.Add(4)

	go backupStateDB(consensusDB, consensusFF, &wg)
	go backupStateDB(featureDB, featureFF, &wg)
	go backupStateDB(txDB, txFF, &wg)
	go backupStateDB(rewardDB, rewardFF, &wg)
	wg.Wait()
}

func (s *BackupManager) backupBeacon(name string, bestView *BeaconBestState) {
	consensusDB := bestView.GetBeaconConsensusStateDB()
	featureDB := bestView.GetBeaconFeatureStateDB()
	rewardDB := bestView.GetBeaconRewardStateDB()
	slashDB := bestView.GetBeaconSlashStateDB()

	consensusFF, _ := flatfile.NewFlatFile(path.Join(name, "beacon", "consensus"), 5000)
	featureFF, _ := flatfile.NewFlatFile(path.Join(name, "beacon", "feature"), 5000)
	rewardFF, _ := flatfile.NewFlatFile(path.Join(name, "beacon", "reward"), 5000)
	slashFF, _ := flatfile.NewFlatFile(path.Join(name, "beacon", "slash"), 5000)

	wg := sync.WaitGroup{}
	wg.Add(4)

	go backupStateDB(consensusDB, consensusFF, &wg)
	go backupStateDB(featureDB, featureFF, &wg)
	go backupStateDB(rewardDB, rewardFF, &wg)
	go backupStateDB(slashDB, slashFF, &wg)
	wg.Wait()
}

func backupStateDB(stateDB *statedb.StateDB, ff *flatfile.FlatFileManager, wg *sync.WaitGroup) {
	defer wg.Done()
	it := stateDB.GetIterator()
	batchData := []StateDBData{}
	totalLen := 0
	if stateDB == nil {
		return
	}
	for it.Next(false, true, true) {
		diskvalue, err := stateDB.Database().TrieDB().DiskDB().Get(it.Key)
		if err != nil {
			continue
		}
		//fmt.Println(it.Key, len(diskvalue))
		key := make([]byte, len(it.Key))
		copy(key, it.Key)
		data := StateDBData{key, diskvalue}
		batchData = append(batchData, data)
		if len(batchData) == 1000 {
			totalLen += 1000
			buf := new(bytes.Buffer)
			enc := gob.NewEncoder(buf)
			err := enc.Encode(batchData)
			if err != nil {
				panic(err)
			}
			_, err = ff.Append(buf.Bytes())
			if err != nil {
				panic(err)
			}
			batchData = []StateDBData{}
		}
	}
	if len(batchData) > 0 {
		buf := new(bytes.Buffer)
		enc := gob.NewEncoder(buf)
		enc.Encode(batchData)
		_, err := ff.Append(buf.Bytes())
		if err != nil {
			panic(err)
		}
	}
}
