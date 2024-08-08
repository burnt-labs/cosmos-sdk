package v6_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections"
	coretesting "cosmossdk.io/core/testing"
	"cosmossdk.io/x/gov"
	v6 "cosmossdk.io/x/gov/migrations/v6"
	"cosmossdk.io/x/gov/types"
	v1 "cosmossdk.io/x/gov/types/v1"

	"github.com/cosmos/cosmos-sdk/codec"
	codectestutil "github.com/cosmos/cosmos-sdk/codec/testutil"
	"github.com/cosmos/cosmos-sdk/testutil"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
)

func TestMigrateStore(t *testing.T) {
	cdc := moduletestutil.MakeTestEncodingConfig(codectestutil.CodecOptions{}, gov.AppModule{}).Codec
	ctx := testutil.DefaultContext("gov")
	storeService := coretesting.KVStoreService(ctx, "gov")
	sb := collections.NewSchemaBuilder(storeService)
	paramsCollection := collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[v1.Params](cdc))
	proposalCollection := collections.NewMap(sb, types.ProposalsKeyPrefix, "proposals", collections.Uint64Key, codec.CollValue[v1.Proposal](cdc))

	// set defaults without newly added fields
	previousParams := v1.DefaultParams()
	previousParams.YesQuorum = ""
	previousParams.ExpeditedQuorum = ""
	previousParams.ProposalCancelMaxPeriod = ""
	previousParams.OptimisticAuthorizedAddresses = nil
	previousParams.OptimisticRejectedThreshold = ""
	err := paramsCollection.Set(ctx, previousParams)
	require.NoError(t, err)

	// Run migrations.
	err = v6.MigrateStore(ctx, storeService, paramsCollection, proposalCollection)
	require.NoError(t, err)

	// Check params
	newParams, err := paramsCollection.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, v1.DefaultParams().YesQuorum, newParams.YesQuorum)
	require.Equal(t, v1.DefaultParams().Quorum, newParams.ExpeditedQuorum) // ExpeditedQuorum is set to Quorum during the migration instead of the default SDK value, for avoiding behavior change.
	require.Equal(t, v1.DefaultParams().ProposalCancelMaxPeriod, newParams.ProposalCancelMaxPeriod)
	require.Equal(t, v1.DefaultParams().OptimisticAuthorizedAddresses, newParams.OptimisticAuthorizedAddresses)
	require.Equal(t, v1.DefaultParams().OptimisticRejectedThreshold, newParams.OptimisticRejectedThreshold)
}
