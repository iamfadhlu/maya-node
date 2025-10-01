package keeperv1

import (
	. "gopkg.in/check.v1"

	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
)

type KeeperCACAOPoolSuite struct{}

//nolint:typecheck
var _ = Suite(&KeeperCACAOPoolSuite{})

func (mas *KeeperCACAOPoolSuite) SetUpSuite(c *C) {
	SetupConfigForTest()
}

func (s *KeeperCACAOPoolSuite) TestCACAOProvider(c *C) {
	ctx, k := setupKeeperForTest(c)

	addr := GetRandomBaseAddress()
	accAddr, err := addr.AccAddress()
	c.Check(err, IsNil)
	rp, err := k.GetCACAOProvider(ctx, accAddr)
	c.Assert(err, IsNil)
	//nolint:typecheck
	c.Check(rp.CacaoAddress, NotNil)
	//nolint:typecheck
	c.Check(rp.Units, NotNil)

	addr = GetRandomBaseAddress()
	accAddr, err = addr.AccAddress()
	c.Assert(err, IsNil)
	rp = CACAOProvider{
		Units:         cosmos.NewUint(12),
		DepositAmount: cosmos.NewUint(12),
		CacaoAddress:  accAddr,
	}
	k.SetCACAOProvider(ctx, rp)
	rp, err = k.GetCACAOProvider(ctx, rp.CacaoAddress)
	c.Assert(err, IsNil)
	c.Check(rp.CacaoAddress.Equals(accAddr), Equals, true)
	c.Check(rp.Units.Equal(cosmos.NewUint(12)), Equals, true)
	c.Check(rp.DepositAmount.Equal(cosmos.NewUint(12)), Equals, true)
	c.Check(rp.WithdrawAmount.Equal(cosmos.NewUint(0)), Equals, true)

	var rps []CACAOProvider
	iterator := k.GetCACAOProviderIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		k.Cdc().MustUnmarshal(iterator.Value(), &rp)
		if rp.CacaoAddress.Empty() {
			continue
		}
		rps = append(rps, rp)
	}
	c.Check(rps[0].CacaoAddress.Equals(accAddr), Equals, true)

	secondAddr := GetRandomBaseAddress()
	secondAccAddr, err := secondAddr.AccAddress()
	c.Check(err, IsNil)
	rp2 := CACAOProvider{
		Units:         cosmos.NewUint(24),
		DepositAmount: cosmos.NewUint(24),
		CacaoAddress:  secondAccAddr,
	}
	k.SetCACAOProvider(ctx, rp2)

	rps = []CACAOProvider{}
	iterator = k.GetCACAOProviderIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		k.Cdc().MustUnmarshal(iterator.Value(), &rp)
		if rp.CacaoAddress.Empty() {
			continue
		}
		rps = append(rps, rp)
	}
	c.Check(len(rps), Equals, 2)

	totalUnits := cosmos.ZeroUint()
	iterator = k.GetCACAOProviderIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		k.Cdc().MustUnmarshal(iterator.Value(), &rp)
		if rp.CacaoAddress.Empty() {
			continue
		}
		totalUnits = totalUnits.Add(rp.Units)
	}
	c.Check(totalUnits.Equal(cosmos.NewUint(36)), Equals, true)

	k.RemoveCACAOProvider(ctx, rp)
	k.RemoveCACAOProvider(ctx, rp2)
}
