package transaction

import (
	"errors"
	"fmt"

	"github.com/incognitochain/incognito-chain/privacy"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy/privacy_v1/zeroknowledge/aggregatedrange"
)

type batchTransaction struct {
	txs []metadata.Transaction
}

func NewBatchTransaction(txs []metadata.Transaction) *batchTransaction {
	return &batchTransaction{txs: txs}
}

func (b *batchTransaction) AddTxs(txs []metadata.Transaction) {
	b.txs = append(b.txs, txs...)
}

func (b *batchTransaction) Validate(transactionStateDB *statedb.StateDB, bridgeStateDB *statedb.StateDB, bcr metadata.BlockchainRetriever) (bool, error, int) {
	return b.validateBatchTxsByItself(b.txs, transactionStateDB, bridgeStateDB, bcr)
}

func (b *batchTransaction) validateBatchTxsByItself(txList []metadata.Transaction, transactionStateDB *statedb.StateDB, bridgeStateDB *statedb.StateDB, bcr metadata.BlockchainRetriever) (bool, error, int) {
	prvCoinID := &common.Hash{}
	err := prvCoinID.SetBytes(common.PRVCoinID[:])
	if err != nil {
		return false, err, -1
	}
	bulletProofListVer1 := make([]*privacy.AggregatedRangeProofV1, 0)
	bulletProofListVer2 := make([]*privacy.AggregatedRangeProofV2, 0)

	for i, tx := range txList {
		shardID := common.GetShardIDFromLastByte(tx.GetSenderAddrLastByte())
		hasPrivacy := tx.IsPrivacy()
		ok, err := tx.ValidateTransaction(hasPrivacy, transactionStateDB, bridgeStateDB, shardID, prvCoinID, true, false)
		if !ok {
			return false, err, i
		}
		if tx.GetMetadata() != nil {
			if hasPrivacy {
				return false, errors.New("Metadata can not exist in privacy tx"), i
			}
			validateMetadata := tx.GetMetadata().ValidateMetadataByItself()
			if !validateMetadata {
				return validateMetadata, NewTransactionErr(UnexpectedError, errors.New("Metadata is invalid")), i
			}
		}

		if hasPrivacy {
			bulletproof := tx.GetProof().GetAggregatedRangeProof()
			if bulletproof == nil {
				continue
			}
			if tx.GetProof().GetVersion() == 1 {
				var p interface{} = bulletproof
				bulletproofV1 := p.(privacy.AggregatedRangeProofV1)
				bulletProofListVer1 = append(bulletProofListVer1, &bulletproofV1)
			} else if tx.GetProof().GetVersion() == 2 {
				var p interface{} = bulletproof
				bulletproofV2 := p.(privacy.AggregatedRangeProofV2)
				bulletProofListVer2 = append(bulletProofListVer2, &bulletproofV2)
			}

		}
	}
	//TODO: add go routine
	ok, err, i := aggregatedrange.VerifyBatchingAggregatedRangeProofs(bulletProofListVer1)
	if err != nil {
		return false, NewTransactionErr(TxProofVerifyFailError, err), -1
	}
	if !ok {
		Logger.Log.Errorf("FAILED VERIFICATION BATCH PAYMENT PROOF VER 1 %d", i)
		return false, NewTransactionErr(TxProofVerifyFailError, fmt.Errorf("FAILED VERIFICATION BATCH VER 1 PAYMENT PROOF %d", i)), -1
	}
	ok, err, i = aggregatedrange.VerifyBatchingAggregatedRangeProofs(bulletProofListVer1)
	if err != nil {
		return false, NewTransactionErr(TxProofVerifyFailError, err), -1
	}
	if !ok {
		Logger.Log.Errorf("FAILED VERIFICATION BATCH PAYMENT PROOF VER 2 %d", i)
		return false, NewTransactionErr(TxProofVerifyFailError, fmt.Errorf("FAILED VERIFICATION BATCH VER 2 PAYMENT PROOF %d", i)), -1
	}
	return true, nil, -1
}
