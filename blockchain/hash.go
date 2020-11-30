package blockchain

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"strconv"

	"github.com/incognitochain/incognito-chain/blockchain/types"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/common"
)

// BuildKeccak256MerkleTree creates a merkle tree using Keccak256 hash func.
// This merkle tree is used for storing all beacon (and bridge) data to relay them to Ethereum.
func BuildKeccak256MerkleTree(data [][]byte) [][]byte {
	if len(data) == 0 {
		emptyRoot := [32]byte{}
		return [][]byte{emptyRoot[:]}
	}
	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := NextPowerOfTwo(len(data))
	arraySize := nextPoT*2 - 1
	merkles := make([][]byte, arraySize)

	// Create the base data hashes and populate the array with them.
	for i, d := range data {
		h := common.Keccak256(d)
		merkles[i] = h[:]
	}

	// Start the array offset after the last data and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

			// When there is no right child, the parent is generated by
			// hashing the concatenation of the left child with itself.
		case merkles[i+1] == nil:
			newHash := keccak256MerkleBranches(merkles[i], merkles[i])
			merkles[offset] = newHash

			// The normal case sets the parent node to the keccak256
			// of the concatentation of the left and right children.
		default:
			newHash := keccak256MerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}

	return merkles
}

func GetKeccak256MerkleRoot(data [][]byte) []byte {
	merkles := BuildKeccak256MerkleTree(data)
	return merkles[len(merkles)-1]
}

func GetKeccak256MerkleProofFromTree(merkles [][]byte, id int) ([][]byte, []bool) {
	path := [][]byte{}
	left := []bool{}
	height := uint(math.Log2(float64(len(merkles))))
	start := 0
	for i := uint(0); i < height; i++ {
		sibling := id ^ 1
		path = append(path, merkles[sibling])
		left = append(left, sibling < id)

		id = (id-start)/2 + start + (1 << (height - i)) // Go to parent node
		start += 1 << (height - i)
	}
	return path, left
}

// keccak256MerkleBranches concatenates the 2 branches of a Merkle tree and hash it to create the parent node using Keccak256 hash function
func keccak256MerkleBranches(left []byte, right []byte) []byte {
	// Concatenate the left and right nodes.
	hash := append(left, right...)
	newHash := common.Keccak256(hash)
	return newHash[:]
}

type Merkle struct {
}

// BuildMerkleTreeStore creates a merkle tree from a slice of transactions,
// stores it using a linear array, and returns a slice of the backing array.  A
// linear array was chosen as opposed to an actual tree structure since it uses
// about half as much memory.  The following describes a merkle tree and how it
// is stored in a linear array.
//
// A merkle tree is a tree in which every non-leaf node is the hash of its
// children nodes.  A diagram depicting how this works for Incognito transactions
// where h(x) is a double sha256 follows:
//
//	         root = h1234 = h(h12 + h34)
//	        /                           \
//	  h12 = h(h1 + h2)            h34 = h(h3 + h4)
//	   /            \              /            \
//	h1 = h(tx1)  h2 = h(tx2)    h3 = h(tx3)  h4 = h(tx4)
//
// The above stored as a linear array is as follows:
//
// 	[h1 h2 h3 h4 h12 h34 root]
//
// As the above shows, the merkle root is always the last element in the array.
//
// The number of inputs is not always a power of two which results in a
// balanced tree structure as above.  In that case, parent nodes with no
// children are also zero and parent nodes with only a single left node
// are calculated by concatenating the left node with itself before hashing.
// Since this function uses nodes that are pointers to the hashes, empty nodes
// will be nil.
//
// The additional bool parameter indicates if we are generating the merkle tree
// using witness transaction id's rather than regular transaction id's. This
// also presents an additional case wherein the wtxid of the salary transaction
// is the zeroHash.
func (merkle Merkle) BuildMerkleTreeStore(transactions []metadata.Transaction) []*common.Hash {
	if len(transactions) == 0 {
		return []*common.Hash{}
	}
	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := NextPowerOfTwo(len(transactions))
	arraySize := nextPoT*2 - 1
	merkles := make([]*common.Hash, arraySize)

	// Create the base transaction hashes and populate the array with them.
	for i, tx := range transactions {
		merkles[i] = tx.Hash()
	}

	// Start the array offset after the last transaction and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

			// When there is no right child, the parent is generated by
			// hashing the concatenation of the left child with itself.
		case merkles[i+1] == nil:
			newHash := merkle.hashMerkleBranches(merkles[i], merkles[i])
			merkles[offset] = newHash

			// The normal case sets the parent node to the double sha256
			// of the concatentation of the left and right children.
		default:
			newHash := merkle.hashMerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}

	return merkles
}

