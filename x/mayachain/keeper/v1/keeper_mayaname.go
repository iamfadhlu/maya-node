package keeperv1

import (
	"fmt"

	"github.com/blang/semver"

	"gitlab.com/mayachain/mayanode/common/cosmos"
	"gitlab.com/mayachain/mayanode/constants"
)

func (k KVStore) setMAYAName(ctx cosmos.Context, key string, record MAYAName) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getMAYAName(ctx cosmos.Context, key string, record *MAYAName) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, record); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}

	return true, nil
}

// GetMAYANameIterator only iterate MAYANames
func (k KVStore) GetMAYANameIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixMAYAName)
}

// SetMAYAName save the MAYAName object to store
func (k KVStore) SetMAYAName(ctx cosmos.Context, name MAYAName) {
	k.setMAYAName(ctx, k.GetKey(ctx, prefixMAYAName, name.Key()), name)
}

// TODO: remove after adding constants access to keeper (https://gitlab.com/mayachain/mayanode/-/issues/58)
func (k KVStore) getMAYANameGracePeriod(ctx cosmos.Context) int64 {
	gracePeriod, err := k.GetMimir(ctx, constants.MAYANameGracePeriodBlocks.String())
	if gracePeriod < 0 || err != nil {
		constAccessor := k.GetConstants()
		if constAccessor == nil {
			constAccessor = constants.GetConstantValues(k.version)
		}
		gracePeriod = constAccessor.GetInt64Value(constants.MAYANameGracePeriodBlocks)
	}
	return gracePeriod
}

// MAYANameExists check whether the given name exists
func (k KVStore) MAYANameExists(ctx cosmos.Context, name string) bool {
	record := MAYAName{
		Name: name,
	}
	if k.has(ctx, k.GetKey(ctx, prefixMAYAName, record.Key())) {
		record, _ = k.GetMAYAName(ctx, name)
		expiration := record.ExpireBlockHeight
		if k.GetVersion().GTE(semver.MustParse("1.112.0")) {
			gracePeriod := k.getMAYANameGracePeriod(ctx)
			expiration = record.ExpireBlockHeight + gracePeriod
		}

		return expiration >= ctx.BlockHeight()
	}
	return false
}

// GetMAYAName get MAYAName with the given pubkey from data store
func (k KVStore) GetMAYAName(ctx cosmos.Context, name string) (MAYAName, error) {
	record := MAYAName{
		Name: name,
	}
	ok, err := k.getMAYAName(ctx, k.GetKey(ctx, prefixMAYAName, record.Key()), &record)
	if !ok {
		return record, fmt.Errorf("MAYAName doesn't exist: %s", name)
	}
	expiration := record.ExpireBlockHeight
	if k.GetVersion().GTE(semver.MustParse("1.112.0")) {
		gracePeriod := k.getMAYANameGracePeriod(ctx)
		expiration = record.ExpireBlockHeight + gracePeriod
	}

	if expiration < ctx.BlockHeight() {
		return MAYAName{Name: name}, nil
	}
	return record, err
}

// DeleteMAYAName remove the given MAYAName from data store
func (k KVStore) DeleteMAYAName(ctx cosmos.Context, name string) error {
	n := MAYAName{Name: name}
	k.del(ctx, k.GetKey(ctx, prefixMAYAName, n.Key()))
	return nil
}

// AffiliateFeeCollector

func (k KVStore) setAffiliateCollector(ctx cosmos.Context, key string, record AffiliateFeeCollector) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getAffiliateCollector(ctx cosmos.Context, key string, record *AffiliateFeeCollector) (bool, error) {
	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return false, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, record); err != nil {
		return true, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}

	return true, nil
}

func (k KVStore) SetAffiliateCollector(ctx cosmos.Context, collector AffiliateFeeCollector) {
	k.setAffiliateCollector(ctx, k.GetKey(ctx, prefixAffiliateCollector, collector.OwnerAddress.String()), collector)
}

func (k KVStore) GetAffiliateCollector(ctx cosmos.Context, acc cosmos.AccAddress) (AffiliateFeeCollector, error) {
	record := NewAffiliateFeeCollector(acc, cosmos.ZeroUint())
	_, err := k.getAffiliateCollector(ctx, k.GetKey(ctx, prefixAffiliateCollector, record.OwnerAddress.String()), &record)
	return record, err
}

func (k KVStore) GetAffiliateCollectorIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixAffiliateCollector)
}

func (k KVStore) GetAffiliateCollectors(ctx cosmos.Context) ([]AffiliateFeeCollector, error) {
	var affCols []AffiliateFeeCollector
	iterator := k.GetAffiliateCollectorIterator(ctx)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		var ac AffiliateFeeCollector
		err := k.Cdc().Unmarshal(iterator.Value(), &ac)
		if err != nil {
			return nil, dbError(ctx, "Unmarshal: ac", err)
		}
		affCols = append(affCols, ac)
	}
	return affCols, nil
}
