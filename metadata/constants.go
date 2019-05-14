package metadata

const (
	InvalidMeta = 1

	LoanRequestMeta  = 2
	LoanResponseMeta = 3
	LoanWithdrawMeta = 4
	LoanUnlockMeta   = 5
	LoanPaymentMeta  = 6

	// Dividend: removed 7-8

	// Crowdsale: removed 10-11

	// CMB: removed 12-19

	BuyFromGOVRequestMeta        = 20
	BuyFromGOVResponseMeta       = 21
	BuyBackRequestMeta           = 22
	BuyBackResponseMeta          = 23
	IssuingRequestMeta           = 24
	IssuingResponseMeta          = 25
	ContractingRequestMeta       = 26
	ContractingResponseMeta      = 27
	OracleFeedMeta               = 28
	OracleRewardMeta             = 29
	RefundMeta                   = 30
	UpdatingOracleBoardMeta      = 31
	MultiSigsRegistrationMeta    = 32
	MultiSigsSpendingMeta        = 33
	WithSenderAddressMeta        = 34
	ResponseBaseMeta             = 35
	BuyGOVTokenRequestMeta       = 36
	ShardBlockSalaryRequestMeta  = 37
	ShardBlockSalaryResponseMeta = 38
	BeaconSalaryRequestMeta      = 39
	BeaconSalaryResponseMeta     = 40
	ReturnStakingMeta            = 41

	//statking
	ShardStakingMeta  = 63
	BeaconStakingMeta = 64
)

const (
	MaxDivTxsPerBlock = 1000
)

// update oracle board actions
const (
	Add = iota + 1
	Remove
)

var minerCreatedMetaTypes = []int{
	BuyFromGOVRequestMeta,
	BuyBackRequestMeta,
	ShardBlockSalaryResponseMeta,
	IssuingResponseMeta,
	ContractingResponseMeta,
}

// Special rules for shardID: stored as 2nd param of instruction of BeaconBlock