func (merkle Merkle) BuildMerkleTreeOfHashes(shardsHash []*common.Hash, length int) []*common.Hash {
	// Calculate how many entries are required to hold the binary merkle
	// tree as a linear array and create an array of that size.
	nextPoT := NextPowerOfTwo(length)
	arraySize := nextPoT*2 - 1
	merkles := make([]*common.Hash, arraySize)

	// Create the base transaction hashes and populate the array with them.
	copy(merkles, shardsHash)
	for i := len(shardsHash); i < len(merkles); i++ {
		merkles[i], _ = common.Hash{}.NewHashFromStr("")
	}

	// Start the array offset after the last transaction and adjusted to the
	// next power of two.
	offset := nextPoT
	for i := 0; i < arraySize-1; i += 2 {
		switch {
		// When there is no left child node, the parent is nil too.
		case merkles[i] == nil:
			merkles[offset] = nil

			// When there is no right child, the parent is generated by
			// hashing the concatenation of the left child with itself.
		case merkles[i+1] == nil:
			newHash := merkle.hashMerkleBranches(merkles[i], merkles[i])
			merkles[offset] = newHash

			// The normal case sets the parent node to the double sha256
			// of the concatentation of the left and right children.
		default:
			newHash := merkle.hashMerkleBranches(merkles[i], merkles[i+1])
			merkles[offset] = newHash
		}
		offset++
	}
	return merkles
}

func (merkle Merkle) VerifyMerkleRootOfHashes(merkleTree []*common.Hash, merkleRoot *common.Hash, length int) bool {
	res := merkle.BuildMerkleTreeOfHashes(merkleTree, length)
	tempRoot := res[len(res)-1].GetBytes()
	return bytes.Equal(tempRoot, merkleRoot.GetBytes())
}

func (merkle Merkle) BuildMerkleTreeOfHashes2(shardsHashes []common.Hash, length int) []common.Hash {
	// tempShardsHashes := make([]*common.Hash, len(shardsHashes))

	tempShardsHashes := []*common.Hash{}

	for _, value := range shardsHashes {
		newHash, _ := common.Hash{}.NewHashFromStr(value.String())
		tempShardsHashes = append(tempShardsHashes, newHash)
	}
	merkleData := merkle.BuildMerkleTreeOfHashes(tempShardsHashes, length)
	tempMerkleData := make([]common.Hash, len(merkleData))
	for i, value := range merkleData {
		tempMerkleData[i] = *value
	}
	return tempMerkleData
}
func (merkle Merkle) VerifyMerkleRootOfHashes2(merkleTree []common.Hash, merkleRoot common.Hash, length int) bool {
	res := merkle.BuildMerkleTreeOfHashes2(merkleTree, length)
	tempRoot := res[len(res)-1].GetBytes()
	return bytes.Equal(tempRoot, merkleRoot.GetBytes())
}

func (merkle Merkle) GetMerklePathForCrossShard(length int, merkleTree []common.Hash, shardID byte) (merklePathShard []common.Hash, merkleShardRoot common.Hash) {
	nextPoT := NextPowerOfTwo(length)
	// merkleSize := nextPoT*2 - 1
	cursor := 0
	lastCursor := 0
	sid := int(shardID)
	i := sid
	time := 0
	for {
		if cursor >= len(merkleTree)-2 {
			break
		}
		if i%2 == 0 {
			merklePathShard = append(merklePathShard, merkleTree[cursor+i+1])
		} else {
			merklePathShard = append(merklePathShard, merkleTree[cursor+i-1])
		}
		i = i / 2

		if time == 0 {
			cursor += nextPoT
		} else {
			tmp := cursor
			cursor += (cursor - lastCursor) / 2
			lastCursor = tmp
		}
		time++
	}
	merkleShardRoot = merkleTree[len(merkleTree)-1]
	return merklePathShard, merkleShardRoot
}
func (merkle Merkle) VerifyMerkleRootFromMerklePath(leaf common.Hash, merklePath []common.Hash, merkleRoot common.Hash, receiverShardID byte) bool {

	i := int(receiverShardID)
	finalHash := &leaf
	for _, hashPath := range merklePath {
		if i%2 == 0 {
			finalHash = merkle.hashMerkleBranches(finalHash, &hashPath)
		} else {
			finalHash = merkle.hashMerkleBranches(&hashPath, finalHash)
		}
		i = i / 2
	}
	merkleRootPointer := &merkleRoot
	return merkleRootPointer.IsEqual(finalHash)
}

