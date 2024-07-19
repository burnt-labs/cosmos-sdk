package schema

import "fmt"

// ModuleSchema represents the logical schema of a module for purposes of indexing and querying.
type ModuleSchema struct {
	Types map[string]Type
}

// Validate validates the module schema.
func (s ModuleSchema) Validate() error {
	enumValueMap := map[string]map[string]bool{}
	for name, typ := range s.Types {
		if err := typ.validate(enumValueMap); err != nil {
			return err
		}
	}

	return nil
}

// ValidateObjectUpdate validates that the update conforms to the module schema.
func (s ModuleSchema) ValidateObjectUpdate(update ObjectUpdate) error {
	for _, objType := range s.ObjectTypes {
		if objType.Name == update.TypeName {
			return objType.ValidateObjectUpdate(update)
		}
	}
	return fmt.Errorf("object type %q not found in module schema", update.TypeName)
}
