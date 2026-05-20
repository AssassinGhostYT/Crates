package crates

import (
	"fmt"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"strings"
)

// Handler is the global crate handler. In a real plugin, this would be managed better.
var GlobalHandler *CrateHandler

// CrateCommand is the main command for managing crates.
type CrateCommand struct {
	Sub crateSubCommand
}

func (c CrateCommand) Run(src cmd.Source, o *cmd.Output, tx *world.Tx) {
	// This will not be called because we use subcommands.
}

func (c CrateCommand) Allow(src cmd.Source) bool {
	p, ok := src.(*player.Player)
	return ok && IsOP(p)
}

type crateSubCommand string

func (crateSubCommand) SubCommands() []string {
	return []string{"set", "give", "remove", "save", "help"}
}

// HelpCommand handles /crate help and provides instructions.
type HelpCommand struct {
	Sub cmd.SubCommand `cmd:"help"`
}

func (c HelpCommand) Run(src cmd.Source, o *cmd.Output, tx *world.Tx) {
	o.Printf("§e--- CRATES SYSTEM HELP ---")
	o.Printf("§b/crate set <rarity> §7- Create a crate at looking position.")
	o.Printf("§b/crate save §7- Save items inside the chest as rewards.")
	o.Printf("§b/crate remove §7- Toggle removal mode (Break crate to delete).")
	o.Printf("§b/crate give <player> <rarity> [amount] §7- Give keys.")
	o.Printf("§b/crate help §7- Show this message.")
	o.Printf("§e--------------------------")
}

func (c HelpCommand) Allow(src cmd.Source) bool {
	p, ok := src.(*player.Player)
	return ok && IsOP(p)
}

// SaveCommand handles /crate save
type SaveCommand struct {
	Sub cmd.SubCommand `cmd:"save"`
}

func (c SaveCommand) Run(src cmd.Source, o *cmd.Output, tx *world.Tx) {
	p, _ := src.(*player.Player)

	pos, found := findLookingCrate(p)
	if !found {
		o.Errorf("You must be looking directly at a registered crate.")
		return
	}

	b := tx.Block(pos)
	chest, ok := b.(interface {
		Inventory(tx *world.Tx, pos cube.Pos) *inventory.Inventory
	})
	if !ok {
		o.Errorf("This crate type does not support direct inventory editing (Mythic crates use Ender Chests).")
		return
	}

	inv := chest.Inventory(tx, pos)
	items := inv.Items()
	
	var newRewards []Reward
	for _, it := range items {
		if !it.Empty() {
			newRewards = append(newRewards, Reward{
				Item:   it,
				Chance: 1.0,
			})
		}
	}

	if len(newRewards) == 0 {
		o.Errorf("The chest is empty! Add some rewards first.")
		return
	}

	equalChance := 100.0 / float64(len(newRewards))
	for i := range newRewards {
		newRewards[i].Chance = equalChance
	}

	cratesMu.Lock()
	GlobalHandler.Crates[pos].Rewards = newRewards
	cratesMu.Unlock()
	
	SaveCrates()
	o.Printf("§aSuccess! Saved %d items as rewards for this crate.", len(newRewards))
}

func (c SaveCommand) Allow(src cmd.Source) bool {
	p, ok := src.(*player.Player)
	return ok && IsOP(p)
}

// RemoveCommand handles /crate remove (Toggle mode)
type RemoveCommand struct {
	Sub cmd.SubCommand `cmd:"remove"`
}

func (c RemoveCommand) Run(src cmd.Source, o *cmd.Output, tx *world.Tx) {
	p, _ := src.(*player.Player)

	enabled := ToggleRemovalMode(p)
	if enabled {
		o.Printf("§eRemoval mode §aENABLED§e. Break a crate to remove it.")
	} else {
		o.Printf("§eRemoval mode §cDISABLED§e.")
	}
}

func (c RemoveCommand) Allow(src cmd.Source) bool {
	p, ok := src.(*player.Player)
	return ok && IsOP(p)
}