// nextPowerOfTwo returns the next highest power of two from a given number if
// it is not already a power of two.  This is a helper function used during the
// calculation of a merkle tree.
func NextPowerOfTwo(n int) int {
	// Return the number if it's already a power of 2.
	if n&(n-1) == 0 {
		return n
	}

	// Figure out and return the next power of two.
	exponent := uint(math.Log2(float64(n))) + 1
	return 1 << exponent // 2^exponent
}

/*
hashMerkleBranches takes two hashes, treated as the left and right tree
nodes, and returns the hash of their concatenation.  This is a helper
function used to aid in the generation of a merkle tree.
*/
func (merkle Merkle) hashMerkleBranches(left *common.Hash, right *common.Hash) *common.Hash {
	// Concatenate the left and right nodes.
	var hash [common.HashSize * 2]byte
	copy(hash[:common.HashSize], left[:])
	copy(hash[common.HashSize:], right[:])

	newHash := common.HashH(hash[:])
	return &newHash
}

func CalcMerkleRoot(txns []metadata.Transaction) common.Hash {
	if len(txns) == 0 {
		return common.Hash{}
	}

	utilTxns := make([]metadata.Transaction, 0, len(txns))
	utilTxns = append(utilTxns, txns...)
	merkles := Merkle{}.BuildMerkleTreeStore(utilTxns)
	return *merkles[len(merkles)-1]
}

func generateZeroValueHash() (common.Hash, error) {
	hash := common.Hash{}
	hash.SetBytes(make([]byte, 32))
	return hash, nil
}

func generateHashFromStringArray(strs []string) (common.Hash, error) {
	// if input is empty list
	// return hash value of bytes zero
	if len(strs) == 0 {
		return generateZeroValueHash()
	}
	var (
		hash common.Hash
		buf  bytes.Buffer
	)
	for _, value := range strs {
		buf.WriteString(value)
	}
	temp := common.HashB(buf.Bytes())
	if err := hash.SetBytes(temp[:]); err != nil {
		return common.Hash{}, NewBlockChainError(HashError, err)
	}
	return hash, nil
}

func generateHashFromMapStringString(maps1 map[string]string) (common.Hash, error) {
	var keys []string
	var res []string
	for k := range maps1 {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		res = append(res, key)
		res = append(res, maps1[key])
	}
	return generateHashFromStringArray(res)
}

func generateHashFromShardState(allShardState map[byte][]types.ShardState) (common.Hash, error) {
	allShardStateStr := []string{}
	var keys []int
	for k := range allShardState {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, shardID := range keys {
		res := ""
		for _, shardState := range allShardState[byte(shardID)] {
			res += strconv.Itoa(int(shardState.Height))
			res += shardState.Hash.String()
			crossShard, _ := json.Marshal(shardState.CrossShard)
			res += string(crossShard)
		}
		allShardStateStr = append(allShardStateStr, res)
	}
	return generateHashFromStringArray(allShardStateStr)
}

func verifyHashFromStringArray(strs []string, hash common.Hash) (common.Hash, bool) {
	res, err := generateHashFromStringArray(strs)
	if err != nil {
		return common.Hash{}, false
	}
	return res, bytes.Equal(res.GetBytes(), hash.GetBytes())
}

func verifyHashFromShardState(allShardState map[byte][]types.ShardState, hash common.Hash) bool {
	res, err := generateHashFromShardState(allShardState)
	if err != nil {
		return false
	}
	return bytes.Equal(res.GetBytes(), hash.GetBytes())
}
