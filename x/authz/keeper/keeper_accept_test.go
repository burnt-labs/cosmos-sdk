package keeper_test

import (
	"context"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttime "github.com/cometbft/cometbft/types/time"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	authztestutil "github.com/cosmos/cosmos-sdk/x/authz/testutil"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// DummyAccepter that just invokes the authorization.Accept method
type DummyAccepter struct {
	invocations int
}

// Accept implements the Accepter interface.
func (a *DummyAccepter) Accept(ctx context.Context, msg sdk.Msg, auth authz.Authorization) (authz.AcceptResponse, error) {
	a.invocations++
	return auth.Accept(ctx, msg)
}

// GetInvocations returns the number of times Accept was called.
func (a *DummyAccepter) GetInvocations() int {
	return a.invocations
}

type AccepterTestSuite struct {
	suite.Suite

	ctx           sdk.Context
	addrs         []sdk.AccAddress
	authzKeeper   authzkeeper.Keeper
	accountKeeper *authztestutil.MockAccountKeeper
	bankKeeper    *authztestutil.MockBankKeeper
	baseApp       *baseapp.BaseApp
	encCfg        moduletestutil.TestEncodingConfig
	dummyAccepter *DummyAccepter
}

func (s *AccepterTestSuite) SetupTest() {
	key := storetypes.NewKVStoreKey(authzkeeper.StoreKey)
	storeService := runtime.NewKVStoreService(key)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test"))
	s.ctx = testCtx.Ctx.WithBlockHeader(cmtproto.Header{Time: cmttime.Now()})
	s.encCfg = moduletestutil.MakeTestEncodingConfig(authzmodule.AppModuleBasic{})

	s.baseApp = baseapp.NewBaseApp(
		"authz",
		log.NewNopLogger(),
		testCtx.DB,
		s.encCfg.TxConfig.TxDecoder(),
	)
	s.baseApp.SetCMS(testCtx.CMS)
	s.baseApp.SetInterfaceRegistry(s.encCfg.InterfaceRegistry)

	s.addrs = simtestutil.CreateIncrementalAccounts(3)

	// gomock initializations
	ctrl := gomock.NewController(s.T())
	s.accountKeeper = authztestutil.NewMockAccountKeeper(ctrl)
	s.accountKeeper.EXPECT().AddressCodec().Return(address.NewBech32Codec("cosmos")).AnyTimes()

	s.bankKeeper = authztestutil.NewMockBankKeeper(ctrl)
	banktypes.RegisterInterfaces(s.encCfg.InterfaceRegistry)
	banktypes.RegisterMsgServer(s.baseApp.MsgServiceRouter(), s.bankKeeper)
	s.bankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false).AnyTimes()

	// Create dummy accepter
	s.dummyAccepter = &DummyAccepter{invocations: 0}

	// Create keeper with accepter
	s.authzKeeper = authzkeeper.NewKeeper(storeService, s.encCfg.Codec, s.baseApp.MsgServiceRouter(), s.accountKeeper).
		SetBankKeeper(s.bankKeeper).
		WithAccepter(s.dummyAccepter)
}

