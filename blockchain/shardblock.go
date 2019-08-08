package blockchain

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/transaction"
)

type ShardBlock struct {
	AggregatedSig   string  `json:"AggregatedSig"`
	R               string  `json:"R"`
	ValidatorsIndex [][]int `json:"ValidatorsIndex"` //[0]: R | [1]:AggregatedSig
	ProducerSig     string  `json:"ProducerSig"`
	Body            ShardBody
	Header          ShardHeader
}

type ShardToBeaconBlock struct {
	AggregatedSig   string  `json:"AggregatedSig"`
	R               string  `json:"R"`
	ValidatorsIndex [][]int `json:"ValidatorsIndex"` //[0]: R | [1]:AggregatedSig
	ProducerSig     string  `json:"ProducerSig"`

	Instructions [][]string
	Header       ShardHeader
}

type CrossShardBlock struct {
	AggregatedSig   string  `json:"AggregatedSig"`
	R               string  `json:"R"`
	ValidatorsIndex [][]int `json:"ValidatorsIndex"` //[0]: R | [1]:AggregatedSig
	ProducerSig     string  `json:"ProducerSig"`
	Header          ShardHeader
	ToShardID       byte
	MerklePathShard []common.Hash
	// Cross Shard data for PRV
	CrossOutputCoin []privacy.OutputCoin
	// Cross Shard Data for Custom Token Tx
	CrossTxTokenData []transaction.TxTokenData
	// Cross Shard For Custom token privacy
	CrossTxTokenPrivacyData []ContentCrossShardTokenPrivacyData
}

func NewShardBlock() *ShardBlock {
	return &ShardBlock{
		Header: ShardHeader{},
		Body: ShardBody{
			Instructions:      [][]string{},
			CrossTransactions: make(map[byte][]CrossTransaction),
			Transactions:      make([]metadata.Transaction, 0),
		},
	}
}
func NewShardBlockWithHeader(header ShardHeader) *ShardBlock {
	return &ShardBlock{
		Header: header,
		Body: ShardBody{
			Instructions:      [][]string{},
			CrossTransactions: make(map[byte][]CrossTransaction),
			Transactions:      make([]metadata.Transaction, 0),
		},
	}
}
func NewShardBlockWithBody(body ShardBody) *ShardBlock {
	return &ShardBlock{
		Header: ShardHeader{},
		Body:   body,
	}
}
func NewShardBlockFull(header ShardHeader, body ShardBody) *ShardBlock {
	return &ShardBlock{
		Header: header,
		Body:   body,
	}
}
func (shardBlock *ShardBlock) BuildShardBlockBody(instructions [][]string, crossTransaction map[byte][]CrossTransaction, transactions []metadata.Transaction) {
	shardBlock.Body.Instructions = append(shardBlock.Body.Instructions, instructions...)
	shardBlock.Body.CrossTransactions = crossTransaction
	shardBlock.Body.Transactions = append(shardBlock.Body.Transactions, transactions...)
}
func (crossShardBlock *CrossShardBlock) Hash() *common.Hash {
	hash := crossShardBlock.Header.Hash()
	return &hash
}

func (shardToBeaconBlock *ShardToBeaconBlock) Hash() *common.Hash {
	hash := shardToBeaconBlock.Header.Hash()
	return &hash
}

