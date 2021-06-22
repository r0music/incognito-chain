package types

import (
	"errors"
	"fmt"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"log"
	"sort"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/transaction"
)

type CrossShardBlock struct {
	ValidationData  string `json:"ValidationData"`
	Header          ShardHeader
	ToShardID       byte
	MerklePathShard []common.Hash
	// Cross Shard data for PRV
	CrossOutputCoin []privacy.Coin
	// Cross Shard For Custom token privacy
	CrossTxTokenPrivacyData []ContentCrossShardTokenPrivacyData
}

func NewCrossShardBlock() *CrossShardBlock {
	return &CrossShardBlock{}
}

func (crossShardBlock CrossShardBlock) CommitteeFromBlock() common.Hash {
	return common.Hash{}
}

func (crossShardBlock CrossShardBlock) GetProposer() string {
	return crossShardBlock.Header.Proposer
}

func (crossShardBlock CrossShardBlock) GetProposeTime() int64 {
	return crossShardBlock.Header.ProposeTime
}

func (crossShardBlock CrossShardBlock) GetProduceTime() int64 {
	return crossShardBlock.Header.Timestamp
}

func (crossShardBlock CrossShardBlock) GetCurrentEpoch() uint64 {
	return crossShardBlock.Header.Epoch
}

func (crossShardBlock CrossShardBlock) GetPrevHash() common.Hash {
	return crossShardBlock.Header.PreviousBlockHash
}

func (crossShardBlock *CrossShardBlock) Hash() *common.Hash {
	hash := crossShardBlock.Header.Hash()
	return &hash
}

func (block CrossShardBlock) GetProducer() string {
	return block.Header.Producer
}

func (block CrossShardBlock) GetVersion() int {
	return block.Header.Version
}

func (block CrossShardBlock) GetHeight() uint64 {
	return block.Header.Height
}

func (block CrossShardBlock) GetShardID() int {
	return int(block.Header.ShardID)
}

func (block CrossShardBlock) GetValidationField() string {
	return block.ValidationData
}

func (block CrossShardBlock) GetRound() int {
	return block.Header.Round
}

func (block CrossShardBlock) GetRoundKey() string {
	return fmt.Sprint(block.Header.Height, "_", block.Header.Round)
}

func (block CrossShardBlock) GetInstructions() [][]string {
	return [][]string{}
}

func (block ShardBlock) GetConsensusType() string {
	return block.Header.ConsensusType
}

func (block CrossShardBlock) GetConsensusType() string {
	return block.Header.ConsensusType
}

func CreateAllCrossShardBlock(shardBlock *ShardBlock, activeShards int) map[byte]*CrossShardBlock {
	allCrossShard := make(map[byte]*CrossShardBlock)
	if activeShards == 1 {
		return allCrossShard
	}
	for i := 0; i < activeShards; i++ {
		shardID := common.GetShardIDFromLastByte(byte(i))
		if shardID != shardBlock.Header.ShardID {
			crossShard, err := CreateCrossShardBlock(shardBlock, shardID)
			if crossShard != nil {
				log.Printf("Create CrossShardBlock from Shard %+v to Shard %+v: %+v \n", shardBlock.Header.ShardID, shardID, crossShard)
			}
			if crossShard != nil && err == nil {
				allCrossShard[byte(i)] = crossShard
			}
		}
	}
	return allCrossShard
}

func CreateCrossShardBlock(shardBlock *ShardBlock, shardID byte) (*CrossShardBlock, error) {
	crossShard := &CrossShardBlock{}
	crossOutputCoin, crossCustomTokenPrivacyData, err := getCrossShardData(shardBlock.Body.Transactions, shardID)
	if err != nil {
		return nil, err
	}
	// Return nothing if nothing to cross
	if len(crossOutputCoin) == 0 && len(crossCustomTokenPrivacyData) == 0 {
		return nil, errors.New("No cross Outputcoin, Cross Custom Token, Cross Custom Token Privacy")
	}
	merklePathShard, merkleShardRoot := GetMerklePathCrossShard(shardBlock.Body.Transactions, shardID)
	if merkleShardRoot != shardBlock.Header.ShardTxRoot {
		return crossShard, fmt.Errorf("Expect Shard Tx Root To be %+v but get %+v", shardBlock.Header.ShardTxRoot, merkleShardRoot)
	}
	crossShard.ValidationData = shardBlock.ValidationData
	crossShard.Header = shardBlock.Header
	crossShard.MerklePathShard = merklePathShard
	crossShard.CrossOutputCoin = crossOutputCoin
	crossShard.CrossTxTokenPrivacyData = crossCustomTokenPrivacyData
	crossShard.ToShardID = shardID
	return crossShard, nil
}