func (s *AccepterTestSuite) TestAccepterInvocationCount() {
	ctx, addrs := s.ctx, s.addrs
	require := s.Require()
	now := ctx.BlockTime()

	granterAddr := addrs[0]
	granteeAddr := addrs[1]
	recipientAddr := addrs[2]

	// Verify initial invocation count is 0
	require.Equal(0, s.dummyAccepter.GetInvocations(), "initial invocation count should be 0")

	// Grant authorization
	expire := now.AddDate(1, 0, 0)
	sendAuthz := banktypes.NewSendAuthorization(coins100, nil)
	err := s.authzKeeper.SaveGrant(ctx, granteeAddr, granterAddr, sendAuthz, &expire)
	require.NoError(err)

	// Verify accepter still not called
	require.Equal(0, s.dummyAccepter.GetInvocations(), "accepter should not be called on SaveGrant")

	// Execute a message that should trigger the accepter
	msgs := []sdk.Msg{
		&banktypes.MsgSend{
			Amount:      coins10,
			FromAddress: granterAddr.String(),
			ToAddress:   recipientAddr.String(),
		},
	}

	result, err := s.authzKeeper.DispatchActions(ctx, granteeAddr, msgs)
	require.NoError(err)
	require.NotNil(result)

	// Verify accepter was called exactly once
	require.Equal(1, s.dummyAccepter.GetInvocations(), "accepter should be called once for one message")

	// Execute another message
	msgs2 := []sdk.Msg{
		&banktypes.MsgSend{
			Amount:      coins10,
			FromAddress: granterAddr.String(),
			ToAddress:   recipientAddr.String(),
		},
	}

	result2, err := s.authzKeeper.DispatchActions(ctx, granteeAddr, msgs2)
	require.NoError(err)
	require.NotNil(result2)

	// Verify accepter was called again
	require.Equal(2, s.dummyAccepter.GetInvocations(), "accepter should be called twice for two messages")

	// Execute multiple messages in one call
	msgs3 := []sdk.Msg{
		&banktypes.MsgSend{
			Amount:      coins10,
			FromAddress: granterAddr.String(),
			ToAddress:   recipientAddr.String(),
		},
		&banktypes.MsgSend{
			Amount:      coins10,
			FromAddress: granterAddr.String(),
			ToAddress:   recipientAddr.String(),
		},
	}

	result3, err := s.authzKeeper.DispatchActions(ctx, granteeAddr, msgs3)
	require.NoError(err)
	require.NotNil(result3)

	// Verify accepter was called for each message
	require.Equal(4, s.dummyAccepter.GetInvocations(), "accepter should be called for each message")
}

func (s *AccepterTestSuite) TestAccepterNotCalledWhenGranterEqualsGrantee() {
	ctx, addrs := s.ctx, s.addrs
	require := s.Require()

	granterAddr := addrs[0]
	recipientAddr := addrs[2]

	// Reset invocation count
	s.dummyAccepter.invocations = 0

	// When granter == grantee, accepter should NOT be called (implicitly accepted)
	msgs := []sdk.Msg{
		&banktypes.MsgSend{
			Amount:      coins10,
			FromAddress: granterAddr.String(),
			ToAddress:   recipientAddr.String(),
		},
	}

	result, err := s.authzKeeper.DispatchActions(ctx, granterAddr, msgs)
	require.NoError(err)
	require.NotNil(result)

	// Verify accepter was NOT called when granter == grantee
	require.Equal(0, s.dummyAccepter.GetInvocations(), "accepter should not be called when granter equals grantee")
}

func (s *AccepterTestSuite) TestAccepterNotCalledWhenAuthorizationNotFound() {
	ctx, addrs := s.ctx, s.addrs
	require := s.Require()

	granterAddr := addrs[0]
	granteeAddr := addrs[1]
	recipientAddr := addrs[2]

	// Reset invocation count
	s.dummyAccepter.invocations = 0

	// Try to execute without granting authorization first
	msgs := []sdk.Msg{
		&banktypes.MsgSend{
			Amount:      coins10,
			FromAddress: granterAddr.String(),
			ToAddress:   recipientAddr.String(),
		},
	}

	result, err := s.authzKeeper.DispatchActions(ctx, granteeAddr, msgs)
	require.Error(err)
	require.Nil(result)
	require.Contains(err.Error(), "authorization not found")

	// Verify accepter was NOT called when authorization is not found
	require.Equal(0, s.dummyAccepter.GetInvocations(), "accepter should not be called when authorization is not found")
}

