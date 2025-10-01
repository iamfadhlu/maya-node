package mayachain

import (
	cosmos "gitlab.com/mayachain/mayanode/common/cosmos"
)

type DummyYggManager struct{}

func NewDummyYggManger() *DummyYggManager {
	return &DummyYggManager{}
}

func (DummyYggManager) Fund(ctx cosmos.Context, mgr Manager) error {
	return errKaboom
}
