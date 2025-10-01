//go:build !testnet && !mocknet && !stagenet
// +build !testnet,!mocknet,!stagenet

package mayachain

var (
	// ETH Router
	ethUSDTAsset = `ETH.USDT-0XDAC17F958D2EE523A2206206994597C13D831EC7`
	// https://etherscan.io/address/0x3624525075b88B24ecc29CE226b0CEc1fFcB6976
	ethOldRouter = ``
	// https://etherscan.io/address/0xD37BbE5744D730a1d98d8DC97c42F0Ca46aD7146
	ethNewRouter = `0xefcb6efd89013d1a825c4bfc5a781266bcc78021`
	// AVAX Router
	avaxOldRouter = ``
	// https://snowtrace.io/address/0x8F66c4AE756BEbC49Ec8B81966DD8bba9f127549#code
	avaxNewRouter = ``
	// ARB Router
	arbOldRouter = ``
	// https://arbiscan.io/address/0x700E97ef07219440487840Dc472E7120A7FF11F4#code
	arbNewRouter = `0x700E97ef07219440487840Dc472E7120A7FF11F4`

	// XRD Router
	xrdOldRouter = ``
	xrdNewRouter = `component_rdx1cp7hrk7k0pjavnpt5h6dsel096kzlj96r8ukw2ywqgdc5tlvpvn0as`
)
