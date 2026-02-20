// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// You may obtain a copy of the licence at:
//
//     https://joinup.ec.europa.eu/software/page/eupl/licence-eupl
//
// SPDX-License-Identifier: EUPL-1.2

// Package config defines chain parameters for the Lethean blockchain.
//
// All constants are derived from the canonical C++ source files
// currency_config.h.in and default.cmake. Mainnet and Testnet configurations
// are provided as package-level variables.
package config

// ---------------------------------------------------------------------------
// Tokenomics
// ---------------------------------------------------------------------------

// Coin is the number of smallest indivisible units in one LTHN.
// 1 LTHN = 10^12 atomic units.
const Coin uint64 = 1_000_000_000_000

// DisplayDecimalPoint is the number of decimal places used when displaying
// amounts in human-readable form.
const DisplayDecimalPoint = 12

// BlockReward is the fixed reward per block in atomic units (1.0 LTHN).
const BlockReward uint64 = 1_000_000_000_000

// DefaultFee is the standard transaction fee in atomic units (0.01 LTHN).
const DefaultFee uint64 = 10_000_000_000

// MinimumFee is the lowest acceptable transaction fee in atomic units (0.01 LTHN).
const MinimumFee uint64 = 10_000_000_000

// Premine is the total pre-mined supply in atomic units (10,000,000 LTHN).
// This covers the coinswap allocation (13,827,203 LTHN reserved) and the
// initial premine (3,690,000 LTHN). The raw value from cmake is
// 10,000,000,000,000,000,000 but that exceeds uint64 range. The C++ code
// uses an unsigned literal suffix; we store the value faithfully.
const Premine uint64 = 10_000_000_000_000_000_000

// BaseRewardDustThreshold is the minimum meaningful fraction of a block
// reward in atomic units.
const BaseRewardDustThreshold uint64 = 1_000_000

// DefaultDustThreshold is the dust threshold for normal transactions.
const DefaultDustThreshold uint64 = 0

// ---------------------------------------------------------------------------
// Address prefixes
// ---------------------------------------------------------------------------

// Address prefix constants determine the leading characters of base58-encoded
// addresses. Each prefix is varint-encoded before the public keys.
const (
	// AddressPrefix is the standard public address prefix.
	// Produces addresses starting with "iTHN".
	AddressPrefix uint64 = 0x1eaf7

	// IntegratedAddressPrefix is the prefix for integrated addresses
	// (address + embedded payment ID). Produces addresses starting with "iTHn".
	IntegratedAddressPrefix uint64 = 0xdeaf7

	// AuditableAddressPrefix is the prefix for auditable addresses.
	// Produces addresses starting with "iThN".
	AuditableAddressPrefix uint64 = 0x3ceff7

	// AuditableIntegratedAddressPrefix is the prefix for auditable
	// integrated addresses. Produces addresses starting with "iThn".
	AuditableIntegratedAddressPrefix uint64 = 0x8b077
)

// ---------------------------------------------------------------------------
// P2P and RPC ports
// ---------------------------------------------------------------------------

const (
	MainnetP2PPort     uint16 = 36942
	MainnetRPCPort     uint16 = 36941
	MainnetStratumPort uint16 = 36940

	TestnetP2PPort     uint16 = 46942
	TestnetRPCPort     uint16 = 46941
	TestnetStratumPort uint16 = 46940
)

// ---------------------------------------------------------------------------
// Timing and difficulty
// ---------------------------------------------------------------------------

