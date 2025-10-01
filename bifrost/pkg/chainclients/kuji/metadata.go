package kuji

import (
	"sync"

	"gitlab.com/mayachain/mayanode/common"
)

type KujiMetadata struct {
	AccountNumber int64
	SeqNumber     int64
	BlockHeight   int64
}

type KujiMetadataStore struct {
	lock  *sync.Mutex
	accts map[common.PubKey]KujiMetadata
}

func NewKujiMetaDataStore() *KujiMetadataStore {
	return &KujiMetadataStore{
		lock:  &sync.Mutex{},
		accts: make(map[common.PubKey]KujiMetadata),
	}
}

func (b *KujiMetadataStore) Get(pk common.PubKey) KujiMetadata {
	b.lock.Lock()
	defer b.lock.Unlock()
	if val, ok := b.accts[pk]; ok {
		return val
	}
	return KujiMetadata{}
}

func (b *KujiMetadataStore) GetByAccount(acct int64) KujiMetadata {
	b.lock.Lock()
	defer b.lock.Unlock()
	for _, meta := range b.accts {
		if meta.AccountNumber == acct {
			return meta
		}
	}
	return KujiMetadata{}
}

func (b *KujiMetadataStore) Set(pk common.PubKey, meta KujiMetadata) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.accts[pk] = meta
}

func (b *KujiMetadataStore) SeqInc(pk common.PubKey) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if meta, ok := b.accts[pk]; ok {
		meta.SeqNumber++
		b.accts[pk] = meta
	}
}
