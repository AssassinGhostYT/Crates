package crates

import (
	"fmt"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/inventory"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/cmd"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Register initializes the crate system and registers the /crate command.
func Register(w *world.World) *CrateHandler {
	cratesMu.Lock()
	for k := range loadedCrates { delete(loadedCrates, k) }
	cratesMu.Unlock()

	GlobalHandler = &CrateHandler{
		Crates: loadedCrates,
	}

	cmd.Register(cmd.New("crate", "Manage crates system", []string{"c"}, HelpCommand{}, SetCommand{}, GiveCommand{}, RemoveCommand{}, SaveCommand{}))

	LoadCrates(w)
	fmt.Printf("\033[32m[Crates] Plugin Crates v%s enabled!\033[0m\n", Version)

	return GlobalHandler
}

// Close deactivates the crate system.
func Close() {
	SaveCrates()
	fmt.Printf("\033[31m[Crates] Plugin Crates v%s disabled!\033[0m\n", Version)
}

// SampleRewards returns a list of sample rewards for testing.
func SampleRewards() []Reward {
	return []Reward{
		{Item: item.NewStack(item.Diamond{}, 5), Chance: 10, IsRare: true},
		{Item: item.NewStack(item.IronIngot{}, 16), Chance: 50},
		{Item: item.NewStack(item.GoldIngot{}, 8), Chance: 30},
		{Item: item.NewStack(item.Emerald{}, 2), Chance: 10},
	}
}

var (
	removalModeMu sync.RWMutex
	removalMode   = make(map[string]bool) // Key: Player Name
)

func ToggleRemovalMode(p *player.Player) bool {
	removalModeMu.Lock()
	defer removalModeMu.Unlock()
	name := p.Name()
	removalMode[name] = !removalMode[name]
	return removalMode[name]
}

func SetRemovalMode(p *player.Player, enabled bool) {
	removalModeMu.Lock()
	defer removalModeMu.Unlock()
	removalMode[p.Name()] = enabled
}

// CrateHandler handles interactions with crates.
type CrateHandler struct {
	player.NopHandler
	Crates map[cube.Pos]*Crate
}

// removeCrateInternal handles the actual removal of data and physical cleanup.
func (h *CrateHandler) removeCrateInternal(tx *world.Tx, p *player.Player, pos cube.Pos) (string, bool) {
	cratesMu.RLock()
	crate, ok := h.Crates[pos]
	cratesMu.RUnlock()

	if !ok {
		return "", false
	}

	rarityName := crate.Rarity.Name

	// 1. Physically clear items inside
	b := tx.Block(pos)
	if chest, ok := b.(interface {
		Inventory(tx *world.Tx, pos cube.Pos) *inventory.Inventory
	}); ok {
		chest.Inventory(tx, pos).Clear()
	}

	// 2. Remove hologram entity correctly
	if crate.Hologram != nil {
		if ent, ok := crate.Hologram.Entity(tx); ok {
			tx.RemoveEntity(ent)
		}
	}
	
	// 3. Delete from map
	cratesMu.Lock()
	delete(h.Crates, pos)
	cratesMu.Unlock()
	
	SaveCrates()
	
	// Auto-disable removal mode
	SetRemovalMode(p, false)
	
	return rarityName, true
}

// HandleStartBreak ...
func (h *CrateHandler) HandleStartBreak(ctx *player.Context, pos cube.Pos) {
}

// HandleBlockBreak handles when a block is fully broken.
func (h *CrateHandler) HandleBlockBreak(ctx *player.Context, pos cube.Pos, drops *[]item.Stack, xp *int) {
	p := ctx.Val()
	
	cratesMu.RLock()
	_, isCrate := h.Crates[pos]
	cratesMu.RUnlock()

	if isCrate {
		removalModeMu.RLock()
		enabled := removalMode[p.Name()]
		removalModeMu.RUnlock()
		
		if enabled {
			// PERFORM REMOVAL inside current transaction
			if name, ok := h.removeCrateInternal(p.Tx(), p, pos); ok {
				p.Message(fmt.Sprintf("§a%s Crate Removed.", name))
				*drops = nil // No items drop
			}
		} else {
			// BLOCK BREAKING if not in removal mode
			ctx.Cancel()
			p.Message("§cYou must use §7/crate remove §cto delete this crate.")
		}
	}
}

// HandleItemUseOnBlock handles interaction.
func (h *CrateHandler) HandleItemUseOnBlock(ctx *player.Context, pos cube.Pos, face cube.Face, clickPos mgl64.Vec3) {
	p := ctx.Val()

	removalModeMu.RLock()
	removing := removalMode[p.Name()]
	removalModeMu.RUnlock()
	
	if removing {
		cratesMu.RLock()
		_, isCrate := h.Crates[pos]
		cratesMu.RUnlock()
		if isCrate {
			// STOP opening the chest during removal mode
			ctx.Cancel() 
			p.Message("§eHold click to break and remove this crate.")
			return
		}
	}

	tx := p.Tx()
	b := tx.Block(pos)
	
	cratesMu.RLock()
	crate, ok := h.Crates[pos]
	cratesMu.RUnlock()
	if !ok {
		return
	}
	
	_, isChest := b.(block.Chest)
	_, isEnderChest := b.(block.EnderChest)
	if !isChest && !isEnderChest {
		return
	}
	
	held, _ := p.HeldItems()
	
	// Admin Editing System (Empty hand + OP)
	if held.Empty() {
		if IsOP(p) {
			p.Message("§e--- CRATE EDITOR ---")
			p.Message(fmt.Sprintf("§fRarity: %s%s", crate.Rarity.Color, crate.Rarity.Name))
			p.Message("§7Put items inside and use §b/crate save §7to update rewards.")
		} else {
			// Normal player clicking with empty hand
			p.Message("§cYou need a " + crate.Rarity.Color + crate.Rarity.Name + " Key §cto open this crate!")
		}
		return
	}

	ctx.Cancel()

	if !IsKey(held, crate.Rarity) {
		p.Message("§cYou need a " + crate.Rarity.Color + crate.Rarity.Name + " Key §cto open this crate!")
		return
	}
	
	p.SetHeldItems(held.Grow(-1), item.Stack{})
	winner := pickRandomReward(crate.Rewards)
	w := tx.World()
	PlayOpeningAnimation(w, pos, crate.Rarity, crate.Rewards, winner)
	
	go func() {
		time.Sleep(time.Millisecond * 2500)
		w.Exec(func(tx *world.Tx) {
			actualPlayer, ok := p.H().Entity(tx)
			if !ok { return }
			p2 := actualPlayer.(*player.Player)

			itemName := "Item"
			if ei, ok := winner.Item.Item().(interface{ EncodeItem() (string, int16) }); ok {
				rawName, _ := ei.EncodeItem()
				itemName = strings.Title(strings.ReplaceAll(strings.TrimPrefix(rawName, "minecraft:"), "_", " "))
			}
			if winner.Item.CustomName() != "" {
				itemName = winner.Item.CustomName()
			}

			p2.Message(fmt.Sprintf("§6[Crate] §aYou received §e%s x%d§a!", itemName, winner.Item.Count()))
			p2.Inventory().AddItem(winner.Item)
			tx.PlaySound(p2.Position(), sound.LevelUp{})
		})
	}()
}

func pickRandomReward(rewards []Reward) Reward {
	if len(rewards) == 0 {
		return Reward{Item: item.NewStack(item.Diamond{}, 1)}
	}
	totalChance := 0.0
	for _, r := range rewards {
		totalChance += r.Chance
	}
	r := rand.Float64() * totalChance
	current := 0.0
	for _, reward := range rewards {
		current += reward.Chance
		if r <= current {
			return reward
		}
	}
	return rewards[0]
}
