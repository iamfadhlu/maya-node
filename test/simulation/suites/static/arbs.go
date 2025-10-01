package static

import (
	"gitlab.com/mayachain/mayanode/test/simulation/actors"
	. "gitlab.com/mayachain/mayanode/test/simulation/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////////////
// Arb
////////////////////////////////////////////////////////////////////////////////////////

func Arbs() *Actor {
	a := &Actor{
		Name: "Swaps",
	}

	// add one arber
	a.Children = append(a.Children,
		actors.NewArbActor(),
	)

	return a
}