const (
	// DifficultyPowTarget is the target block interval for PoW blocks in seconds.
	DifficultyPowTarget uint64 = 120

	// DifficultyPosTarget is the target block interval for PoS blocks in seconds.
	DifficultyPosTarget uint64 = 120

	// DifficultyTotalTarget is the effective combined target:
	// (PoW + PoS) / 4 = 60 seconds.
	DifficultyTotalTarget uint64 = (DifficultyPowTarget + DifficultyPosTarget) / 4

	// DifficultyWindow is the number of blocks used for difficulty calculation.
	DifficultyWindow uint64 = 720

	// DifficultyLag is the additional lookback beyond the window.
	DifficultyLag uint64 = 15

	// DifficultyCut is the number of timestamps cut from each end after sorting.
	DifficultyCut uint64 = 60

	// DifficultyBlocksCount is the total number of blocks considered
	// (Window + Lag).
	DifficultyBlocksCount uint64 = DifficultyWindow + DifficultyLag

	// DifficultyPowStarter is the initial PoW difficulty.
	DifficultyPowStarter uint64 = 1

	// DifficultyPosStarter is the initial PoS difficulty.
	DifficultyPosStarter uint64 = 1

	// DifficultyPowTargetHF6 is the PoW target after hardfork 6 (240s).
	DifficultyPowTargetHF6 uint64 = 240

	// DifficultyPosTargetHF6 is the PoS target after hardfork 6 (240s).
	DifficultyPosTargetHF6 uint64 = 240

	// DifficultyTotalTargetHF6 is the combined target after HF6.
	DifficultyTotalTargetHF6 uint64 = (DifficultyPowTargetHF6 + DifficultyPosTargetHF6) / 4
)

// ---------------------------------------------------------------------------
// Block and transaction limits
// ---------------------------------------------------------------------------

const (
	// MaxBlockNumber is the absolute maximum block height.
	MaxBlockNumber uint64 = 500_000_000

	// MaxBlockSize is the maximum block header blob size in bytes.
	MaxBlockSize uint64 = 500_000_000

	// TxMaxAllowedInputs is the maximum number of inputs per transaction.
	// Limited primarily by the asset surjection proof.
	TxMaxAllowedInputs uint64 = 256

	// TxMaxAllowedOutputs is the maximum number of outputs per transaction.
	TxMaxAllowedOutputs uint64 = 2000

	// TxMinAllowedOutputs is the minimum number of outputs (effective from HF4 Zarcanum).
	TxMinAllowedOutputs uint64 = 2

	// DefaultDecoySetSize is the ring size for pre-HF4 transactions.
	DefaultDecoySetSize uint64 = 10

	// HF4MandatoryDecoySetSize is the ring size required from HF4 onwards.
	HF4MandatoryDecoySetSize uint64 = 15

	// HF4MandatoryMinCoinage is the minimum coinage in blocks required for HF4.
	HF4MandatoryMinCoinage uint64 = 10

	// MinedMoneyUnlockWindow is the number of blocks before mined coins
	// can be spent.
	MinedMoneyUnlockWindow uint64 = 10

	// BlockGrantedFullRewardZone is the block size threshold in bytes after
	// which block reward is calculated using the actual block size.
	BlockGrantedFullRewardZone uint64 = 125_000

	// CoinbaseBlobReservedSize is the reserved space for the coinbase
	// transaction blob in bytes.
	CoinbaseBlobReservedSize uint64 = 1100

	// BlockFutureTimeLimit is the maximum acceptable future timestamp for
	// PoW blocks in seconds (2 hours).
	BlockFutureTimeLimit uint64 = 60 * 60 * 2

	// PosBlockFutureTimeLimit is the maximum acceptable future timestamp
	// for PoS blocks in seconds (20 minutes).
	PosBlockFutureTimeLimit uint64 = 60 * 20

	// TimestampCheckWindow is the number of blocks used when checking
	// whether a block timestamp is valid.
	TimestampCheckWindow uint64 = 60

	// PosStartHeight is the block height from which PoS is enabled.
	PosStartHeight uint64 = 0

	// RewardBlocksWindow is the number of recent blocks used to calculate
	// the reward median.
	RewardBlocksWindow uint64 = 400

	// FreeTxMaxBlobSize is the soft txpool-based limit for free transactions
	// in bytes.
	FreeTxMaxBlobSize uint64 = 1024

	// PreHardforkTxFreezePeriod is the number of blocks before hardfork
	// activation when no new transactions are accepted (effective from HF5).
	PreHardforkTxFreezePeriod uint64 = 60
)

