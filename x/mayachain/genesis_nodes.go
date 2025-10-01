//go:build !mocknet && !stagenet && !regtest
// +build !mocknet,!stagenet,!regtest

package mayachain

var GenesisNodes = []string{
	"maya1q3jj8n8pkvl2kjv3pajdyju4hp92cmxnadknd2", // ThorStarter - kiasaki
	"maya10sy79jhw9hw9sqwdgu0k4mw4qawzl7czewzs47", // Independent - Alex
	"maya1gv85v0jvc0rsjunku3qxempax6kmrg5jqh8vmg", // Dash - Alex
	"maya1vm43yk3jq0evzn2u6a97mh2k9x4xf5mzp62g23", // QiCapital - Acro
	"maya12amvthg5jqv99j0w4pnmqwqnysgvgljxmazgnq", // ThorWallet - Matt
}
