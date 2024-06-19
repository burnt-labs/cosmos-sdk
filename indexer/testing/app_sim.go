package indexertesting

import (
	"fmt"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/btree"
	"pgregory.net/rapid"

	indexerbase "cosmossdk.io/indexer/base"
	"cosmossdk.io/indexer/testing/schemagen"
)

type AppSimulatorOptions struct {
	AppSchema          map[string]indexerbase.ModuleSchema
	Listener           indexerbase.Listener
	EventAlignedWrites bool
	MaxUpdatesPerBlock int
	Seed               int
}

type AppSimulator struct {
	options  AppSimulatorOptions
	modules  *btree.Map[string, *moduleState]
	blockNum uint64
}

func NewAppSimulator(options AppSimulatorOptions) *AppSimulator {
	modules := &btree.Map[string, *moduleState]{}
	for module, schema := range options.AppSchema {
		modState := &moduleState{
			ModuleSchema: schema,
			Objects:      &btree.Map[string, *objectState]{},
		}
		modules.Set(module, modState)
		for _, objectType := range schema.ObjectTypes {
			state := &btree.Map[string, *Entry]{}
			objState := &objectState{
				ObjectType: objectType,
				Objects:    state,
				UpdateGen:  schemagen.StatefulObjectUpdate(objectType, state),
			}
			modState.Objects.Set(objectType.Name, objState)
		}
	}

	return &AppSimulator{
		options: options,
		modules: modules,
	}
}

func (a *AppSimulator) Initialize() error {
	if f := a.options.Listener.InitializeModuleSchema; f != nil {
		var err error
		a.modules.Scan(func(moduleName string, mod *moduleState) bool {
			err = f(moduleName, mod.ModuleSchema)
			return err == nil
		})
		return err
	}
	return nil
}

func (a *AppSimulator) NextBlock() error {
	a.blockNum++

	if f := a.options.Listener.StartBlock; f != nil {
		err := f(a.blockNum)
		if err != nil {
			return err
		}
	}

	a.newBlockFromSeed(a.options.Seed + int(a.blockNum))

	if f := a.options.Listener.Commit; f != nil {
		err := f()
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *AppSimulator) actionNewBlock(t *rapid.T) {
	maxUpdates := a.options.MaxUpdatesPerBlock
	if maxUpdates <= 0 {
		maxUpdates = 100
	}
	numUpdates := rapid.IntRange(1, maxUpdates).Draw(t, "numUpdates")
	for i := 0; i < numUpdates; i++ {
		moduleIdx := rapid.IntRange(0, a.modules.Len()).Draw(t, "moduleIdx")
		keys, values := a.modules.KeyValues()
		modState := values[moduleIdx]
		objectIdx := rapid.IntRange(0, modState.Objects.Len()).Draw(t, "objectIdx")
		objState := modState.Objects.Values()[objectIdx]
		update := objState.UpdateGen.Draw(t, "update")
		require.NoError(t, objState.ObjectType.ValidateObjectUpdate(update))
		require.NoError(t, a.applyUpdate(keys[moduleIdx], update))
	}
}

func (a *AppSimulator) newBlockFromSeed(seed int) {
	rapid.Custom[any](func(t *rapid.T) any {
		a.actionNewBlock(t)
		return nil
	}).Example(seed)
}

func (a *AppSimulator) applyUpdate(module string, update indexerbase.ObjectUpdate) error {
	modState, ok := a.modules.Get(module)
	if !ok {
		return fmt.Errorf("module %v not found", module)
	}

	objState, ok := modState.Objects.Get(update.TypeName)
	if !ok {
		return fmt.Errorf("object type %v not found in module %v", update.TypeName, module)
	}

	keyStr := fmt.Sprintf("%v", update.Key)
	if update.Delete {
		objState.Objects.Delete(keyStr)
	} else {
		objState.Objects.Set(fmt.Sprintf("%v", update.Key), &Entry{Key: update.Key, Value: update.Value})
	}

	if a.options.Listener.OnObjectUpdate != nil {
		err := a.options.Listener.OnObjectUpdate(module, update)
		return err
	}
	return nil
}

type Entry struct {
	Key   any
	Value any
}

type moduleState struct {
	ModuleSchema indexerbase.ModuleSchema
	Objects      *btree.Map[string, *objectState]
}

type objectState struct {
	ObjectType indexerbase.ObjectType
	Objects    *btree.Map[string, *Entry]
	UpdateGen  *rapid.Generator[indexerbase.ObjectUpdate]
}