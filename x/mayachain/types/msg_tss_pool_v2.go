package types

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"gitlab.com/mayachain/mayanode/common"
	"gitlab.com/mayachain/mayanode/common/cosmos"
)

// NewMsgTssPool is a constructor function for MsgTssPool
func NewMsgTssPoolV2(
	pks []string,
	poolpk common.PubKey,
	keysharesBackup []byte,
	keygenType KeygenType,
	height int64,
	bl []Blame,
	chains []string,
	signer cosmos.AccAddress,
	keygenTime int64,
	poolPubKeyEddsa common.PubKey,
	keysharesBackupEddsa []byte,
) (*MsgTssPool, error) {
	id, err := getTssIDV2(pks, poolpk, height, bl)
	if err != nil {
		return nil, fmt.Errorf("fail to get tss id: %w", err)
	}
	// Convert []Blame to []*Blame
	blamePointers := make([]*Blame, len(bl))
	for i := range bl {
		blamePointers[i] = &bl[i]
	}
	return &MsgTssPool{
		ID:                   id,
		PubKeys:              pks,
		PoolPubKey:           poolpk,
		PoolPubKeyEddsa:      poolPubKeyEddsa,
		Height:               height,
		KeygenType:           keygenType,
		Blame:                blamePointers,
		Chains:               chains,
		Signer:               signer,
		KeygenTime:           keygenTime,
		KeysharesBackup:      keysharesBackup,
		KeysharesBackupEddsa: keysharesBackupEddsa,
	}, nil
}

// getTssID
func getTssIDV2(members []string, poolPk common.PubKey, height int64, bl []Blame) (string, error) {
	// ensure input pubkeys list is deterministically sorted
	sort.SliceStable(members, func(i, j int) bool {
		return members[i] < members[j]
	})

	pubkeys := make([]string, 0)
	for _, b := range bl {
		for _, node := range b.BlameNodes {
			pubkeys = append(pubkeys, node.Pubkey)
		}
	}
	sort.SliceStable(pubkeys, func(i, j int) bool {
		return pubkeys[i] < pubkeys[j]
	})

	sb := strings.Builder{}
	for _, item := range members {
		sb.WriteString("m:" + item)
	}
	for _, item := range pubkeys {
		sb.WriteString("p:" + item)
	}
	sb.WriteString(poolPk.String())
	sb.WriteString(fmt.Sprintf("%d", height))
	hash := sha256.New()
	_, err := hash.Write([]byte(sb.String()))
	if err != nil {
		return "", fmt.Errorf("fail to get tss id: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// IsSuccess when blame is empty , then treat it as success
func (m MsgTssPool) IsSuccessV2() bool {
	return len(m.Blame) == 0
}
