package signer

import (
	"errors"
	"testing"

	. "gopkg.in/check.v1"

	"gitlab.com/mayachain/mayanode/bifrost/mayaclient"
	"gitlab.com/mayachain/mayanode/constants"
)

type MockMayachainBridge struct {
	mayaclient.MayachainBridge
	constants    map[string]int64
	shouldError  bool
	getCallCount int
}

func (m *MockMayachainBridge) GetConstants() (map[string]int64, error) {
	m.getCallCount++
	if m.shouldError {
		return nil, errors.New("mock error")
	}
	return m.constants, nil
}

type ConstantsProviderSuite struct {
	bridge   *MockMayachainBridge
	provider *ConstantsProvider
}

var _ = Suite(&ConstantsProviderSuite{})

func TestPackage(t *testing.T) { TestingT(t) }

func (s *ConstantsProviderSuite) SetUpTest(c *C) {
	s.bridge = &MockMayachainBridge{
		constants: map[string]int64{
			constants.ChurnInterval.String(): 100,
			"SomeOtherConstant":              200,
		},
	}
	s.provider = NewConstantsProvider(s.bridge)
}

func (s *ConstantsProviderSuite) TestNewConstantsProvider(c *C) {
	provider := NewConstantsProvider(s.bridge)
	c.Assert(provider.constants, NotNil)
	c.Assert(provider.requestHeight, Equals, int64(0))
	c.Assert(provider.bridge, Equals, s.bridge)
	c.Assert(provider.constantsLock, NotNil)
}

func (s *ConstantsProviderSuite) TestGetInt64Value_Success(c *C) {
	val, err := s.provider.GetInt64Value(1, constants.ChurnInterval)
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(100))
	c.Assert(s.bridge.getCallCount, Equals, 1)
}

func (s *ConstantsProviderSuite) TestGetInt64Value_CacheHit(c *C) {
	// First call should fetch from bridge
	_, err := s.provider.GetInt64Value(1, constants.ChurnInterval)
	c.Assert(err, IsNil)

	// Second call within churn interval should use cache
	val, err := s.provider.GetInt64Value(50, constants.ChurnInterval)
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(100))
	c.Assert(s.bridge.getCallCount, Equals, 1) // Should still be 1 as we used cache
}

func (s *ConstantsProviderSuite) TestGetInt64Value_RefetchAfterChurnInterval(c *C) {
	// First call
	_, err := s.provider.GetInt64Value(1, constants.ChurnInterval)
	c.Assert(err, IsNil)

	// Call after churn interval should refetch
	val, err := s.provider.GetInt64Value(102, constants.ChurnInterval)
	c.Assert(err, IsNil)
	c.Assert(val, Equals, int64(100))
	c.Assert(s.bridge.getCallCount, Equals, 2) // Should be 2 as we fetched again
}

func (s *ConstantsProviderSuite) TestGetInt64Value_BridgeError(c *C) {
	s.bridge.shouldError = true
	_, err := s.provider.GetInt64Value(1, constants.ChurnInterval)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, "fail to get constants from mayachain: fail to get constants: mock error")
}

func (s *ConstantsProviderSuite) TestEnsureConstants_InitialFetch(c *C) {
	err := s.provider.EnsureConstants(1)
	c.Assert(err, IsNil)
	c.Assert(s.provider.requestHeight, Equals, int64(1))
	c.Assert(s.bridge.getCallCount, Equals, 1)
}

func (s *ConstantsProviderSuite) TestEnsureConstants_WithinChurnInterval(c *C) {
	// First fetch
	err := s.provider.EnsureConstants(1)
	c.Assert(err, IsNil)

	// Second fetch within churn interval
	err = s.provider.EnsureConstants(50)
	c.Assert(err, IsNil)
	c.Assert(s.bridge.getCallCount, Equals, 1) // Should still be 1 as we're within interval
}

func (s *ConstantsProviderSuite) TestEnsureConstants_AfterChurnInterval(c *C) {
	// First fetch
	err := s.provider.EnsureConstants(1)
	c.Assert(err, IsNil)

	// Fetch after churn interval
	err = s.provider.EnsureConstants(102)
	c.Assert(err, IsNil)
	c.Assert(s.bridge.getCallCount, Equals, 2) // Should be 2 as we fetched again
}

func (s *ConstantsProviderSuite) TestEnsureConstants_BridgeError(c *C) {
	s.bridge.shouldError = true
	err := s.provider.EnsureConstants(1)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, "fail to get constants: mock error")
}
