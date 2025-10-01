package types

import "gitlab.com/mayachain/mayanode/common"

// TODO replace to thorNode's code once endpoint is build.
type Vaults struct {
	Asgard    []common.PubKey `json:"asgard"`
	Yggdrasil []common.PubKey `json:"yggdrasil"`
}