// getCrossShardData get cross data (send to a shard) from list of transaction:
// 1. (Privacy) PRV: Output coin
// 2. Tx Custom Token: Tx Token Data
// 3. Privacy Custom Token: Token Data + Output coin
func getCrossShardData(txList []metadata.Transaction, shardID byte) ([]privacy.Coin, []ContentCrossShardTokenPrivacyData, error) {
	coinList := []coin.Coin{}
	txTokenPrivacyDataMap := make(map[common.Hash]*ContentCrossShardTokenPrivacyData)
	var txTokenPrivacyDataList []ContentCrossShardTokenPrivacyData
	for _, tx := range txList {
		var prvProof privacy.Proof

		if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
			customTokenPrivacyTx, ok := tx.(transaction.TransactionToken)
			if !ok {
				return nil, nil, errors.New("Cannot cast transaction")
			}
			prvProof = customTokenPrivacyTx.GetTxBase().GetProof()
			txTokenData := customTokenPrivacyTx.GetTxTokenData()
			txTokenProof := txTokenData.TxNormal.GetProof()
			if txTokenProof != nil {
				for _, outCoin := range txTokenProof.GetOutputCoins() {
					coinShardID, err := outCoin.GetShardID()
					if err == nil && coinShardID == shardID {
						if _, ok := txTokenPrivacyDataMap[txTokenData.PropertyID]; !ok {
							contentCrossTokenPrivacyData := CloneTxTokenPrivacyDataForCrossShard(txTokenData)
							txTokenPrivacyDataMap[txTokenData.PropertyID] = &contentCrossTokenPrivacyData
						}
						txTokenPrivacyDataMap[txTokenData.PropertyID].OutputCoin = append(txTokenPrivacyDataMap[txTokenData.PropertyID].OutputCoin, outCoin)
					}
				}
			}
		} else {
			prvProof = tx.GetProof()
		}
		if prvProof != nil {
			for _, outCoin := range prvProof.GetOutputCoins() {
				coinShardID, err := outCoin.GetShardID()
				if err == nil && coinShardID == shardID {
					coinList = append(coinList, outCoin)
				}
			}
		}
	}
	if len(txTokenPrivacyDataMap) != 0 {
		for _, value := range txTokenPrivacyDataMap {
			txTokenPrivacyDataList = append(txTokenPrivacyDataList, *value)
		}
		sort.SliceStable(txTokenPrivacyDataList[:], func(i, j int) bool {
			return txTokenPrivacyDataList[i].PropertyID.String() < txTokenPrivacyDataList[j].PropertyID.String()
		})
	}
	return coinList, txTokenPrivacyDataList, nil
}

// VerifyCrossShardBlockUTXO Calculate Final Hash as Hash of:
//	1. CrossTransactionFinalHash
//	2. TxTokenDataVoutFinalHash
//	3. CrossTxTokenPrivacyData
// These hashes will be calculated as comment in getCrossShardDataHash function
func VerifyCrossShardBlockUTXO(block *CrossShardBlock) bool {
	var outputCoinHash common.Hash
	var txTokenDataHash common.Hash
	var txTokenPrivacyDataHash common.Hash
	outCoins := block.CrossOutputCoin
	outputCoinHash = calHashOutCoinCrossShard(outCoins)
	txTokenDataHash = calHashTxTokenDataHashList()
	txTokenPrivacyDataList := block.CrossTxTokenPrivacyData
	txTokenPrivacyDataHash = calHashTxTokenPrivacyDataHashList(txTokenPrivacyDataList)
	tmpByte := append(append(outputCoinHash.GetBytes(), txTokenDataHash.GetBytes()...), txTokenPrivacyDataHash.GetBytes()...)
	finalHash := common.HashH(tmpByte)
	return Merkle{}.VerifyMerkleRootFromMerklePath(finalHash, block.MerklePathShard, block.Header.ShardTxRoot, block.ToShardID)
}

func calHashOutCoinCrossShard(outCoins []privacy.Coin) common.Hash {
	tmpByte := []byte{}
	var outputCoinHash common.Hash
	if len(outCoins) != 0 {
		for _, outCoin := range outCoins {
			tmpByte = append(tmpByte, outCoin.Bytes()...)
		}
		outputCoinHash = common.HashH(tmpByte)
	} else {
		outputCoinHash = common.HashH([]byte(""))
	}
	return outputCoinHash
}

func calHashTxTokenDataHashList() common.Hash {
	return common.HashH([]byte(""))
}