// ---------------------------------------------------------------------------
// Block version constants
// ---------------------------------------------------------------------------

const (
	BlockMajorVersionGenesis  uint8 = 1
	BlockMinorVersionGenesis  uint8 = 0
	BlockMajorVersionInitial  uint8 = 0
	HF1BlockMajorVersion      uint8 = 1
	HF3BlockMajorVersion      uint8 = 2
	HF3BlockMinorVersion      uint8 = 0
	CurrentBlockMajorVersion  uint8 = 3
	CurrentBlockMinorVersion  uint8 = 0
)

// ---------------------------------------------------------------------------
// Transaction version constants
// ---------------------------------------------------------------------------

const (
	TransactionVersionInitial  uint8 = 0
	TransactionVersionPreHF4   uint8 = 1
	TransactionVersionPostHF4  uint8 = 2
	TransactionVersionPostHF5  uint8 = 3
	CurrentTransactionVersion  uint8 = 3
)

// ---------------------------------------------------------------------------
// PoS constants
// ---------------------------------------------------------------------------

const (
	PosScanWindow                  uint64 = 60 * 10 // 10 minutes in seconds
	PosScanStep                    uint64 = 15      // seconds
	PosModifierInterval            uint64 = 10
	PosMinimumCoinstakeAge         uint64 = 10 // blocks
	PosStrictSequenceLimit         uint64 = 20
	PosStarterKernelHash                  = "00000000000000000006382a8d8f94588ce93a1351924f6ccb9e07dd287c6e4b"
)

// ---------------------------------------------------------------------------
// P2P constants
// ---------------------------------------------------------------------------

const (
	P2PLocalWhitePeerlistLimit uint64 = 1000
	P2PLocalGrayPeerlistLimit  uint64 = 5000
	P2PDefaultConnectionsCount uint64 = 8
	P2PDefaultHandshakeInterval uint64 = 60      // seconds
	P2PDefaultPacketMaxSize    uint64 = 50_000_000
	P2PIPBlockTime             uint64 = 60 * 60 * 24 // 24 hours
	P2PIPFailsBeforeBlock      uint64 = 10
)

// ---------------------------------------------------------------------------
// Network identity
// ---------------------------------------------------------------------------

const (
	// CurrencyFormationVersion identifies the mainnet network.
	CurrencyFormationVersion uint64 = 84

	// CurrencyFormationVersionTestnet identifies the testnet network.
	CurrencyFormationVersionTestnet uint64 = 100

	// P2PNetworkIDVer is derived from CurrencyFormationVersion + 0.
	P2PNetworkIDVer uint64 = CurrencyFormationVersion + 0
)

// NetworkIDMainnet is the 16-byte network UUID for mainnet P2P handshake.
// From net_node.inl: bytes 0-9 are fixed, byte 10 = testnet flag (0),
// bytes 11-14 fixed, byte 15 = formation version (84 = 0x54).
var NetworkIDMainnet = [16]byte{
	0x11, 0x10, 0x01, 0x11, 0x01, 0x01, 0x11, 0x01,
	0x10, 0x11, 0x00, 0x11, 0x01, 0x11, 0x21, 0x54,
}

// NetworkIDTestnet is the 16-byte network UUID for testnet P2P handshake.
// Byte 10 = testnet flag (1), byte 15 = formation version (100 = 0x64).
var NetworkIDTestnet = [16]byte{
	0x11, 0x10, 0x01, 0x11, 0x01, 0x01, 0x11, 0x01,
	0x10, 0x11, 0x01, 0x11, 0x01, 0x11, 0x21, 0x64,
}

// ClientVersion is the version string sent in CORE_SYNC_DATA.
const ClientVersion = "Lethean/go-blockchain 0.1.0"


// ---------------------------------------------------------------------------
// Currency identity
// ---------------------------------------------------------------------------

const (
	CurrencyNameAbbreviation = "LTHN"
	CurrencyNameBase         = "Lethean"
	CurrencyNameShort        = "Lethean"
)

