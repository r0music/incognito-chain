package metadata

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"

	"github.com/ninjadotorg/constant/common"
	"github.com/ninjadotorg/constant/database"
	"github.com/ninjadotorg/constant/privacy"
)

type MultiSigsSpending struct {
	Signs map[string][]byte
	MetadataBase
}

func NewMultiSigsSpending(
	signs map[string][]byte,
	metaType int,
) *MultiSigsSpending {
	metaBase := MetadataBase{
		Type: metaType,
	}
	return &MultiSigsSpending{
		Signs:        signs,
		MetadataBase: metaBase,
	}
}

func getMultiSigsRegistration(
	txr Transaction,
	db database.DatabaseInterface,
) ([]byte, error) {
	pk := txr.GetSigPubKey()
	multiSigsReg, err := db.GetMultiSigsRegistration(pk)
	return multiSigsReg, err
}

func (msSpending *MultiSigsSpending) ValidateTxWithBlockChain(
	txr Transaction,
	bcr BlockchainRetriever,
	chainID byte,
	db database.DatabaseInterface,
) (bool, error) {
	// check spending address is already registered or not
	_, err := getMultiSigsRegistration(txr, db)
	if err != nil {
		return common.FalseValue, err
	}
	return common.TrueValue, nil
}

func (msSpending *MultiSigsSpending) ValidateSanityData(
	bcr BlockchainRetriever,
	txr Transaction,
) (bool, bool, error) {
	if len(msSpending.Signs) == 0 {
		return common.FalseValue, common.FalseValue, errors.New("Wrong request info's signs")
	}
	for pkStr, sign := range msSpending.Signs {
		if len(pkStr) == 0 {
			return common.FalseValue, common.FalseValue, errors.New("Wrong request info's public key string")
		}
		if len(sign) == 0 {
			return common.FalseValue, common.FalseValue, errors.New("Wrong request info's signs")
		}
	}
	return common.TrueValue, common.TrueValue, nil
}

func (msSpending *MultiSigsSpending) ValidateMetadataByItself() bool {
	if msSpending.Type != MultiSigsSpendingMeta {
		return common.FalseValue
	}
	return common.TrueValue
}

func (msSpending *MultiSigsSpending) Hash() *common.Hash {
	// record := string(common.ToBytes(msSpending.Signs))
	record := string(msSpending.MetadataBase.Hash()[:])

	// final hash
	hash := common.DoubleHashH([]byte(record))
	return &hash
}

func (msSpending *MultiSigsSpending) VerifyMultiSigs(
	txr Transaction,
	db database.DatabaseInterface,
) (bool, error) {
	multiSigsRegBytes, err := getMultiSigsRegistration(txr, db)
	if err != nil {
		return common.FalseValue, err
	}

	var multiSigsReg MultiSigsRegistration
	err = json.Unmarshal(multiSigsRegBytes, &multiSigsReg)
	if err != nil {
		return common.FalseValue, err
	}

	verifiedCount := 0
	spendablePubKeys := multiSigsReg.SpendableMembers
	for _, pk := range spendablePubKeys {
		pkStr := string(pk)
		sign, ok := msSpending.Signs[pkStr]
		if !ok {
			continue
		}
		verKey := new(ecdsa.PublicKey)
		point := new(privacy.EllipticPoint)
		point, _ = privacy.DecompressKey(pk)
		verKey.X, verKey.Y = point.X, point.Y
		verKey.Curve = privacy.Curve

		// convert signature from byte array to ECDSASign
		r, s := common.FromByteArrayToECDSASig(sign)

		// verify signature
		res := ecdsa.Verify(verKey, txr.Hash()[:], r, s)
		if res {
			verifiedCount += 1
		}
	}
	if verifiedCount < (len(spendablePubKeys)/2)+1 {
		return common.FalseValue, errors.New("There are not enough signatures in order to spend on the multisigs account")
	}
	return common.TrueValue, nil
}
