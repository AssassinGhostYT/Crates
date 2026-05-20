package crates

import (
	"github.com/df-mc/dragonfly/server/item"
	"fmt"
)

// Key represents a key used to open a crate.
type Key struct {
	Rarity Rarity
}

// NewKey creates a new key for the given rarity.
func NewKey(rarity Rarity) item.Stack {
	// We use a GoldNugget as the base item for the key.
	s := item.NewStack(item.GoldNugget{}, 1)
	s = s.WithCustomName(fmt.Sprintf("%s%s Key", rarity.Color, rarity.Name))
	s = s.WithLore("§7Use this key on a crate\n§7to get a reward!")
	return s
}

// IsKey checks if the item stack is a key for the given rarity.
func IsKey(s item.Stack, rarity Rarity) bool {
	return s.CustomName() == fmt.Sprintf("%s%s Key", rarity.Color, rarity.Name)
}
