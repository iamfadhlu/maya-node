package types

import (
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func NewCACAOProvider(addr cosmos.AccAddress) CACAOProvider {
	return CACAOProvider{
		CacaoAddress: addr,
		Units:        cosmos.ZeroUint(),
	}
}

func (rp CACAOProvider) Key() string {
	return rp.CacaoAddress.String()
}