func (shardBlock *ShardBlock) Hash() *common.Hash {
	hash := shardBlock.Header.Hash()
	return &hash
}
func (shardBlock *ShardBlock) validateSanityData() (bool, error) {
	//Check Header
	if shardBlock.Header.Height == 1 && len(shardBlock.Header.ProducerAddress.Bytes()) != 0 {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block with Height 1 Producer Address have %+v bytes but get %+v bytes", 0, len(shardBlock.Header.ProducerAddress.Bytes())))
	}
	// producer address must have 66 bytes: 33-byte public key, 33-byte transmission key
	if shardBlock.Header.Height > 1 && len(shardBlock.Header.ProducerAddress.Bytes()) != 66 {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Producer Address have %+v bytes but get %+v bytes", 66, len(shardBlock.Header.ProducerAddress.Bytes())))
	}
	if int(shardBlock.Header.ShardID) < 0 || int(shardBlock.Header.ShardID) > 256 {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block ShardID in range 0 - 255 but get %+v ", shardBlock.Header.ShardID))
	}
	if shardBlock.Header.Version < SHARD_BLOCK_VERSION {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Version greater or equal than %+v but get %+v ", SHARD_BLOCK_VERSION, shardBlock.Header.Version))
	}
	if len(shardBlock.Header.PreviousBlockHash[:]) != common.HashSize {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Previous Hash in the right format"))
	}
	if shardBlock.Header.Height < 1 {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Height to be greater than 0"))
	}
	if shardBlock.Header.Height == 1 && !shardBlock.Header.PreviousBlockHash.IsEqual(&common.Hash{}) {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block with Height 1 (first block) have Zero Hash Value"))
	}
	if shardBlock.Header.Height > 1 && shardBlock.Header.PreviousBlockHash.IsEqual(&common.Hash{}) {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block with Height greater than 1 have Non-Zero Hash Value"))
	}
	if shardBlock.Header.Round < 1 {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Round greater or equal than 1"))
	}
	if shardBlock.Header.Epoch < 1 {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Epoch greater or equal than 1"))
	}
	if shardBlock.Header.Timestamp < 0 {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Epoch greater or equal than 0"))
	}
	if len(shardBlock.Header.TxRoot[:]) != common.HashSize {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Tx Root in the right format"))
	}
	if len(shardBlock.Header.ShardTxRoot[:]) != common.HashSize {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Shard Tx Root in the right format"))
	}
	if len(shardBlock.Header.CrossTransactionRoot[:]) != common.HashSize {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Cross Transaction Root in the right format"))
	}
	if len(shardBlock.Header.InstructionsRoot[:]) != common.HashSize {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Instructions Root in the right format"))
	}
	if len(shardBlock.Header.CommitteeRoot[:]) != common.HashSize {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Committee Root in the right format"))
	}
	if shardBlock.Header.Height == 1 && !shardBlock.Header.CommitteeRoot.IsEqual(&common.Hash{}) {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block with Height 1 have Zero Hash Value"))
	}
	if shardBlock.Header.Height > 1 && shardBlock.Header.CommitteeRoot.IsEqual(&common.Hash{}) {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block with Height greater than 1 have Non-Zero Hash Value"))
	}
	if len(shardBlock.Header.PendingValidatorRoot[:]) != common.HashSize {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Committee Root in the right format"))
	}
	if len(shardBlock.Header.CrossShardBitMap) > 254 {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Cross Shard Length Less Than 255"))
	}
	if shardBlock.Header.BeaconHeight < 1 {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block has Beacon Height greater or equal than 1"))
	}
	//if shardBlock.Header.BeaconHeight == 1 && !shardBlock.Header.BeaconHash.IsEqual(&common.Hash{}) {
	//	return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block with Beacon Height 1 have Zero Hash Value"))
	//}
	if shardBlock.Header.BeaconHeight > 1 && shardBlock.Header.BeaconHash.IsEqual(&common.Hash{}) {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block with Beacon Height greater or equal than 1 have Non-Zero Hash Value"))
	}
	if shardBlock.Header.TotalTxsFee == nil {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Total Txs Fee have nil value"))
	}
	if len(shardBlock.Header.InstructionMerkleRoot[:]) != common.HashSize {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Instruction Merkle Root in the right format"))
	}
	// body
	if shardBlock.Body.Instructions == nil {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Instruction is not nil"))
	}
	if len(shardBlock.Body.Instructions) != 0 && shardBlock.Header.InstructionMerkleRoot.IsEqual(&common.Hash{}) {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Instruction Merkle Root have Non-Zero Hash Value because Instrucstion List is not empty"))
	}
	if shardBlock.Body.CrossTransactions == nil {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Cross Transactions Map is not nil"))
	}
	if len(shardBlock.Body.CrossTransactions) != 0 && shardBlock.Header.CrossTransactionRoot.IsEqual(&common.Hash{}) {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Cross Transaction Root have Non-Zero Hash Value because Cross Transaction List is not empty"))
	}
	if shardBlock.Body.Transactions == nil {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Transactions is not nil"))
	}
	if len(shardBlock.Body.Transactions) != 0 && shardBlock.Header.TxRoot.IsEqual(&common.Hash{}) {
		return false, NewBlockChainError(ShardBlockSanityError, fmt.Errorf("Expect Shard Block Tx Root have Non-Zero Hash Value because Transactions List is not empty"))
	}
	return true, nil
}

