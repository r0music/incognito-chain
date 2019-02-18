package toshardins

import (
	"github.com/ninjadotorg/constant/common"
	"github.com/ninjadotorg/constant/database"
	"github.com/ninjadotorg/constant/metadata"
	"github.com/ninjadotorg/constant/privacy"
	"github.com/ninjadotorg/constant/transaction"
)

type TxSendBackTokenVoteFailIns struct {
	PaymentAddress privacy.PaymentAddress
	Amount         uint64
	PropertyID     common.Hash
}

func (txSendBackTokenVoteFailIns *TxSendBackTokenVoteFailIns) GetStringFormat() []string {
	panic("implement me")
}

func (txSendBackTokenVoteFailIns *TxSendBackTokenVoteFailIns) BuildTransaction(
	minerPrivateKey *privacy.SpendingKey,
	db database.DatabaseInterface,
) metadata.Transaction {
	tx := NewSendBackTokenVoteFailTx(
		minerPrivateKey,
		db,
		txSendBackTokenVoteFailIns.PaymentAddress,
		txSendBackTokenVoteFailIns.Amount,
		txSendBackTokenVoteFailIns.PropertyID,
	)
	return tx
}

func NewSendBackTokenVoteFailIns(
	paymentAddress privacy.PaymentAddress,
	amount uint64,
	propertyID common.Hash,
) Instruction {
	return &TxSendBackTokenVoteFailIns{
		PaymentAddress: paymentAddress,
		Amount:         amount,
		PropertyID:     propertyID,
	}
}

func NewSendBackTokenVoteFailTx(
	minerPrivateKey *privacy.SpendingKey,
	db database.DatabaseInterface,
	paymentAddress privacy.PaymentAddress,
	amount uint64,
	propertyID common.Hash,
) metadata.Transaction {
	txTokenVout := transaction.TxTokenVout{
		Value:          amount,
		PaymentAddress: paymentAddress,
	}
	newTx := transaction.TxCustomToken{
		TxTokenData: transaction.TxTokenData{
			Type:       transaction.CustomTokenInit,
			Amount:     amount,
			PropertyID: propertyID,
			Vins:       []transaction.TxTokenVin{},
			Vouts:      []transaction.TxTokenVout{txTokenVout},
		},
	}
	newTx.SetMetadata(metadata.NewSendBackTokenVoteFailMetadata())
	return &newTx
}
