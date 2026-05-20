package crates

import (
	"fmt"
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/particle"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
	"image/color"
	"math"
	"strings"
	"time"
)

// SpawnHologram spawns a floating text above the crate and returns its handle.
// It also cleans up any existing holograms at that position to avoid duplication.
func SpawnHologram(w *world.World, pos cube.Pos, rarity Rarity) *world.EntityHandle {
	text := fmt.Sprintf("%s%s CRATE\n§fRight-click with key to open!", rarity.Color, strings.ToUpper(rarity.Name))
	hologramPos := pos.Vec3Centre().Add(mgl64.Vec3{0, 1.2, 0})
	h := entity.NewText(text, hologramPos)

	w.Exec(func(tx *world.Tx) {
		// --- ANTI-DUPLICATION LOGIC ---
		// We look in a small box around the intended hologram position.
		searchBox := cube.Box(-0.5, -0.5, -0.5, 0.5, 0.5, 0.5).Translate(hologramPos)
		for e := range tx.EntitiesWithin(searchBox) {
			// Check if it's a text entity by type encoding
			if e.H().Type().EncodeEntity() == "dragonfly:text" {
				tx.RemoveEntity(e)
			}
		}
		tx.AddEntity(h)
	})
	return h
}

// SpawnIdleParticles spawns rotating particles around a crate position.
func SpawnIdleParticles(w *world.World, pos cube.Pos, rarity Rarity) {
	go func() {
		angle := 0.0
		center := pos.Vec3Centre()
		
		for {
			// Check if crate still exists in map (to stop particles if removed)
			cratesMu.RLock()
			_, exists := loadedCrates[pos]
			cratesMu.RUnlock()
			if !exists {
				return
			}

			angle += 0.2
			if angle > 2*math.Pi {
				angle = 0
			}
			
			if rarity == RarityMythic {
				w.Exec(func(tx *world.Tx) {
					purple := color.RGBA{R: 160, G: 32, B: 240, A: 255}
					s, c := math.Sincos(angle)
					for i := -0.8; i <= 0.8; i += 0.2 {
						tx.AddParticle(center.Add(mgl64.Vec3{i * c, i, i * s}), particle.Dust{Colour: purple})
						tx.AddParticle(center.Add(mgl64.Vec3{-i * c, i, -i * s}), particle.Dust{Colour: purple})
					}
				})
			} else {
				x1 := math.Cos(angle) * 0.8
				z1 := math.Sin(angle) * 0.8
				x2 := math.Cos(angle + math.Pi) * 0.8
				z2 := math.Sin(angle + math.Pi) * 0.8
				
				p1 := center.Add(mgl64.Vec3{x1, -0.2 + math.Sin(angle)*0.2, z1})
				p2 := center.Add(mgl64.Vec3{x2, -0.2 + math.Sin(angle + math.Pi)*0.2, z2})
				
				w.Exec(func(tx *world.Tx) {
					tx.AddParticle(p1, particle.Flame{})
					tx.AddParticle(p2, particle.Flame{})
				})
			}
			
			time.Sleep(time.Millisecond * 100)
		}
	}()
}

// PlayOpeningAnimation plays the cinematic "Trial Chamber" opening sequence.
func PlayOpeningAnimation(w *world.World, pos cube.Pos, rarity Rarity, rewards []Reward, winner Reward) {
	center := pos.Vec3Centre()
	
	w.Exec(func(tx *world.Tx) {
		for _, v := range tx.Viewers(center) {
			v.ViewBlockAction(pos, block.OpenAction{})
		}
		tx.PlaySound(center, sound.ChestOpen{})
	})
	
	go func() {
		var currentItem *world.EntityHandle
		
		for i := 0; i < 20; i++ {
			showReward := rewards[i%len(rewards)]
			
			w.Exec(func(tx *world.Tx) {
				if currentItem != nil {
					if ent, ok := currentItem.Entity(tx); ok {
						tx.RemoveEntity(ent)
					}
				}
				
				opts := world.EntitySpawnOpts{Position: center.Add(mgl64.Vec3{0, 0.5, 0})}
				currentItem = entity.NewItemPickupDelay(opts, showReward.Item, time.Hour)
				tx.AddEntity(currentItem)
				
				tx.AddParticle(center.Add(mgl64.Vec3{0, 1, 0}), particle.Dust{Colour: colorFromRarity(rarity)})
			})
			
			time.Sleep(time.Millisecond * 150)
		}
		
		w.Exec(func(tx *world.Tx) {
			if currentItem != nil {
				if ent, ok := currentItem.Entity(tx); ok {
					tx.RemoveEntity(ent)
				}
			}
			
			opts := world.EntitySpawnOpts{Position: center.Add(mgl64.Vec3{0, 0.8, 0})}
			winnerItemHandle := entity.NewItemPickupDelay(opts, winner.Item, time.Hour)
			winnerEntity := tx.AddEntity(winnerItemHandle)
			
			tx.AddParticle(center.Add(mgl64.Vec3{0, 1, 0}), particle.HugeExplosion{})
			for i := 0; i < 20; i++ {
				tx.AddParticle(center.Add(mgl64.Vec3{0, 1, 0}), particle.Lava{})
			}
			
			go func() {
				time.Sleep(time.Second * 3)
				w.Exec(func(tx *world.Tx) {
					tx.RemoveEntity(winnerEntity)
					for _, v := range tx.Viewers(center) {
						v.ViewBlockAction(pos, block.CloseAction{})
					}
					tx.PlaySound(center, sound.ChestClose{})
				})
			}()
		})
	}()
}

func colorFromRarity(r Rarity) color.RGBA {
	switch r {
	case RarityCommon: return color.RGBA{R: 178, G: 178, B: 178, A: 255}
	case RarityRare:   return color.RGBA{R: 0, G: 204, B: 255, A: 255}
	case RarityEpic:   return color.RGBA{R: 255, G: 0, B: 255, A: 255}
	case RarityMythic: return color.RGBA{R: 128, G: 0, B: 128, A: 255}
	default:           return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
}
