package block

import "fmt"

// Registry holds all known block definitions. The operator populates it
// at startup. The API server reads it to serve the block catalog.
type Registry struct {
	blocks map[string]Block
}

// NewRegistry creates an empty block registry.
func NewRegistry() *Registry {
	return &Registry{blocks: make(map[string]Block)}
}

// Register adds a block to the registry. Returns an error if a block
// with the same kind is already registered.
func (r *Registry) Register(b Block) error {
	kind := b.Descriptor().Kind
	if _, exists := r.blocks[kind]; exists {
		return fmt.Errorf("block %q already registered", kind)
	}
	r.blocks[kind] = b
	return nil
}

// Get retrieves a block by its kind.
func (r *Registry) Get(kind string) (Block, bool) {
	b, ok := r.blocks[kind]
	return b, ok
}

// List returns descriptors for all registered blocks.
func (r *Registry) List() []Descriptor {
	descriptors := make([]Descriptor, 0, len(r.blocks))
	for _, b := range r.blocks {
		descriptors = append(descriptors, b.Descriptor())
	}
	return descriptors
}

// ListByCategory returns descriptors filtered by category.
func (r *Registry) ListByCategory(category Category) []Descriptor {
	var result []Descriptor
	for _, b := range r.blocks {
		if b.Descriptor().Category == category {
			result = append(result, b.Descriptor())
		}
	}
	return result
}
