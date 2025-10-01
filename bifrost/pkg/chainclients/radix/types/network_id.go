package types

import "gitlab.com/mayachain/mayanode/common"

type Network struct {
	Id          uint8
	LogicalName string
}

func NetworkFromChainNetwork(chainNetwork common.ChainNetwork) Network {
	switch chainNetwork {
	case common.MainNet, common.StageNet:
		return Network{Id: 1, LogicalName: "mainnet"}
	case common.TestNet:
		return Network{Id: 34, LogicalName: "hammunet"}
	case common.MockNet:
		return Network{Id: 240, LogicalName: "localnet"}
	default:
		return Network{}
	}
}