func (s *AccepterTestSuite) TestAccepterNotCalledWhenAuthorizationExpired() {
	ctx, addrs := s.ctx, s.addrs
	require := s.Require()
	now := ctx.BlockTime()

	granterAddr := addrs[0]
	granteeAddr := addrs[1]
	recipientAddr := addrs[2]

	// Grant authorization with expiration in the future first (required by SaveGrant validation)
	expire := now.AddDate(0, 0, 1) // expires in 1 day
	sendAuthz := banktypes.NewSendAuthorization(coins100, nil)
	err := s.authzKeeper.SaveGrant(ctx, granteeAddr, granterAddr, sendAuthz, &expire)
	require.NoError(err)

	// Reset invocation count
	s.dummyAccepter.invocations = 0

	// Advance block time to after expiration
	expiredCtx := ctx.WithBlockTime(now.AddDate(0, 0, 2))

	// Try to execute with expired authorization
	msgs := []sdk.Msg{
		&banktypes.MsgSend{
			Amount:      coins10,
			FromAddress: granterAddr.String(),
			ToAddress:   recipientAddr.String(),
		},
	}

	result, err := s.authzKeeper.DispatchActions(expiredCtx, granteeAddr, msgs)
	require.Error(err)
	require.Nil(result)
	require.Contains(err.Error(), "authorization expired")

	// Verify accepter was NOT called when authorization is expired
	require.Equal(0, s.dummyAccepter.GetInvocations(), "accepter should not be called when authorization is expired")
}

func (s *AccepterTestSuite) TestAccepterNilFallsBackToAuthorizationAccept() {
	ctx, addrs := s.ctx, s.addrs
	require := s.Require()
	now := ctx.BlockTime()

	granterAddr := addrs[0]
	granteeAddr := addrs[1]
	recipientAddr := addrs[2]

	// Create keeper WITHOUT accepter (nil accepter)
	key := storetypes.NewKVStoreKey(authzkeeper.StoreKey + "_nil_test")
	storeService := runtime.NewKVStoreService(key)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test2"))
	testCtx2 := testCtx.Ctx.WithBlockHeader(cmtproto.Header{Time: cmttime.Now()})

	ctrl := gomock.NewController(s.T())
	accountKeeper := authztestutil.NewMockAccountKeeper(ctrl)
	accountKeeper.EXPECT().AddressCodec().Return(address.NewBech32Codec("cosmos")).AnyTimes()

	bankKeeper := authztestutil.NewMockBankKeeper(ctrl)
	// Create a new baseApp to avoid duplicate registration
	baseApp2 := baseapp.NewBaseApp(
		"authz2",
		log.NewNopLogger(),
		testCtx.DB,
		s.encCfg.TxConfig.TxDecoder(),
	)
	baseApp2.SetCMS(testCtx.CMS)
	baseApp2.SetInterfaceRegistry(s.encCfg.InterfaceRegistry)
	banktypes.RegisterInterfaces(s.encCfg.InterfaceRegistry)
	banktypes.RegisterMsgServer(baseApp2.MsgServiceRouter(), bankKeeper)
	bankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false).AnyTimes()

	// Create keeper WITHOUT accepter (should fall back to authorization.Accept)
	keeperWithoutAccepter := authzkeeper.NewKeeper(storeService, s.encCfg.Codec, baseApp2.MsgServiceRouter(), accountKeeper).
		SetBankKeeper(bankKeeper)
	// Note: NOT calling WithAccepter, so accepter is nil

	// Grant authorization
	expire := now.AddDate(1, 0, 0)
	sendAuthz := banktypes.NewSendAuthorization(coins100, nil)
	err := keeperWithoutAccepter.SaveGrant(testCtx2, granteeAddr, granterAddr, sendAuthz, &expire)
	require.NoError(err)

	// Execute a message - should work without accepter (falls back to authorization.Accept)
	msgs := []sdk.Msg{
		&banktypes.MsgSend{
			Amount:      coins10,
			FromAddress: granterAddr.String(),
			ToAddress:   recipientAddr.String(),
		},
	}

	result, err := keeperWithoutAccepter.DispatchActions(testCtx2, granteeAddr, msgs)
	require.NoError(err)
	require.NotNil(result)

	// Verify our dummy accepter was NOT called (it's not even set on this keeper)
	require.Equal(0, s.dummyAccepter.GetInvocations(), "dummy accepter should not be called when keeper has nil accepter")
}

func TestAccepterTestSuite(t *testing.T) {
	suite.Run(t, new(AccepterTestSuite))
}