func (shardBlock *ShardBlock) UnmarshalJSON(data []byte) error {
	tempBlk := &struct {
		AggregatedSig string  `json:"AggregatedSig"`
		R             string  `json:"R"`
		ValidatorsIdx [][]int `json:"ValidatorsIndex"`
		ProducerSig   string  `json:"ProducerSig"`
		Header        ShardHeader
		Body          *json.RawMessage
	}{}
	err := json.Unmarshal(data, &tempBlk)
	if err != nil {
		return NewBlockChainError(UnmashallJsonShardBlockError, err)
	}
	shardBlock.AggregatedSig = tempBlk.AggregatedSig
	shardBlock.R = tempBlk.R
	shardBlock.ValidatorsIndex = tempBlk.ValidatorsIdx
	shardBlock.ProducerSig = tempBlk.ProducerSig
	blkBody := ShardBody{}
	err = blkBody.UnmarshalJSON(*tempBlk.Body)
	if err != nil {
		return NewBlockChainError(UnmashallJsonShardBlockError, err)
	}
	shardBlock.Header = tempBlk.Header
	// Init shard block data if get nil value
	if shardBlock.Body.Transactions == nil {
		shardBlock.Body.Transactions = []metadata.Transaction{}
	}
	if shardBlock.Body.Instructions == nil {
		shardBlock.Body.Instructions = [][]string{}
	}
	if shardBlock.Body.CrossTransactions == nil {
		shardBlock.Body.CrossTransactions = make(map[byte][]CrossTransaction)
	}
	if shardBlock.Header.TotalTxsFee == nil {
		shardBlock.Header.TotalTxsFee = make(map[common.Hash]uint64)
	}
	if ok, err := shardBlock.validateSanityData(); !ok || err != nil {
		return NewBlockChainError(UnmashallJsonShardBlockError, err)
	}
	shardBlock.Body = blkBody
	return nil
}

// /*
// AddTransaction adds a new transaction into block
// */
// // #1 - tx
func (shardBlock *ShardBlock) AddTransaction(tx metadata.Transaction) error {
	if shardBlock.Body.Transactions == nil {
		return NewBlockChainError(UnExpectedError, errors.New("not init tx arrays"))
	}
	shardBlock.Body.Transactions = append(shardBlock.Body.Transactions, tx)
	return nil
}

// func (shardBlock *ShardBlock) VerifyBlockReward(blockchain *BlockChain) error {
// 	hasBlockReward := false
// 	txsFee := uint64(0)
// 	for _, tx := range shardBlock.Body.Transactions {
// 		if tx.GetMetadataType() == metadata.ShardBlockReward {
// 			if hasBlockReward {
// 				return errors.New("This block contains more than one coinbase transaction for shard block producer!")
// 			}
// 			hasBlockReward = true
// 		} else {
// 			txsFee += tx.GetTxFee()
// 		}
// 	}
// 	if !hasBlockReward {
// 		return errors.New("This block dont have coinbase tx for shard block producer")
// 	}
// 	numberOfTxs := len(shardBlock.Body.Transactions)
// 	if shardBlock.Body.Transactions[numberOfTxs-1].GetMetadataType() != metadata.ShardBlockReward {
// 		return errors.New("Coinbase transaction must be the last transaction")
// 	}

// 	receivers, values := shardBlock.Body.Transactions[numberOfTxs-1].GetReceivers()
// 	if len(receivers) != 1 {
// 		return errors.New("Wrong receiver")
// 	}
// 	if !common.ByteEqual(receivers[0], shardBlock.Header.ProducerAddress.Pk) {
// 		return errors.New("Wrong receiver")
// 	}
// 	reward := blockchain.getRewardAmount(shardBlock.Header.Height)
// 	reward += txsFee
// 	if reward != values[0] {
// 		return errors.New("Wrong reward value")
// 	}
// 	return nil
// }

