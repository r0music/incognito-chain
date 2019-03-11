package jsonresult

import "github.com/big0t/constant-chain/blockchain"

type GetHeaderResult struct {
	BlockNum  int    `json:"blocknum"`
	ShardID   byte   `json:"shardID"`
	BlockHash string `json:"blockhash"`
	// Header    blockchain.ShardBlock `json:"header"`
	Header blockchain.ShardHeader `json:"header"`
}
