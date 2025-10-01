package mayachain

import (
	"fmt"

	se "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/hashicorp/go-multierror"
)

// BASEChain error code start at 99
const (
	// CodeBadVersion error code for bad version
	CodeInternalError     uint32 = 99
	CodeTxFail            uint32 = 100
	CodeBadVersion        uint32 = 101
	CodeInvalidMessage    uint32 = 102
	CodeInvalidVault      uint32 = 104
	CodeInvalidMemo       uint32 = 105
	CodeInvalidPoolStatus uint32 = 107

	CodeSwapFail                 uint32 = 108
	CodeSwapFailNotEnoughFee     uint32 = 110
	CodeSwapFailInvalidAmount    uint32 = 113
	CodeSwapFailInvalidBalance   uint32 = 114
	CodeSwapFailNotEnoughBalance uint32 = 115

	CodeAddLiquidityFailValidation    uint32 = 120
	CodeFailGetLiquidityProvider      uint32 = 122
	CodeAddLiquidityMismatchAddr      uint32 = 123
	CodeLiquidityInvalidPoolAsset     uint32 = 124
	CodeAddLiquidityRUNEOverLimit     uint32 = 125
	CodeAddLiquidityCacaoMoreThanBond uint32 = 126

	CodeWithdrawFailValidation         uint32 = 130
	CodeFailAddOutboundTx              uint32 = 131
	CodeFailSaveEvent                  uint32 = 132
	CodeNoLiquidityUnitLeft            uint32 = 135
	CodeWithdrawWithin24Hours          uint32 = 136
	CodeWithdrawFail                   uint32 = 137
	CodeEmptyChain                     uint32 = 138
	CodeWithdrawFailIsBonderValidation uint32 = 139
	CodeMaxWithdrawReach               uint32 = 140
	CodeMaxWithdrawWillBeReach         uint32 = 141
	CodeInvalidTier                    uint32 = 142
	CodeWithdrawLiquidityMismatchAddr  uint32 = 143
	CodeInvalidSymbolicNodeMimirValue  uint32 = 144
)

var (
	errNotAuthorized                  = fmt.Errorf("not authorized")
	errInvalidVersion                 = fmt.Errorf("bad version")
	errBadVersion                     = se.Register(DefaultCodespace, CodeBadVersion, errInvalidVersion.Error())
	errInvalidMessage                 = se.Register(DefaultCodespace, CodeInvalidMessage, "invalid message")
	errInvalidMemo                    = se.Register(DefaultCodespace, CodeInvalidMemo, "invalid memo")
	errFailSaveEvent                  = se.Register(DefaultCodespace, CodeFailSaveEvent, "fail to save add events")
	errAddLiquidityFailValidation     = se.Register(DefaultCodespace, CodeAddLiquidityFailValidation, "fail to validate add liquidity")
	errAddLiquidityRUNEOverLimit      = se.Register(DefaultCodespace, CodeAddLiquidityRUNEOverLimit, "add liquidity rune is over limit")
	errAddLiquidityCACAOMoreThanBond  = se.Register(DefaultCodespace, CodeAddLiquidityCacaoMoreThanBond, "add liquidity cacao is more than bond")
	errInvalidPoolStatus              = se.Register(DefaultCodespace, CodeInvalidPoolStatus, "invalid pool status")
	errFailAddOutboundTx              = se.Register(DefaultCodespace, CodeFailAddOutboundTx, "prepare outbound tx not successful")
	errWithdrawFailValidation         = se.Register(DefaultCodespace, CodeWithdrawFailValidation, "fail to validate withdraw")
	errWithdrawFailIsBonderValidation = se.Register(DefaultCodespace, CodeWithdrawFailIsBonderValidation, "bonded liquidity may not be withdrawn")
	errFailGetLiquidityProvider       = se.Register(DefaultCodespace, CodeFailGetLiquidityProvider, "fail to get liquidity provider")
	errAddLiquidityMismatchAddr       = se.Register(DefaultCodespace, CodeAddLiquidityMismatchAddr, "mismatch of address")
	errWithdrawLiquidityMismatchAddr  = se.Register(DefaultCodespace, CodeWithdrawLiquidityMismatchAddr, "mismatch of address")
	errSwapFailNotEnoughFee           = se.Register(DefaultCodespace, CodeSwapFailNotEnoughFee, "fail swap, not enough fee")
	errSwapFailInvalidAmount          = se.Register(DefaultCodespace, CodeSwapFailInvalidAmount, "fail swap, invalid amount")
	errSwapFailInvalidBalance         = se.Register(DefaultCodespace, CodeSwapFailInvalidBalance, "fail swap, invalid balance")
	errSwapFailNotEnoughBalance       = se.Register(DefaultCodespace, CodeSwapFailNotEnoughBalance, "fail swap, not enough balance")
	errNoLiquidityUnitLeft            = se.Register(DefaultCodespace, CodeNoLiquidityUnitLeft, "nothing to withdraw")
	errWithdrawWithin24Hours          = se.Register(DefaultCodespace, CodeWithdrawWithin24Hours, "you cannot withdraw for 24 hours after providing liquidity for this blockchain")
	errWithdrawFail                   = se.Register(DefaultCodespace, CodeWithdrawFail, "fail to withdraw")
	errInternal                       = se.Register(DefaultCodespace, CodeInternalError, "internal error")
	errMaxWithdrawReach               = se.Register(DefaultCodespace, CodeMaxWithdrawReach, "address already reached the max amount of withdraw")
	errMaxWithdrawWillBeReach         = se.Register(DefaultCodespace, CodeMaxWithdrawWillBeReach, "max amount to withdraw exceeded")
	errInvalidSymbolicNodeMimirValue  = se.Register(DefaultCodespace, CodeInvalidSymbolicNodeMimirValue, "invalid symbolic node mimir value")
)

// ErrInternal return an error  of errInternal with additional message
func ErrInternal(err error, msg string) error {
	return se.Wrap(multierror.Append(errInternal, err), msg)
}
