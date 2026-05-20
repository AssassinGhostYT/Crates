package crates

import (
	"encoding/json"
	"fmt"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
	"os"
	"strings"
	"sync"
)

// Rarity represents the rarity of a crate.
type Rarity struct {
	Name  string
	Color string
}

var (
	RarityCommon = Rarity{Name: "Common", Color: "§7"}
	RarityRare   = Rarity{Name: "Rare", Color: "§b"}
	RarityEpic   = Rarity{Name: "Epic", Color: "§d"}
	RarityMythic = Rarity{Name: "Mythic", Color: "§5"}
)

const Version = "1.0.0"

// Reward represents a possible item reward from a crate.
type Reward struct {
	Item   item.Stack
	Chance float64
	IsRare bool
}

// rewardJSON is a helper to serialize Rewards.
type rewardJSON struct {
	ItemName   string  `json:"name"`
	ItemCount  int     `json:"count"`
	CustomName string  `json:"custom_name,omitempty"`
	Chance     float64 `json:"chance"`
	IsRare     bool    `json:"is_rare"`
}

// crateJSON is a helper to serialize Crates.
type crateJSON struct {
	Rarity       Rarity       `json:"rarity"`
	Rewards      []rewardJSON `json:"rewards"`
	HologramUUID uuid.UUID    `json:"hologram_uuid"`
	Pos          cube.Pos     `json:"pos"`
}

// Crate represents a physical crate in the world.
type Crate struct {
	Rarity       Rarity
	Rewards      []Reward
	HologramUUID uuid.UUID
	Hologram     *world.EntityHandle
	Pos          cube.Pos
}

var (
	cratesMu     sync.RWMutex
	loadedCrates = make(map[cube.Pos]*Crate)
)

// SaveCrates saves all loaded crates to a JSON file.
func SaveCrates() {
	cratesMu.RLock()
	defer cratesMu.RUnlock()

	_ = os.MkdirAll("plugin_data/Crates", 0755)

	serializable := make(map[string]crateJSON)
	for pos, crate := range loadedCrates {
		var rewards []rewardJSON
		for _, r := range crate.Rewards {
			name, _ := r.Item.Item().(interface{ EncodeItem() (string, int16) }).EncodeItem()
			rewards = append(rewards, rewardJSON{
				ItemName:   name,
				ItemCount:  r.Item.Count(),
				CustomName: r.Item.CustomName(),
				Chance:     r.Chance,
				IsRare:     r.IsRare,
			})
		}

		uuidVal := uuid.Nil
		if crate.Hologram != nil {
			uuidVal = crate.Hologram.UUID()
		}

		key := fmt.Sprintf("%d,%d,%d", pos.X(), pos.Y(), pos.Z())
		serializable[key] = crateJSON{
			Rarity:       crate.Rarity,
			Rewards:      rewards,
			HologramUUID: uuidVal,
			Pos:          pos,
		}
	}

	data, _ := json.Marshal(serializable)
	_ = os.WriteFile("plugin_data/Crates/crates.json", data, 0644)
}

// LoadCrates loads crates from the JSON file.
func LoadCrates(w *world.World) {
	data, err := os.ReadFile("plugin_data/Crates/crates.json")
	if err != nil {
		return
	}

	serializable := make(map[string]crateJSON)
	if err := json.Unmarshal(data, &serializable); err != nil {
		return
	}

	cratesMu.Lock()
	for k := range loadedCrates { delete(loadedCrates, k) }
	
	for _, cj := range serializable {
		var rewards []Reward
		for _, rj := range cj.Rewards {
			it, ok := world.ItemByName(rj.ItemName, 0)
			if !ok { continue }
			
			stack := item.NewStack(it, rj.ItemCount)
			if rj.CustomName != "" {
				stack = stack.WithCustomName(rj.CustomName)
			}
			rewards = append(rewards, Reward{
				Item:   stack,
				Chance: rj.Chance,
				IsRare: rj.IsRare,
			})
		}

		loadedCrates[cj.Pos] = &Crate{
			Rarity:       cj.Rarity,
			Rewards:      rewards,
			HologramUUID: cj.HologramUUID,
			Pos:          cj.Pos,
		}
	}
	cratesMu.Unlock()

	cratesMu.RLock()
	for pos, crate := range loadedCrates {
		SpawnIdleParticles(w, pos, crate.Rarity)
		crate.Hologram = SpawnHologram(w, pos, crate.Rarity)
	}
	cratesMu.RUnlock()
}

// IsOP checks if a player is an operator.
func IsOP(p *player.Player) bool {
	data, err := os.ReadFile("ops.txt")
	if err != nil {
		return false
	}
	name := strings.ToLower(p.Name())
	for _, line := range strings.Split(string(data), "\n") {
		if strings.ToLower(strings.TrimSpace(line)) == name {
			return true
		}
	}
	return false
}

// NewCrate creates a new crate with the given rarity and rewards.
func NewCrate(rarity Rarity, rewards []Reward) *Crate {
	return &Crate{
		Rarity:  rarity,
		Rewards: rewards,
	}
}
