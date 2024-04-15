package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/x/mint/types"
)

// InitGenesis new mint genesis
func (keeper Keeper) InitGenesis(ctx context.Context, ak types.AccountKeeper, data *types.GenesisState) error {
	if data == nil {
		return fmt.Errorf("nil mint genesis state")
	}

	data.Minter.EpochProvisions = data.Params.GenesisEpochProvisions

	if err := keeper.Minter.Set(ctx, data.Minter); err != nil {
		return err
	}

	if err := keeper.Params.Set(ctx, data.Params); err != nil {
		return err
	}

	ak.GetModuleAccount(ctx, types.ModuleName)

	if err := keeper.setLastReductionEpochNum(ctx, data.ReductionStartedEpoch); err != nil {
		return err
	}

	return nil
}

// ExportGenesis returns a GenesisState for a given context and keeper.
func (keeper Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	minter, err := keeper.Minter.Get(ctx)
	if err != nil {
		return nil, err
	}

	params, err := keeper.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	lastHalvenEpoch, err := keeper.getLastReductionEpochNum(ctx)
	if err != nil {
		return nil, err
	}

	return types.NewGenesisState(minter, params, lastHalvenEpoch), nil
}