// SetCommand handles /crate set <rarity>
type SetCommand struct {
	Sub    cmd.SubCommand `cmd:"set"`
	Rarity string         `cmd:"rarity"`
}

func (c SetCommand) Run(src cmd.Source, o *cmd.Output, tx *world.Tx) {
	p, _ := src.(*player.Player)

	rarity, err := parseRarity(c.Rarity)
	if err != nil {
		o.Errorf("Invalid rarity. Use: common, rare, epic, mythic")
		return
	}

	var pos cube.Pos
	found := false
	start := p.Position().Add(mgl64.Vec3{0, p.H().Type().BBox(p).Max()[1] * 0.9, 0})
	direction := p.Rotation().Vec3()
	for i := 0.0; i < 5.0; i += 0.05 {
		testPos := cube.PosFromVec3(start.Add(direction.Mul(i)))
		if len(tx.Block(testPos).Model().BBox(testPos, tx)) > 0 {
			pos = testPos
			found = true
			break
		}
	}

	if !found {
		o.Errorf("You must be looking at a block.")
		return
	}

	cratesMu.Lock()
	GlobalHandler.Crates[pos] = &Crate{
		Rarity:   rarity,
		Rewards:  SampleRewards(),
		Hologram: SpawnHologram(p.Tx().World(), pos, rarity),
		Pos:      pos,
	}
	cratesMu.Unlock()
	
	SpawnIdleParticles(p.Tx().World(), pos, rarity)
	SaveCrates()
	
	o.Printf("§aSuccess! Block at %v set as a %s%s Crate§a.", pos, rarity.Color, rarity.Name)
}

func (c SetCommand) Allow(src cmd.Source) bool {
	p, ok := src.(*player.Player)
	return ok && IsOP(p)
}

func findLookingCrate(p *player.Player) (cube.Pos, bool) {
	start := p.Position().Add(mgl64.Vec3{0, p.H().Type().BBox(p).Max()[1] * 0.9, 0})
	direction := p.Rotation().Vec3()
	for i := 0.0; i < 5.0; i += 0.05 {
		pos := cube.PosFromVec3(start.Add(direction.Mul(i)))
		cratesMu.RLock()
		_, ok := GlobalHandler.Crates[pos]
		cratesMu.RUnlock()
		if ok {
			return pos, true
		}
	}
	return cube.Pos{}, false
}

// GiveCommand handles /crate give <player> <rarity> [amount]
type GiveCommand struct {
	Sub    cmd.SubCommand `cmd:"give"`
	Target []cmd.Target   `cmd:"player"`
	Rarity string         `cmd:"rarity"`
	Amount int            `cmd:"amount"`
}

func (c GiveCommand) Run(src cmd.Source, o *cmd.Output, tx *world.Tx) {
	rarity, err := parseRarity(c.Rarity)
	if err != nil {
		o.Errorf("Invalid rarity. Use: common, rare, epic, mythic")
		return
	}

	if c.Amount <= 0 {
		c.Amount = 1
	}

	key := NewKey(rarity)
	key = item.NewStack(key.Item(), c.Amount).WithCustomName(key.CustomName()).WithLore(key.Lore()...)

	for _, target := range c.Target {
		if p, ok := target.(*player.Player); ok {
			p.Inventory().AddItem(key)
			o.Printf("§aGave %d %s%s Keys §ato %s.", c.Amount, rarity.Color, rarity.Name, p.Name())
		}
	}
}

func (c GiveCommand) Allow(src cmd.Source) bool {
	p, ok := src.(*player.Player)
	return ok && IsOP(p)
}

func parseRarity(s string) (Rarity, error) {
	switch strings.ToLower(s) {
	case "common": return RarityCommon, nil
	case "rare":   return RarityRare, nil
	case "epic":   return RarityEpic, nil
	case "mythic": return RarityMythic, nil
	}
	return Rarity{}, fmt.Errorf("unknown rarity")
}
