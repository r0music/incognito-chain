package jsonresult

import "github.com/incognitochain/incognito-chain/blockchain"

type GetBestBlockResult struct {
	BestBlocks map[int]GetBestBlockItem `json:"BestBlocks"`
}

type GetBestBlockItem struct {
	Height           uint64 `json:"Height"`
	Hash             string `json:"Hash"`
	TotalTxs         uint64 `json:"TotalTxs"`
	BlockProducer    string `json:"BlockProducer"`
	BlockProducerSig string `json:"BlockProducerSig"`
	Epoch            uint64 `json:"Epoch"`
	Time             int64  `json:"Time"`
}

func NewGetBestBlockItemFromShard(bestState *blockchain.ShardBestState) *GetBestBlockItem {
	result := &GetBestBlockItem{
		Height:           bestState.BestBlock.Header.Height,
		Hash:             bestState.BestBlockHash.String(),
		TotalTxs:         bestState.TotalTxns,
		BlockProducer:    bestState.BestBlock.Header.ProducerAddress.String(),
		BlockProducerSig: bestState.BestBlock.ProducerSig,
		Time:             bestState.BestBlock.Header.Timestamp,
	}
	return result
}

func NewGetBestBlockItemFromBeacon(bestState *blockchain.BeaconBestState) *GetBestBlockItem {
	result := &GetBestBlockItem{
		Height:           bestState.BestBlock.Header.Height,
		Hash:             bestState.BestBlock.Hash().String(),
		BlockProducer:    bestState.BestBlock.Header.ProducerAddress.String(),
		BlockProducerSig: bestState.BestBlock.ProducerSig,
		Epoch:            bestState.Epoch,
		Time:             bestState.BestBlock.Header.Timestamp,
	}
	return result
}

type GetBestBlockHashResult struct {
	BestBlockHashes map[int]string `json:"BestBlockHashes"`
}
