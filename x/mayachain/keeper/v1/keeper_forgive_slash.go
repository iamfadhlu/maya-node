package keeperv1

import (
	"fmt"

	"gitlab.com/mayachain/mayanode/common/cosmos"
)

func (k KVStore) setForgiveSlashVoter(ctx cosmos.Context, key string, record ForgiveSlashVoter) {
	store := ctx.KVStore(k.storeKey)
	buf := k.cdc.MustMarshal(&record)
	if buf == nil {
		store.Delete([]byte(key))
	} else {
		store.Set([]byte(key), buf)
	}
}

func (k KVStore) getForgiveSlashVoter(ctx cosmos.Context, key string, record *ForgiveSlashVoter) (bool, error) {
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

// SetForgiveSlashVoter - save a forgiveSlash voter object
func (k KVStore) SetForgiveSlashVoter(ctx cosmos.Context, forgiveSlash ForgiveSlashVoter) {
	k.setForgiveSlashVoter(ctx, k.GetKey(ctx, prefixForgiveSlashVoter, forgiveSlash.String()), forgiveSlash)
}

// GetForgiveSlashVoter - gets information of forgiveSlash voter
func (k KVStore) GetForgiveSlashVoter(ctx cosmos.Context, addr cosmos.AccAddress) (ForgiveSlashVoter, error) {
	record := NewForgiveSlashVoter(addr)
	_, err := k.getForgiveSlashVoter(ctx, k.GetKey(ctx, prefixForgiveSlashVoter, record.String()), &record)
	return record, err
}

// DeleteSlashVoter - deletes a foriveSlash voter object
func (k KVStore) DeleteForgiveSlashVoter(ctx cosmos.Context, addr cosmos.AccAddress) error {
	record := NewForgiveSlashVoter(addr)
	k.del(ctx, k.GetKey(ctx, prefixForgiveSlashVoter, record.String()))
	return nil
}

// GetForgiveSlashVoterIterator - get an iterator for forgiveSlash voter
func (k KVStore) GetForgiveSlashVoterIterator(ctx cosmos.Context) cosmos.Iterator {
	return k.getIterator(ctx, prefixForgiveSlashVoter)
}
