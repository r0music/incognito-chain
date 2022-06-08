package statedb

import (
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/trie"
)

func (stateDB *StateDB) Retrieve(db incdb.Database, shouldAddToStateBloom bool, shouldDelete bool, stateBloom *trie.StateBloom) error {
	temp := stateDB.trie.NodeIterator(nil)
	it := trie.NewIterator(temp)
	keys := make(map[common.Hash]struct{})
	for it.Next(false) {
		if len(it.Key) == 0 {
			continue
		}
		key := it.Key
		h := common.Hash{}
		err := h.SetBytes(key)
		if err != nil {
			return err
		}
		if shouldAddToStateBloom {
			if err := stateBloom.Put(key, nil); err != nil {
				return err
			}
		}
		if shouldDelete {
			if ok, err := stateBloom.Contain(key); err != nil {
				return err
			} else if ok {
				continue
			}
			keys[h] = struct{}{}
		}
	}
	if shouldDelete && len(keys) != 0 {
		batch := db.NewBatch()
		for key := range keys {
			if err := batch.Delete(key.Bytes()); err != nil {
				return err
			}
			if batch.ValueSize() >= incdb.IdealBatchSize {
				if err := batch.Write(); err != nil {
					return err
				}
				batch.Reset()
			}
		}
		if batch.ValueSize() > 0 {
			batch.Write()
			batch.Reset()
		}
	}

	return nil
}