// ---------------------------------------------------------------------------
// Alias constants
// ---------------------------------------------------------------------------

const (
	AliasMinimumPublicShortNameAllowed uint64 = 6
	AliasNameMaxLen                    uint64 = 255
	AliasValidChars                           = "0123456789abcdefghijklmnopqrstuvwxyz-."
	AliasCommentMaxSizeBytes           uint64 = 400
	MaxAliasPerBlock                   uint64 = 1000
)

// ---------------------------------------------------------------------------
// ChainConfig aggregates all chain parameters into a single struct.
// ---------------------------------------------------------------------------

// ChainConfig holds the complete set of parameters for a particular chain
// (mainnet or testnet).
type ChainConfig struct {
	// Name is the human-readable chain name.
	Name string

	// Abbreviation is the ticker symbol.
	Abbreviation string

	// IsTestnet indicates whether this is a test network.
	IsTestnet bool

	// CurrencyFormationVersion identifies the network.
	CurrencyFormationVersion uint64

	// Coin is the number of atomic units per coin.
	Coin uint64

	// DisplayDecimalPoint is the number of decimal places.
	DisplayDecimalPoint uint8

	// BlockReward is the fixed block reward in atomic units.
	BlockReward uint64

	// DefaultFee is the default transaction fee in atomic units.
	DefaultFee uint64

	// MinimumFee is the minimum acceptable fee in atomic units.
	MinimumFee uint64

	// Premine is the pre-mined amount in atomic units.
	Premine uint64

	// AddressPrefix is the base58 prefix for standard addresses.
	AddressPrefix uint64

	// IntegratedAddressPrefix is the base58 prefix for integrated addresses.
	IntegratedAddressPrefix uint64

	// AuditableAddressPrefix is the base58 prefix for auditable addresses.
	AuditableAddressPrefix uint64

	// AuditableIntegratedAddressPrefix is the base58 prefix for auditable
	// integrated addresses.
	AuditableIntegratedAddressPrefix uint64

	// P2PPort is the default peer-to-peer port.
	P2PPort uint16

	// RPCPort is the default RPC port.
	RPCPort uint16

	// StratumPort is the default stratum mining port.
	StratumPort uint16

	// DifficultyPowTarget is the target PoW block interval in seconds.
	DifficultyPowTarget uint64

	// DifficultyPosTarget is the target PoS block interval in seconds.
	DifficultyPosTarget uint64

	// DifficultyWindow is the number of blocks in the difficulty window.
	DifficultyWindow uint64

	// DifficultyLag is the additional lookback beyond the window.
	DifficultyLag uint64

	// DifficultyCut is the number of timestamps cut after sorting.
	DifficultyCut uint64

	// DifficultyPowStarter is the initial PoW difficulty.
	DifficultyPowStarter uint64

	// DifficultyPosStarter is the initial PoS difficulty.
	DifficultyPosStarter uint64

	// MaxBlockNumber is the absolute maximum block height.
	MaxBlockNumber uint64

	// TxMaxAllowedInputs is the maximum inputs per transaction.
	TxMaxAllowedInputs uint64

	// TxMaxAllowedOutputs is the maximum outputs per transaction.
	TxMaxAllowedOutputs uint64

	// DefaultDecoySetSize is the default ring size.
	DefaultDecoySetSize uint64

	// HF4MandatoryDecoySetSize is the mandatory ring size from HF4.
	HF4MandatoryDecoySetSize uint64

	// MinedMoneyUnlockWindow is the maturity period for mined coins.
	MinedMoneyUnlockWindow uint64

	// P2PMaintainersPubKey is the hex-encoded maintainers public key.
	P2PMaintainersPubKey string

	// NetworkID is the 16-byte network UUID for P2P handshake.
	NetworkID [16]byte
}

