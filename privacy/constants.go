package privacy

const (
	CompressedPointSize      = 33
	PointCompressed     byte = 0x2

	CMRingSize    = 8 // 2^3
	CMRingSizeExp = 3

	MaxExp = 64

	// size of zero knowledge proof corresponding one input
	OneOfManyProofSize = 781 // corresponding to CMRingSize = 4: 521

	SNPrivacyProofSize   = 326
	SNNoPrivacyProofSize = 196

	// size of zero knowledge proof corresponding one output
	ComZeroProofSize     = 66

	InputCoinsPrivacySize  = 40  // serial number + 7 bytes saving size
	OutputCoinsPrivacySize = 246 // vKey + coin commitment + SND + Encrypted (138 bytes) + 9 bytes saving size

	InputCoinsNoPrivacySize  = 178  // vKey + coin commitment + SND + serial number + Randomness + Value + 7 bytes saving size
	OutputCoinsNoPrivacySize = 147 // vKey + coin commitment + SND + Randomness + Value + 9 bytes saving size

	// it is used for both privacy and no privacy
	SigPubKeySize    = 33
	SigPrivacySize   = 96
	SigNoPrivacySize = 64

	BigIntSize = 32 // bytes
	Uint64Size = 8  // bytes

	EncryptedRandomnessSize = 48 //bytes
	EncryptedSymKeySize = 66 //bytes
)
