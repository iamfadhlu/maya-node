package watchers

import (
	"strings"

	"gitlab.com/mayachain/mayanode/config"
)

////////////////////////////////////////////////////////////////////////////////////////
// Init
////////////////////////////////////////////////////////////////////////////////////////

var mayanodeURL string

func init() {
	config.Init()
	mayanodeURL = config.GetBifrost().MayaChain.ChainHost
	if !strings.HasPrefix(mayanodeURL, "http") {
		mayanodeURL = "http://" + mayanodeURL
	}
}