func (blk *ShardBlock) CreateShardToBeaconBlock(bc *BlockChain) *ShardToBeaconBlock {
	if bc.IsTest {
		return &ShardToBeaconBlock{}
	}
	block := ShardToBeaconBlock{}
	block.AggregatedSig = blk.AggregatedSig

	block.ValidatorsIndex = make([][]int, 2)                                               //multi-node
	block.ValidatorsIndex[0] = append(block.ValidatorsIndex[0], blk.ValidatorsIndex[0]...) //multi-node
	block.ValidatorsIndex[1] = append(block.ValidatorsIndex[1], blk.ValidatorsIndex[1]...) //multi-node

	block.R = blk.R
	block.ProducerSig = blk.ProducerSig
	block.Header = blk.Header
	block.Instructions = blk.Body.Instructions
	previousShardBlockByte, err := bc.config.DataBase.FetchBlock(blk.Header.PreviousBlockHash)
	if err != nil {
		Logger.log.Error(err)
		return nil
	}
	previousShardBlock := ShardBlock{}
	err = json.Unmarshal(previousShardBlockByte, &previousShardBlock)
	if err != nil {
		Logger.log.Error(err)
		return nil
	}
	//beaconBlocks, err := FetchBeaconBlockFromHeight(bc.config.DataBase, previousShardBlock.Header.BeaconHeight+1, block.Header.BeaconHeight)
	//if err != nil {
	//	Logger.log.Error(err)
	//	return nil
	//}
	instructions, err := CreateShardInstructionsFromTransactionAndInstruction(blk.Body.Transactions, bc, blk.Header.ShardID)
	if err != nil {
		Logger.log.Error(err)
		return nil
	}

	block.Instructions = append(block.Instructions, instructions...)
	return &block
}

func (blk *ShardBlock) CreateAllCrossShardBlock(activeShards int) map[byte]*CrossShardBlock {
	allCrossShard := make(map[byte]*CrossShardBlock)
	if activeShards == 1 {
		return allCrossShard
	}
	for i := 0; i < activeShards; i++ {
		shardID := common.GetShardIDFromLastByte(byte(i))
		if shardID != blk.Header.ShardID {
			crossShard, err := blk.CreateCrossShardBlock(shardID)
			if crossShard != nil {
				Logger.log.Infof("Create CrossShardBlock from Shard %+v to Shard %+v: %+v \n", blk.Header.ShardID, shardID, crossShard)
			}
			if crossShard != nil && err == nil {
				allCrossShard[byte(i)] = crossShard
			}
		}
	}
	return allCrossShard
}

func (block *ShardBlock) CreateCrossShardBlock(shardID byte) (*CrossShardBlock, error) {
	crossShard := &CrossShardBlock{}
	crossOutputCoin, crossTxTokenData, crossCustomTokenPrivacyData := getCrossShardData(block.Body.Transactions, shardID)
	//fmt.Println("CS:", len(crossOutputCoin), len(crossTxTokenData), len(crossCustomTokenPrivacyData))
	// Return nothing if nothing to cross
	if len(crossOutputCoin) == 0 && len(crossTxTokenData) == 0 && len(crossCustomTokenPrivacyData) == 0 {
		//fmt.Println("CreateCrossShardBlock no crossshard", block.Header.Height)
		return nil, NewBlockChainError(CrossShardBlockError, errors.New("No cross outputcoin"))
	}
	merklePathShard, merkleShardRoot := GetMerklePathCrossShard2(block.Body.Transactions, shardID)
	if merkleShardRoot != block.Header.ShardTxRoot {
		return crossShard, NewBlockChainError(CrossShardBlockError, errors.New("ShardTxRoot mismatch"))
	}
	//Copy signature and header
	crossShard.AggregatedSig = block.AggregatedSig

	crossShard.ValidatorsIndex = make([][]int, 2)                                                      //multi-node
	crossShard.ValidatorsIndex[0] = append(crossShard.ValidatorsIndex[0], block.ValidatorsIndex[0]...) //multi-node
	crossShard.ValidatorsIndex[1] = append(crossShard.ValidatorsIndex[1], block.ValidatorsIndex[1]...) //multi-node

	crossShard.R = block.R
	crossShard.ProducerSig = block.ProducerSig
	crossShard.Header = block.Header
	crossShard.MerklePathShard = merklePathShard
	crossShard.CrossOutputCoin = crossOutputCoin
	crossShard.CrossTxTokenData = crossTxTokenData
	crossShard.CrossTxTokenPrivacyData = crossCustomTokenPrivacyData
	crossShard.ToShardID = shardID
	return crossShard, nil
}

// func (block *ShardBlock) getBlockRewardInst(blockHeight uint64) ([]string, error) {
// 	txsFee := uint64(0)

// 	for _, tx := range block.Body.Transactions {
// 		txsFee += tx.GetTxFee()
// 	}
// 	blkRewardInfo := metadata.NewBlockRewardInfo(txsFee, blockHeight)
// 	inst, err := blkRewardInfo.GetStringFormat()
// 	return inst, err
// }
