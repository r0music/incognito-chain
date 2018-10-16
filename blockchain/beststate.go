package blockchain

import (
	"sync"

	"github.com/ninjadotorg/cash-prototype/common"
	"github.com/ninjadotorg/cash-prototype/privacy/client"
)

// BestState houses information about the current best block and other info
// related to the state of the main chain as it exists from the point of view of
// the current best block.
//
// The BestSnapshot method can be used to obtain access to this information
// in a concurrent safe manner and the data will not be changed out from under
// the caller when chain state changes occur as the function name implies.
// However, the returned snapshot must be treated as immutable since it is
// shared by all callers.
type BestState struct {
	BestBlockHash *common.Hash // The hash of the block.
	BestBlock     *Block       // The hash of the block.

	CmTree *client.IncMerkleTree // The commitments merkle tree of the best block

	Height int32 // The height of the block.
	// Difficulty  []uint32  // The difficulty bits of the block.
	BlockSize uint64 // The size of the block.
	// BlockWeight []uint64  // The weight of the block.
	NumTxns   uint64 // The number of txns in the block.
	TotalTxns uint64 // The total number of txns in the chain.
	// MedianTime time.Time // Median time as per CalcPastMedianTime.
	sync.Mutex

	CndMap  map[string]uint64
}

/*
Init create a beststate data from block and commitment tree
*/
// #1 - block
// #2 - commitment merkle tree
func (self *BestState) Init(block *Block, tree *client.IncMerkleTree) {
	bestBlockHash := block.Hash()
	self.BestBlock = block
	self.BestBlockHash = bestBlockHash
	self.CmTree = tree

	self.TotalTxns += uint64(len(block.Transactions))
	self.NumTxns = uint64(len(block.Transactions))
	self.Height = block.Height
}

func (self *BestState) Update(block *Block) {
	tree := self.CmTree
	err := UpdateMerkleTreeForBlock(tree, block)
	if err != nil {
		Logger.log.Error("WHAT")
	}
	bestBlockHash := block.Hash()
	self.BestBlock = block
	self.BestBlockHash = bestBlockHash
	self.CmTree = tree

	self.TotalTxns += uint64(len(block.Transactions))
	self.NumTxns = uint64(len(block.Transactions))
	self.Height = block.Height
}
