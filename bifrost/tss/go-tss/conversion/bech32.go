package conversion

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func SetupBech32Prefix() {
	config := sdk.GetConfig()
	// mayachain will import go-tss as a library , thus this is not needed, we copy the prefix here to avoid go-tss to import mayachain
	config.SetBech32PrefixForAccount("maya", "mayapub")
	config.SetBech32PrefixForValidator("mayav", "mayavpub")
	config.SetBech32PrefixForConsensusNode("mayac", "mayacpub")
}
