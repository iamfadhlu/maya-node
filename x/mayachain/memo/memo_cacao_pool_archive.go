package mayachain

import (
	"gitlab.com/mayachain/mayanode/common"
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func (p *parser) ParseCacaoPoolWithdrawMemoV1() (CacaoPoolWithdrawMemo, error) {
	basisPoints := p.getUint(1, true, cosmos.ZeroInt().Uint64())
	affiliateAddress := p.getAddressWithKeeper(2, false, common.NoAddress, common.BASEChain, p.version)
	affiliateBasisPoints := p.getUintWithMaxValue(3, false, 0, constants.MaxBasisPts)
	return NewCacaoPoolWithdrawMemo(basisPoints, affiliateBasisPoints, []string{affiliateAddress.String()}, []cosmos.Uint{affiliateBasisPoints}), p.Error()
}