// Mainnet holds the chain configuration for the Lethean mainnet.
var Mainnet = ChainConfig{
	Name:                             CurrencyNameBase,
	Abbreviation:                     CurrencyNameAbbreviation,
	IsTestnet:                        false,
	CurrencyFormationVersion:         CurrencyFormationVersion,
	Coin:                             Coin,
	DisplayDecimalPoint:              DisplayDecimalPoint,
	BlockReward:                      BlockReward,
	DefaultFee:                       DefaultFee,
	MinimumFee:                       MinimumFee,
	Premine:                          Premine,
	AddressPrefix:                    AddressPrefix,
	IntegratedAddressPrefix:          IntegratedAddressPrefix,
	AuditableAddressPrefix:           AuditableAddressPrefix,
	AuditableIntegratedAddressPrefix: AuditableIntegratedAddressPrefix,
	P2PPort:                          MainnetP2PPort,
	RPCPort:                          MainnetRPCPort,
	StratumPort:                      MainnetStratumPort,
	DifficultyPowTarget:              DifficultyPowTarget,
	DifficultyPosTarget:              DifficultyPosTarget,
	DifficultyWindow:                 DifficultyWindow,
	DifficultyLag:                    DifficultyLag,
	DifficultyCut:                    DifficultyCut,
	DifficultyPowStarter:             DifficultyPowStarter,
	DifficultyPosStarter:             DifficultyPosStarter,
	MaxBlockNumber:                   MaxBlockNumber,
	TxMaxAllowedInputs:               TxMaxAllowedInputs,
	TxMaxAllowedOutputs:              TxMaxAllowedOutputs,
	DefaultDecoySetSize:              DefaultDecoySetSize,
	HF4MandatoryDecoySetSize:         HF4MandatoryDecoySetSize,
	MinedMoneyUnlockWindow:           MinedMoneyUnlockWindow,
	P2PMaintainersPubKey:             "8f138bb73f6d663a3746a542770781a09579a7b84cb4125249e95530824ee607",
	NetworkID:                       NetworkIDMainnet,
}

// Testnet holds the chain configuration for the Lethean testnet.
var Testnet = ChainConfig{
	Name:                             CurrencyNameBase + "_testnet",
	Abbreviation:                     CurrencyNameAbbreviation,
	IsTestnet:                        true,
	CurrencyFormationVersion:         CurrencyFormationVersionTestnet,
	Coin:                             Coin,
	DisplayDecimalPoint:              DisplayDecimalPoint,
	BlockReward:                      BlockReward,
	DefaultFee:                       DefaultFee,
	MinimumFee:                       MinimumFee,
	Premine:                          Premine,
	AddressPrefix:                    AddressPrefix,
	IntegratedAddressPrefix:          IntegratedAddressPrefix,
	AuditableAddressPrefix:           AuditableAddressPrefix,
	AuditableIntegratedAddressPrefix: AuditableIntegratedAddressPrefix,
	P2PPort:                          TestnetP2PPort,
	RPCPort:                          TestnetRPCPort,
	StratumPort:                      TestnetStratumPort,
	DifficultyPowTarget:              DifficultyPowTarget,
	DifficultyPosTarget:              DifficultyPosTarget,
	DifficultyWindow:                 DifficultyWindow,
	DifficultyLag:                    DifficultyLag,
	DifficultyCut:                    DifficultyCut,
	DifficultyPowStarter:             DifficultyPowStarter,
	DifficultyPosStarter:             DifficultyPosStarter,
	MaxBlockNumber:                   MaxBlockNumber,
	TxMaxAllowedInputs:               TxMaxAllowedInputs,
	TxMaxAllowedOutputs:              TxMaxAllowedOutputs,
	DefaultDecoySetSize:              DefaultDecoySetSize,
	HF4MandatoryDecoySetSize:         HF4MandatoryDecoySetSize,
	MinedMoneyUnlockWindow:           MinedMoneyUnlockWindow,
	P2PMaintainersPubKey:             "8f138bb73f6d663a3746a542770781a09579a7b84cb4125249e95530824ee607",
	NetworkID:                       NetworkIDTestnet,
}
