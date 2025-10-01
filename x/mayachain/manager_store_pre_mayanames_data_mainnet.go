//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package mayachain

import _ "embed"

//go:embed preregister_mayanames.json
var preregisterMAYANames []byte
