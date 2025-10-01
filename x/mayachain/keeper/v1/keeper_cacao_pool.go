package keeperv1

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common/cosmos"
)

////////////////////////////////////////////////////////////////////////////////////////
// CACAOPool
////////////////////////////////////////////////////////////////////////////////////////

func (k KVStore) GetCACAOPool(ctx cosmos.Context) (CACAOPool, error) {
	record := NewCACAOPool()
	key := k.GetKey(ctx, prefixCACAOPool, "")

	store := ctx.KVStore(k.storeKey)
	if !store.Has([]byte(key)) {
		return record, nil
	}

	bz := store.Get([]byte(key))
	if err := k.cdc.Unmarshal(bz, &record); err != nil {
		return record, dbError(ctx, fmt.Sprintf("Unmarshal kvstore: (%T) %s", record, key), err)
	}
	return record, nil
}

func (k KVStore) SetCACAOPool(ctx cosmos.Context, pool CACAOPool) {
	store := ctx.KVStore(k.storeKey)
	key := k.GetKey(ctx, prefixCACAOPool, "")
	buf := k.cdc.MustMarshal(&pool)
	store.Set([]byte(key), buf)
}

////////////////////////////////////////////////////////////////////////////////////////
// CACAOProviders
////////////////////////////////////////////////////////////////////////////////////////

func (k KVStore) setCACAOProvider(ctx cosmos.Context, key string, record CACAOProvider) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getCACAOProvider(ctx cosmos.Context, key string, record *CACAOProvider) (bool, error) {
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

// GetCACAOProviderIterator iterate CACAO providers
func (k KVStore) GetCACAOProviderIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixCACAOProvider)
}

// GetCACAOProvider retrieve CACAO provider from the data store
func (k KVStore) GetCACAOProvider(ctx cosmos.Context, addr cosmos.AccAddress) (CACAOProvider, error) {
	record := CACAOProvider{
		CacaoAddress:   addr,
		DepositAmount:  cosmos.ZeroUint(),
		WithdrawAmount: cosmos.ZeroUint(),
		Units:          cosmos.ZeroUint(),
	}

	_, err := k.getCACAOProvider(ctx, k.GetKey(ctx, prefixCACAOProvider, record.Key()), &record)
	return record, err
}

// SetCACAOProvider save the CACAO provider to kv store
func (k KVStore) SetCACAOProvider(ctx cosmos.Context, rp CACAOProvider) {
	k.setCACAOProvider(ctx, k.GetKey(ctx, prefixCACAOProvider, rp.Key()), rp)
}

// RemoveCACAOProvider remove the CACAO provider from the kv store
func (k KVStore) RemoveCACAOProvider(ctx cosmos.Context, rp CACAOProvider) {
	k.del(ctx, k.GetKey(ctx, prefixCACAOProvider, rp.Key()))
}
