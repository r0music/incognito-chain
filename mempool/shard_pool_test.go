package mempool

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/pubsub"
	"sync"
	"testing"
	"time"
)
var (
	shardPoolTest         *ShardPool
	bestShardHeight = make(map[byte]uint64)
	shardPoolMapInterface = make(map[byte]blockchain.ShardPool)
	pbShardPool           = pubsub.NewPubSubManager()
	shardBlock2           = &blockchain.ShardBlock{
		Header: blockchain.ShardHeader{
			ShardID: 0,
			Height: 2,
			Timestamp: time.Now().Unix()-100,
		},
	}
	shardBlock2Forked           = &blockchain.ShardBlock{
		Header: blockchain.ShardHeader{
			ShardID: 0,
			Height: 2,
			Timestamp: time.Now().Unix(),
		},
	}
	shardBlock3 = &blockchain.ShardBlock{
		Header: blockchain.ShardHeader{
			ShardID: 0,
			Height: 3,
			PrevBlockHash: shardBlock2.Header.Hash(),
		},
	}
	shardBlock3Forked = &blockchain.ShardBlock{
		Header: blockchain.ShardHeader{
			ShardID: 0,
			Height: 3,
			PrevBlockHash: shardBlock2Forked.Header.Hash(),
		},
	}
	shardBlock4 = &blockchain.ShardBlock{
		Header: blockchain.ShardHeader{
			ShardID: 0,
			Height: 4,
			PrevBlockHash: shardBlock3.Header.Hash(),
		},
	}
	shardBlock5 = &blockchain.ShardBlock{
		Header: blockchain.ShardHeader{
			ShardID: 0,
			Height: 5,
			PrevBlockHash: shardBlock4.Header.Hash(),
		},
	}
	shardBlock6 = &blockchain.ShardBlock{
		Header: blockchain.ShardHeader{
			ShardID: 0,
			Height: 6,
			PrevBlockHash: shardBlock5.Header.Hash(),
		},
	}
	shardBlock7 = &blockchain.ShardBlock{
		Header: blockchain.ShardHeader{
			ShardID: 0,
			Height: 7,
			PrevBlockHash: shardBlock6.Header.Hash(),
		},
	}
	pendingShardBlocks = []*blockchain.ShardBlock{}
	validShardBlocks = []*blockchain.ShardBlock{}
)
var InitShardPoolTest = func(pubsubManager *pubsub.PubSubManager) {
	shardPoolTest = new(ShardPool)
	shardPoolTest.mtx = new(sync.RWMutex)
	shardPoolTest.shardID = 0
	shardPoolTest.latestValidHeight = 1
	shardPoolTest.validPool = []*blockchain.ShardBlock{}
	shardPoolTest.pendingPool = make(map[uint64]*blockchain.ShardBlock)
	shardPoolTest.conflictedPool = make(map[common.Hash]*blockchain.ShardBlock)
	shardPoolTest.config = ShardPoolConfig{
		MaxValidBlock:   MAX_VALID_SHARD_BLK_IN_POOL,
		MaxPendingBlock: MAX_PENDING_SHARD_BLK_IN_POOL,
		CacheSize:       SHARD_CACHE_SIZE,
	}
	shardPoolTest.cache, _ = lru.New(beaconPool.config.CacheSize)
	shardPoolTest.PubSubManager = pubsubManager
	_, subChanRole, _ := shardPoolTest.PubSubManager.RegisterNewSubscriber(pubsub.ShardRoleTopic)
	shardPoolTest.RoleInCommitteesEvent = subChanRole
}
var _ = func() (_ struct{}) {
	for i:=0; i < 255; i++ {
		shardID := byte(i)
		bestShardHeight[shardID] = 1
	}
	blockchain.SetBestStateBeacon(&blockchain.BestStateBeacon{
		BestShardHeight: bestShardHeight,
	})
	blockchain.SetBestStateShard(0,&blockchain.BestStateShard{
		ShardHeight:1,
	} )
	InitShardPool(shardPoolMapInterface, pbShardPool)
	InitShardPoolTest(pbShardPool)
	go pbShardPool.Start()
	oldBlockHash := common.Hash{}
	for i := testLatestValidHeight + 1; i < MAX_VALID_BEACON_BLK_IN_POOL + testLatestValidHeight+2; i++ {
		shardBlock := &blockchain.ShardBlock{
			Header: blockchain.ShardHeader{
				ShardID: 0,
				Height: uint64(i),
			},
		}
		if i != 0 {
			shardBlock.Header.PrevBlockHash = oldBlockHash
		}
		oldBlockHash = shardBlock.Header.Hash()
		validShardBlocks = append(validShardBlocks, shardBlock)
	}
	for i := MAX_VALID_BEACON_BLK_IN_POOL + testLatestValidHeight + 2; i < MAX_VALID_BEACON_BLK_IN_POOL + MAX_PENDING_BEACON_BLK_IN_POOL + testLatestValidHeight + 3; i++ {
		shardBlock := &blockchain.ShardBlock{
			Header: blockchain.ShardHeader{
				ShardID: 0,
				Height: uint64(i),
			},
		}
		if i != 0 {
			shardBlock.Header.PrevBlockHash = oldBlockHash
		}
		oldBlockHash = shardBlock.Header.Hash()
		pendingShardBlocks = append(pendingShardBlocks, shardBlock)
	}
	Logger.Init(common.NewBackend(nil).Logger("test", true))
	return
}()
func ResetShardPool(){
	for i:=0; i < 255; i++ {
		shardID := byte(i)
		if shardPoolMap[shardID].RoleInCommitteesEvent != nil {
			close(shardPoolMap[shardID].RoleInCommitteesEvent)
		}
		shardPoolMap[shardID] = new(ShardPool)
		shardPoolMap[shardID].shardID = shardID
		shardPoolMap[shardID].latestValidHeight = 1
		shardPoolMap[shardID].RoleInCommittees = -1
		shardPoolMap[shardID].validPool = []*blockchain.ShardBlock{}
		shardPoolMap[shardID].conflictedPool = make(map[common.Hash]*blockchain.ShardBlock)
		shardPoolMap[shardID].config = defaultConfig
		shardPoolMap[shardID].pendingPool = make(map[uint64]*blockchain.ShardBlock)
		shardPoolMap[shardID].cache, _ = lru.New(shardPoolMap[shardID].config.CacheSize)
		shardPoolMap[shardID].PubSubManager = pbShardPool
		_, subChanRole, _ := shardPoolMap[shardID].PubSubManager.RegisterNewSubscriber(pubsub.ShardRoleTopic)
		shardPoolMap[shardID].RoleInCommitteesEvent = subChanRole
	}
}
func TestInitShardPool(t *testing.T) {
	for i:= 0; i < 255; i++ {
		shardID := byte(i)
		if shardPoolMap[shardID].latestValidHeight != 1 {
			t.Fatalf("Shard %+v Invalid Latest valid height, expect 1 but get %+v", shardPoolMap[shardID].shardID, shardPoolMap[shardID].latestValidHeight)
		}
		if shardPoolMap[shardID].RoleInCommittees != -1 {
			t.Fatal("Invalid Latest Role in committees")
		}
		if shardPoolMap[shardID].validPool == nil || (shardPoolMap[shardID].validPool != nil && len(shardPoolMap[shardID].validPool) != 0 ){
			t.Fatal("Invalid Valid Pool")
		}
		if shardPoolMap[shardID].pendingPool == nil {
			t.Fatal("Invalid Pending Pool")
		}
		if shardPoolMap[shardID].conflictedPool == nil {
			t.Fatal("Invalid Conflicted Pool")
		}
		if shardPoolMap[shardID].config.MaxValidBlock != MAX_VALID_SHARD_BLK_IN_POOL {
			t.Fatal("Invalid Max Valid Pool")
		}
		if shardPoolMap[shardID].config.MaxPendingBlock != MAX_PENDING_SHARD_BLK_IN_POOL {
			t.Fatal("Invalid Max Pending Pool")
		}
		if shardPoolMap[shardID].config.CacheSize != SHARD_CACHE_SIZE {
			t.Fatal("Invalid Shard Cache Size")
		}
		if shardPoolMap[shardID].cache == nil {
			t.Fatal("Invalid Cache")
		}
		if shardPoolMap[shardID].PubSubManager == nil {
			t.Fatal("Invalid Pubsub manager")
		}
		if shardPoolMap[shardID].RoleInCommitteesEvent == nil {
			t.Fatal("Invalid Role event")
		}
	}
}
func TestShardPoolStart(t *testing.T) {
	InitShardPoolTest(pbShardPool)
	cQuit := make(chan struct{})
	go shardPoolTest.Start(cQuit)
	// send event
	for i := 200 ; i < 255; i++ {
		go pbShardPool.PublishMessage(pubsub.NewMessage(pubsub.ShardRoleTopic, i))
		<-time.Tick(100 * time.Millisecond)
		shardPoolTest.mtx.RLock()
		if shardPoolTest.RoleInCommittees != i {
			t.Fatal("Fail to get Role In committees from event")
		}
		shardPoolTest.mtx.RUnlock()
	}
	close(cQuit)
	<-time.Tick(500 * time.Millisecond)
	shardPoolTest.mtx.RLock()
	if shardPoolTest.RoleInCommittees != -1 {
		t.Fatal("Fail to set default Role In committees when beacon pool is stop")
	}
	shardPoolTest.mtx.RUnlock()
}
func TestShardPoolSetShardState(t *testing.T) {
	for i:= 0; i < 255; i++ {
		shardID := byte(i)
		shardPoolMap[shardID].SetShardState(0)
		if shardPoolMap[shardID].latestValidHeight != 1 {
			t.Fatal("Invalid Latest Valid Height")
		}
		shardPoolMap[shardID].SetShardState(testLatestValidHeight)
		if shardPoolMap[shardID].latestValidHeight != testLatestValidHeight {
			t.Fatalf("Height Should be set %+v but get %+v \n", testLatestValidHeight, shardPoolMap[shardID].latestValidHeight)
		}
	}
	ResetShardPool()
}
func TestShardPoolGetShardState(t *testing.T) {
	for i:= 0; i < 255; i++ {
		shardID := byte(i)
		shardPoolMap[shardID].SetShardState(0)
		if shardPoolMap[shardID].GetShardState() != uint64(1) {
			t.Fatal("Invalid Latest Valid Height")
		}
		shardPoolMap[shardID].SetShardState(testLatestValidHeight)
		if shardPoolMap[shardID].GetShardState() != testLatestValidHeight {
			t.Fatalf("Height Should be set %+v but get %+v \n", testLatestValidHeight, shardPoolMap[shardID].latestValidHeight)
		}
	}
	ResetShardPool()
}
func TestShardPoolValidateShardBlock(t *testing.T) {
	// skip old block
	// Test receive old block than latestvalidheight
	// - Test old block is less than latestvalidheight 2 value => store in conflicted block
	InitShardPoolTest(pbShardPool)
	shardPoolTest.SetShardState(4)
	err = shardPoolTest.validateShardBlock(shardBlock3,false)
	if err != nil {
		if err.(*BlockPoolError).Code != ErrCodeMessage[OldBlockError].Code {
			t.Fatalf("Block %+v should return error %+v but get %+v", shardBlock3.Header.Height, ErrCodeMessage[OldBlockError].Code, err.(*BlockPoolError).Code)
		}
		if block, ok := shardPoolTest.conflictedPool[shardBlock3.Header.Hash()]; !ok {
			t.Fatalf("Block %+v should be in conflict pool but get %+v", shardBlock3.Header.Height, block.Header.Height)
		}
	}
	delete(shardPoolTest.conflictedPool, shardBlock3.Header.Hash())
	err = shardPoolTest.validateShardBlock(shardBlock4,true)
	if err != nil {
		if err.(*BlockPoolError).Code != ErrCodeMessage[OldBlockError].Code {
			t.Fatalf("Block %+v should return error %+v but get %+v", shardBlock4.Header.Height, ErrCodeMessage[OldBlockError].Code, err.(*BlockPoolError).Code)
		}
		if block, ok := shardPoolTest.conflictedPool[shardBlock4.Header.Hash()]; !ok {
			t.Fatalf("Block %+v should be in conflict pool but get %+v", shardBlock4.Header.Height, block.Header.Height)
		}
	}
	delete(shardPoolTest.conflictedPool, shardBlock4.Header.Hash())
	// - Test old block discard and not store in conflicted pool
	err = shardPoolTest.validateShardBlock(shardBlock2, false)
	if err == nil {
		t.Fatalf("Block %+v should be discard with state %+v", shardBlock2.Header.Height, shardPoolTest.latestValidHeight)
	} else {
		if err.(*BlockPoolError).Code != ErrCodeMessage[OldBlockError].Code {
			t.Fatalf("Block %+v should be discard with state %+v, error should be %+v but get %+v", beaconBlock2.Header.Height, beaconPoolTest.latestValidHeight, ErrCodeMessage[OldBlockError].Code, err.(*BlockPoolError).Code)
		}
		if block, ok := shardPoolTest.conflictedPool[shardBlock2.Header.Hash()]; ok {
			t.Fatalf("Block %+v should NOT be in conflict pool but get %+v", shardBlock2.Header.Height, block.Header.Height)
		}
	}
	//test duplicate and pending
	err = shardPoolTest.validateShardBlock(shardBlock6, false)
	if err != nil {
		t.Fatalf("Block %+v should be able to get in pending pool, state %+v", beaconBlock6.Header.Height, beaconPoolTest.latestValidHeight)
	}
	shardPoolTest.pendingPool[shardBlock6.Header.Height] = shardBlock6
	if _, ok := shardPoolTest.pendingPool[shardBlock6.Header.Height]; !ok {
		t.Fatalf("Block %+v should be in pending pool", shardBlock6.Header.Height)
	}
	err = shardPoolTest.validateShardBlock(shardBlock6, false)
	if err == nil {
		t.Fatalf("Block %+v should be duplicate \n", shardBlock6.Header.Height)
	} else {
		if err.(*BlockPoolError).Code != ErrCodeMessage[DuplicateBlockError].Code {
			t.Fatalf("Block %+v should return error %+v but get %+v", shardBlock6.Header.Height, ErrCodeMessage[DuplicateBlockError].Code, err.(*BlockPoolError).Code)
		}
	}
	// ignore if block is duplicate or exceed pool size or not
	err = shardPoolTest.validateShardBlock(shardBlock6, true)
	if err != nil {
		t.Fatalf("Block %+v should not be duplicate \n", shardBlock6.Header.Height)
	}
	delete(shardPoolTest.pendingPool, shardBlock6.Header.Height)
	for index, shardBlock := range pendingShardBlocks {
		if index < len(pendingShardBlocks) - 1 {
			shardPoolTest.pendingPool[shardBlock.Header.Height] = shardBlock
		}  else {
			err = shardPoolTest.validateShardBlock(shardBlock, false)
			if err == nil {
				t.Fatalf("Block %+v exceed pending pool capacity %+v \n", shardBlock.Header.Height, len(beaconPoolTest.pendingPool))
			} else {
				if err.(*BlockPoolError).Code != ErrCodeMessage[MaxPoolSizeError].Code {
					t.Fatalf("Block %+v should return error %+v but get %+v", shardBlock.Header.Height, ErrCodeMessage[MaxPoolSizeError].Code, err.(*BlockPoolError).Code)
				}
			}
			err = shardPoolTest.validateShardBlock(shardBlock, true)
			if err != nil {
				t.Fatalf("Block %+v exceed pending pool capacity %+v BUT SHOULD BE Ignore \n", shardBlock.Header.Height, len(beaconPoolTest.pendingPool))
			}
		}
	}
	for index, shardBlock := range validShardBlocks {
		if index < len(validShardBlocks) - 1 {
			shardPoolTest.validPool = append(shardPoolTest.validPool, shardBlock)
			shardPoolTest.latestValidHeight = shardBlock.Header.Height
		} else {
			err = shardPoolTest.validateShardBlock(shardBlock, false)
			if err == nil {
				t.Fatalf("Block %+v exceed valid pool capacity %+v plus pending pool capacity %+v \n", shardBlock.Header.Height, len(shardPoolTest.validPool), len(shardPoolTest.pendingPool))
			} else {
				if err.(*BlockPoolError).Code != ErrCodeMessage[MaxPoolSizeError].Code {
					t.Fatalf("Block %+v should return error %+v but get %+v", shardBlock.Header.Height, ErrCodeMessage[MaxPoolSizeError].Code, err.(*BlockPoolError).Code)
				}
			}
		}
	}
}

func TestShardPoolInsertNewShardBlockToPool(t *testing.T) {
	InitShardPoolTest(pbShardPool)
	// Condition 1: beacon best state has shard height is greater than block height
	isOk := shardPoolTest.insertNewShardBlockToPool(shardBlock2)
	if isOk {
		t.Fatalf("Block %+v is invalid with state %+v", shardBlock2.Header.Height, shardPoolTest.latestValidHeight)
	} else {
		if _, ok := shardPoolTest.pendingPool[shardBlock2.Header.Height]; !ok {
			t.Fatalf("Block %+v should be in pending pool", shardBlock2.Header.Height)
		}
	}
	// set higher best shard state
	blockchain.GetBestStateBeacon().SetBestShardHeight(0,4)
	// Condition 2: check height
	// Test Height is not equal to latestvalidheight + 1 (not expected block)
	isOk = shardPoolTest.insertNewShardBlockToPool(shardBlock3)
	if isOk {
		t.Fatalf("Block %+v is invalid with state %+v", shardBlock3.Header.Height, shardPoolTest.latestValidHeight)
	} else {
		if _, ok := shardPoolTest.pendingPool[shardBlock3.Header.Height]; !ok {
			t.Fatalf("Block %+v should be in pending pool", shardBlock3.Header.Height)
		}
	}
	delete(shardPoolTest.pendingPool, shardBlock3.Header.Height)
	for index, shardBlock := range pendingShardBlocks {
		if index < len(pendingShardBlocks) - 1 {
			shardPoolTest.pendingPool[shardBlock.Header.Height] = shardBlock
		}
	}
	// if pending list is full then block with invalid height will not get into pool (pending and valid)
	isOk = shardPoolTest.insertNewShardBlockToPool(shardBlock3)
	if isOk {
		t.Fatalf("Block %+v is invalid with state %+v", shardBlock3.Header.Height, shardPoolTest.latestValidHeight)
	} else {
		if _, ok := shardPoolTest.pendingPool[shardBlock3.Header.Height]; ok {
			t.Fatalf("Block %+v should NOT be in pending pool", shardBlock3.Header.Height)
		}
	}
	// reset valid pool and pending pool
	InitShardPoolTest(pbShardPool)
	// Test Height equal to latestvalidheight + 1 and best shard height is greater than each valid block height
	blockchain.GetBestStateBeacon().SetBestShardHeight(0, validShardBlocks[len(validShardBlocks) - 1].Header.Height+1)
	// Condition 3: Pool is full capacity -> push to pending pool
	for index, shardBlock := range validShardBlocks {
		if index < len(validShardBlocks) - 1 {
			shardPoolTest.validPool = append(shardPoolTest.validPool, shardBlock)
			shardPoolTest.latestValidHeight = shardBlock.Header.Height
		} else {
			isOk := shardPoolTest.insertNewShardBlockToPool(shardBlock)
			if isOk {
				t.Fatalf("Block %+v is valid with state %+v but pool cappacity reach max %+v", shardBlock.Header.Height, beaconPoolTest.latestValidHeight, len(beaconPoolTest.validPool))
			} else {
				if _, ok := shardPoolTest.pendingPool[shardBlock.Header.Height]; !ok {
					t.Fatalf("Block %+v should be in pending pool", shardBlock.Header.Height)
				}
			}
		}
	}
	delete(shardPoolTest.pendingPool, validShardBlocks[len(validShardBlocks)-1].Header.Height)
	for index, shardBlock := range pendingShardBlocks {
		if index < len(pendingShardBlocks) - 1 {
			shardPoolTest.pendingPool[shardBlock.Header.Height] = shardBlock
		}
	}
	// if pending list is full then block with invalid height will not get into pool (pending and valid)
	isOk = shardPoolTest.insertNewShardBlockToPool(validShardBlocks[len(validShardBlocks)-1])
	if isOk {
		t.Fatalf("Block %+v is invalid with state %+v", validShardBlocks[len(validShardBlocks)-1].Header.Height, shardPoolTest.latestValidHeight)
	} else {
		if _, ok := shardPoolTest.pendingPool[validShardBlocks[len(validShardBlocks)-1].Header.Height]; ok {
			t.Fatalf("Block %+v should NOT be in pending pool", validShardBlocks[len(validShardBlocks)-1].Header.Height)
		}
	}
	// reset valid pool and pending pool
	InitShardPoolTest(pbShardPool)
	// Condition 4: check how many block in valid pool
	// if no block in valid pool then block will be push to valid pool
	isOk = shardPoolTest.insertNewShardBlockToPool(shardBlock2)
	if !isOk {
		t.Fatalf("Block %+v is valid because no block in valid pool and it next valid state %+v", shardBlock2.Header.Height, shardPoolTest.latestValidHeight)
	} else {
		if len(shardPoolTest.validPool) != 1 {
			t.Fatalf("Valid pool should have one block")
		}
		if shardPoolTest.validPool[0].Header.Height != 2 {
			t.Fatalf("Block %+v should be in valid pool ", shardBlock2.Header.Height)
		}
	}
	// If valid pool is not empty then
	// Condition 5: current block does not point to latest block in valid pool
	// => latest block in valid pool is FORKED => discard
	// validpool has shardblock2 and latestvalidheight is 2
	isOk = shardPoolTest.insertNewShardBlockToPool(shardBlock3Forked)
	if isOk {
		t.Fatalf("Block %+v is invalid because previous block is not latest block in validpool nor in conflicted pool", shardBlock2.Header.Height)
	} else {
		if len(shardPoolTest.pendingPool) != 1 {
			t.Fatalf("Valid pool should have one block but get %+v", len(shardPoolTest.pendingPool))
		}
		if len(shardPoolTest.validPool) != 0 {
			t.Fatalf("Valid pool should have no block but get %+v", len(shardPoolTest.validPool))
		}
		if shardPoolTest.latestValidHeight != 1 {
			t.Fatalf("Latest valid height should be 1 but get %+v", shardPoolTest.latestValidHeight)
		}
		if _, isOk := shardPoolTest.pendingPool[shardBlock3Forked.Header.Height]; !isOk {
			t.Fatalf("Block %+v should be in pending pool ", shardBlock2Forked.Header.Height)
		}
	}
	// reset valid pool and pending pool
	InitShardPoolTest(pbShardPool)
	// If next block point to latest block in valid pool then accept it
	isOk = shardPoolTest.insertNewShardBlockToPool(shardBlock2Forked)
	isOk = shardPoolTest.insertNewShardBlockToPool(shardBlock3Forked)
	if !isOk {
		t.Fatalf("Block %+v should be push into valid pool", shardBlock3Forked)
	} else {
		if len(shardPoolTest.pendingPool) != 0 {
			t.Fatalf("Valid pool should have zero block but get %+v", len(shardPoolTest.pendingPool))
		}
		if len(shardPoolTest.validPool) != 2 {
			t.Fatalf("Valid pool should have 2 block but get %+v", len(shardPoolTest.validPool))
		}
		if shardPoolTest.latestValidHeight != 3 {
			t.Fatalf("Latest valid height should be 3 but get %+v", shardPoolTest.latestValidHeight)
		}
		if shardPoolTest.validPool[0].Header.Height != 2 && shardPoolTest.validPool[1].Header.Height != 3 {
			t.Fatalf("Block %+v and %+v should be in valid pool but get %+v, %+v", shardBlock2Forked.Header.Height, shardBlock3Forked.Header.Height, shardPoolTest.validPool[0].Header.Height,shardPoolTest.validPool[1].Header.Height)
		}
	}
	// reset valid pool and pending pool
	InitShardPoolTest(pbShardPool)
	isOk = shardPoolTest.insertNewShardBlockToPool(shardBlock2)
	err = shardPoolTest.validateShardBlock(shardBlock2Forked, false)
	if err == nil {
		t.Fatal("Should receive old block error but got no err")
	} else {
		if len(shardPoolTest.conflictedPool) != 1 {
			t.Fatalf("Valid pool should have 1 block but get %+v", len(shardPoolTest.conflictedPool))
		}
		if _, isOk := shardPoolTest.conflictedPool[shardBlock2Forked.Header.Hash()]; !isOk {
			t.Fatalf("Block %+v, %+v should be push into conflict pool", shardBlock2Forked.Header.Height, shardBlock2Forked.Header.Hash())
		}
	}
	isOk = shardPoolTest.insertNewShardBlockToPool(shardBlock3Forked)
	if !isOk {
		t.Fatalf("Block %+v should be push into valid pool", shardBlock3Forked.Header.Height)
	} else {
		if len(shardPoolTest.pendingPool) != 0 {
			t.Fatalf("Valid pool should have zero block but get %+v", len(shardPoolTest.pendingPool))
		}
		if len(shardPoolTest.conflictedPool) != 0 {
			t.Fatalf("Valid pool should have 0 block but get %+v", len(shardPoolTest.conflictedPool))
		}
		if len(shardPoolTest.validPool) != 2 {
			t.Fatalf("Valid pool should have 1 block but get %+v", len(shardPoolTest.validPool))
		}
		if shardPoolTest.latestValidHeight != 3 {
			t.Fatalf("Latest valid height should be 3 but get %+v", shardPoolTest.latestValidHeight)
		}
		if shardPoolTest.validPool[0].Header.Height != 2 && shardPoolTest.validPool[1].Header.Height != 3 {
			t.Fatalf("Block %+v and %+v should be in valid pool but get %+v, %+v", shardBlock2Forked.Header.Height, shardBlock3Forked.Header.Height, shardPoolTest.validPool[0].Header.Height,shardPoolTest.validPool[1].Header.Height)
		}
	}
}
//
//func TestShardPoolPromotePendingPool(t *testing.T) {
//	InitShardPoolTest(pb)
//	beaconPoolTest.pendingPool[beaconBlock2.Header.Height] = beaconBlock2
//	beaconPoolTest.pendingPool[beaconBlock3.Header.Height] = beaconBlock3
//	beaconPoolTest.pendingPool[beaconBlock4.Header.Height] = beaconBlock4
//	beaconPoolTest.pendingPool[beaconBlock5.Header.Height] = beaconBlock5
//	beaconPoolTest.pendingPool[beaconBlock6.Header.Height] = beaconBlock6
//	beaconPoolTest.promotePendingPool()
//	if len(beaconPoolTest.validPool) != 4 {
//		t.Fatalf("Shoud have 4 block in valid pool but get %+v ", len(beaconPoolTest.validPool))
//	}
//	for index, block := range beaconPoolTest.validPool {
//		switch index {
//		case 0:
//			if block.Header.Height != 2 {
//				t.Fatalf("Expect block 2 but get %+v ", block.Header.Height)
//			}
//		case 1:
//			if block.Header.Height != 3 {
//				t.Fatalf("Expect block 3 but get %+v ", block.Header.Height)
//			}
//		case 2:
//			if block.Header.Height != 4 {
//				t.Fatalf("Expect block 4 but get %+v ", block.Header.Height)
//			}
//		case 3:
//			if block.Header.Height != 5 {
//				t.Fatalf("Expect block 5 but get %+v ", block.Header.Height)
//			}
//		}
//	}
//	if len(beaconPoolTest.pendingPool) != 1 {
//		t.Fatalf("Shoud have 1 block in valid pool but get %+v ", len(beaconPoolTest.pendingPool))
//	}
//	if _, ok := beaconPoolTest.pendingPool[beaconBlock6.Header.Height]; !ok {
//		t.Fatalf("Expect Block %+v in pending pool", beaconBlock6.Header.Height)
//	}
//}
//
//func TestShardPoolAddBeaconBlock(t *testing.T) {
//	InitShardPoolTest(pb)
//	beaconPoolTest.SetBeaconState(testLatestValidHeight)
//	for _, block := range validShardBlocks {
//		err := beaconPoolTest.AddBeaconBlock(block)
//		if err != nil {
//			t.Fatalf("Block %+v should be added into pool but get %+v", block.Header.Height, err )
//		}
//	}
//	if len(beaconPoolTest.validPool) != MAX_VALID_BEACON_BLK_IN_POOL {
//		t.Fatalf("Expected number of block %+v in valid pool but get %+v", MAX_VALID_BEACON_BLK_IN_POOL, len(beaconPoolTest.validPool))
//	}
//	if len(beaconPoolTest.pendingPool) != 1 {
//		t.Fatalf("Expected number of block %+v in pending pool but get %+v", 1, len(beaconPoolTest.pendingPool))
//	}
//	if _, isOk := beaconPoolTest.pendingPool[validShardBlocks[len(validShardBlocks)-1].Header.Height]; !isOk {
//		t.Fatalf("Expect block %+v to be in pending pool", validShardBlocks[len(validShardBlocks)-1].Header.Height)
//	}
//	delete(beaconPoolTest.pendingPool, validShardBlocks[len(validShardBlocks)-1].Header.Height)
//	for index, block := range pendingShardBlocks {
//		if index < len(pendingShardBlocks) - 1 {
//			err := beaconPoolTest.AddBeaconBlock(block)
//			if err != nil {
//				t.Fatalf("Block %+v should be added into pool but get %+v", block.Header.Height, err)
//			}
//		} else {
//			err := beaconPoolTest.AddBeaconBlock(block)
//			if err == nil {
//				t.Fatalf("Block %+v should NOT be added into pool but get no error", block.Header.Height)
//			} else {
//				if err.(*BlockPoolError).Code != ErrCodeMessage[MaxPoolSizeError].Code {
//					t.Fatalf("Expect err %+v but get %+v", ErrCodeMessage[MaxPoolSizeError], err)
//				}
//			}
//		}
//	}
//	if len(beaconPoolTest.pendingPool) != MAX_PENDING_BEACON_BLK_IN_POOL {
//		t.Fatalf("Expected number of block %+v in pending pool but get %+v", MAX_PENDING_BEACON_BLK_IN_POOL, len(beaconPoolTest.pendingPool))
//	}
//}
//func TestShardPoolUpdateLatestBeaconState(t *testing.T) {
//	InitShardPoolTest(pb)
//	// init state of latestvalidheight
//	if beaconPoolTest.latestValidHeight != 1 {
//		t.Fatalf("Expect to latestvalidheight is 1 but get %+v", beaconPoolTest.latestValidHeight)
//	}
//	// with no block and no blockchain state => latestvalidheight is 0
//	beaconPoolTest.updateLatestBeaconState()
//	if beaconPoolTest.latestValidHeight != 0 {
//		t.Fatalf("Expect to latestvalidheight is 0 but get %+v", beaconPoolTest.latestValidHeight)
//	}
//	// if valid block list is not empty then each time update latest state
//	// it will set to the height of last block in valid block list
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock2)
//	beaconPoolTest.updateLatestBeaconState()
//	if beaconPoolTest.latestValidHeight != 2 {
//		t.Fatalf("Expect to latestvalidheight is 0 but get %+v", beaconPoolTest.latestValidHeight)
//	}
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock3)
//	beaconPoolTest.updateLatestBeaconState()
//	if beaconPoolTest.latestValidHeight != 3 {
//		t.Fatalf("Expect to latestvalidheight is 0 but get %+v", beaconPoolTest.latestValidHeight)
//	}
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock4)
//	beaconPoolTest.updateLatestBeaconState()
//	if beaconPoolTest.latestValidHeight != 4 {
//		t.Fatalf("Expect to latestvalidheight is 0 but get %+v", beaconPoolTest.latestValidHeight)
//	}
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock5)
//	beaconPoolTest.updateLatestBeaconState()
//	if beaconPoolTest.latestValidHeight != 5 {
//		t.Fatalf("Expect to latestvalidheight is 0 but get %+v", beaconPoolTest.latestValidHeight)
//	}
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock6)
//	beaconPoolTest.updateLatestBeaconState()
//	if beaconPoolTest.latestValidHeight != 6 {
//		t.Fatalf("Expect to latestvalidheight is 0 but get %+v", beaconPoolTest.latestValidHeight)
//	}
//	beaconPoolTest.validPool = []*blockchain.BeaconBlock{}
//	beaconPoolTest.updateLatestBeaconState()
//	if beaconPoolTest.latestValidHeight != 0 {
//		t.Fatalf("Expect to latestvalidheight is 0 but get %+v", beaconPoolTest.latestValidHeight)
//	}
//}
//func TestShardPoolRemoveBlock(t *testing.T) {
//	InitShardPoolTest(pb)
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock2)
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock3)
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock4)
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock5)
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock6)
//	// remove VALID block according to latestblockheight input
//	// also update latest valid height after call this function
//	beaconPoolTest.removeBlock(4)
//	if len(beaconPoolTest.validPool) != 2 {
//		t.Fatalf("Expect to get only 2 block left but got %+v", len(beaconPoolTest.validPool))
//	}
//	if beaconPoolTest.latestValidHeight != 6 {
//		t.Fatalf("Expect to latestvalidheight is 6 but get %+v", beaconPoolTest.latestValidHeight)
//	}
//	beaconPoolTest.removeBlock(6)
//	if len(beaconPoolTest.validPool) != 0 {
//		t.Fatalf("Expect to have NO block left but got %+v", len(beaconPoolTest.validPool))
//	}
//	// because no block left in valid pool and blockchain state is 0 so latest valid state should be 0
//	if beaconPoolTest.latestValidHeight != 0 {
//		t.Fatalf("Expect to latestvalidheight is 0 but get %+v", beaconPoolTest.latestValidHeight)
//	}
//}
//func TestShardPoolCleanOldBlock(t *testing.T) {
//	InitShardPoolTest(pb)
//	if len(beaconPoolTest.pendingPool) != 0 {
//		t.Fatalf("Expected number of block 0 in pending pool but get %+v", len(beaconPoolTest.pendingPool))
//	}
//	if len(beaconPoolTest.conflictedPool) != 0 {
//		t.Fatalf("Expected number of block 0 in pending pool but get %+v", len(beaconPoolTest.conflictedPool))
//	}
//	// clean OLD block in Pending and Conflict pool
//	// old block in pending pool has height < latestvalidheight
//	// old block in conflicted pool has height < latestvalidheight - 2
//	beaconPoolTest.pendingPool[beaconBlock2.Header.Height] = beaconBlock2
//	beaconPoolTest.pendingPool[beaconBlock3.Header.Height] = beaconBlock3
//	beaconPoolTest.conflictedPool[beaconBlock3Forked.Header.Hash()] = beaconBlock3Forked
//	beaconPoolTest.pendingPool[beaconBlock4.Header.Height] = beaconBlock4
//	beaconPoolTest.pendingPool[beaconBlock5.Header.Height] = beaconBlock5
//	beaconPoolTest.pendingPool[beaconBlock6.Header.Height] = beaconBlock6
//	if len(beaconPoolTest.pendingPool) != 5 {
//		t.Fatalf("Expected number of block 5 in pending pool but get %+v", len(beaconPoolTest.pendingPool))
//	}
//	if len(beaconPoolTest.conflictedPool) != 1 {
//		t.Fatalf("Expected number of block 1 in pending pool but get %+v", len(beaconPoolTest.conflictedPool))
//	}
//	beaconPoolTest.CleanOldBlock(2)
//	if len(beaconPoolTest.pendingPool) != 4 {
//		t.Fatalf("Expected number of block 4 in pending pool but get %+v", len(beaconPoolTest.pendingPool))
//	}
//	if len(beaconPoolTest.conflictedPool) != 1 {
//		t.Fatalf("Expected number of block 1 in pending pool but get %+v", len(beaconPoolTest.conflictedPool))
//	}
//	beaconPoolTest.CleanOldBlock(3)
//	if len(beaconPoolTest.pendingPool) != 3 {
//		t.Fatalf("Expected number of block 3 in pending pool but get %+v", len(beaconPoolTest.pendingPool))
//	}
//	if len(beaconPoolTest.conflictedPool) != 1 {
//		t.Fatalf("Expected number of block 1 in pending pool but get %+v", len(beaconPoolTest.conflictedPool))
//	}
//	beaconPoolTest.CleanOldBlock(5)
//	if len(beaconPoolTest.conflictedPool) != 1 {
//		t.Fatalf("Expected number of block 1 in pending pool but get %+v", len(beaconPoolTest.conflictedPool))
//	}
//	beaconPoolTest.CleanOldBlock(6)
//	if len(beaconPoolTest.pendingPool) != 0 {
//		t.Fatalf("Expected number of block 0 in pending pool but get %+v", len(beaconPoolTest.pendingPool))
//	}
//	if len(beaconPoolTest.conflictedPool) != 0 {
//		t.Fatalf("Expected number of block 0 in pending pool but get %+v", len(beaconPoolTest.conflictedPool))
//	}
//}
//func TestShardPoolGetValidBlock(t *testing.T) {
//	InitShardPoolTest(pb)
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock2)
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock3)
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock4)
//	beaconPoolTest.validPool = append(beaconPoolTest.validPool, beaconBlock5)
//	beaconPoolTest.updateLatestBeaconState()
//	beaconPoolTest.pendingPool[beaconBlock6.Header.Height] = beaconBlock6
//	beaconPoolTest.pendingPool[beaconBlock7.Header.Height] = beaconBlock7
//	// no role in committee then return only valid pool
//	beaconPoolTest.RoleInCommittees = false
//	validShardBlocks := beaconPoolTest.GetValidBlock()
//	if len(validShardBlocks) != 4 {
//		t.Fatalf("Expect return 4 blocks but get %+v", len(validShardBlocks))
//	}
//	for _, block := range validShardBlocks {
//		if block.Header.Height == beaconBlock6.Header.Height {
//			t.Fatal("Return block height 6 should not have block in pending pool")
//		}
//	}
//	// if has role in beacon committee then return valid pool
//	// IF VALID POOL IS EMPTY ONLY return one block from pending pool if condition is match
//	// - Condition: block with height = latestvalidheight + 1 (if present) in pending poool
//	beaconPoolTest.RoleInCommittees = true
//	// if beacon pool in committee but valid block list not empty then return NO block from pending pool
//	validAnd0PendingBlocks := beaconPoolTest.GetValidBlock()
//	if len(validAnd0PendingBlocks) != 4 {
//		t.Fatalf("Expect return 4 blocks but get %+v", len(validAnd0PendingBlocks))
//	}
//	for _, block := range validAnd0PendingBlocks {
//		if block.Header.Height == beaconBlock6.Header.Height {
//			t.Fatal("Return block height 6 should not have block in pending pool")
//		}
//		if block.Header.Height == beaconBlock7.Header.Height {
//			t.Fatal("Return block height 7 should not have block in pending pool")
//		}
//	}
//	// if beacon pool in committee but valid block list IS EMPTY
//	// then return ONLY 1 block from pending pool that match condition (see above)
//	beaconPoolTest.validPool = []*blockchain.BeaconBlock{}
//	oneValidFromPendingBlocks := beaconPoolTest.GetValidBlock()
//	if len(oneValidFromPendingBlocks) != 1 {
//		t.Fatalf("Expect return 1 blocks but get %+v", len(oneValidFromPendingBlocks))
//	}
//	if oneValidFromPendingBlocks[0].Header.Height != beaconBlock6.Header.Height {
//		t.Fatalf("Expect return block height 6 but get %+v", oneValidFromPendingBlocks[0].Header.Height)
//	}
//	if oneValidFromPendingBlocks[0].Header.Height == beaconBlock7.Header.Height {
//		t.Fatalf("DONT expect return block height 7 but get %+v", oneValidFromPendingBlocks[0].Header.Height)
//	}
//}
