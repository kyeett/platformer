package main

import (
	"github.com/SolarLune/resolv/resolv"
	"github.com/hajimehoshi/ebiten"
	"github.com/kyeett/gomponents/components"
	"github.com/peterhellberg/gfx"
)

func rectToShape(hb gfx.Rect) *resolv.Rectangle {
	return resolv.NewRectangle(int32(hb.Min.X), int32(hb.Min.Y), int32(hb.W()), int32(hb.H()))
}

const factor = 100

func (g *Game) updateMovement(screen *ebiten.Image) {
	var space resolv.Space
	// Add possible collision entities
	for _, e := range g.filteredEntities(components.HitboxType) {
		if e == playerID {
			continue
		}
		pos := g.entities.GetUnsafe(e, components.PosType).(*components.Pos)
		hb := g.entities.GetUnsafe(e, components.HitboxType).(*components.Hitbox)
		if hb.Properties["monsters_only"] {
			continue
		}

		hbMoved := hb.Moved(pos.Vec)

		// Debug things

		scaler := hbMoved.Size().Scaled(factor)
		resizedBox := hbMoved.Resized(gfx.V(0, 0), scaler)

		s := rectToShape(resizedBox)
		// s.SetTags(e)
		// s.SetData(hb)
		tags := []string{e}
		if hb.Properties["allow_from_down"] {
			tags = append(tags, "allow_from_down")
			// fmt.Print(e, " is roof: ")
		}
		s.SetTags(tags...)
		space.AddShape(s)
	}

	for _, e := range []string{playerID} {
		pos := g.entities.GetUnsafe(e, components.PosType).(*components.Pos)
		v := g.entities.GetUnsafe(e, components.VelocityType).(*components.Velocity)
		hb := g.entities.GetUnsafe(e, components.HitboxType).(*components.Hitbox)
		hbMoved := hb.Moved(pos.Vec)
		scaler := hb.Size().Scaled(factor)
		r := rectToShape(hbMoved.Resized(gfx.V(0, 0), scaler))

		// Check collision vertically
		filterFunc := func(s resolv.Shape) bool { return true }
		if v.Y < 0 {
			filterFunc = func(s resolv.Shape) bool {
				return !s.HasTags("allow_from_down")
			}
		}
		verticalSpace := space.Filter(filterFunc)

		if res := verticalSpace.Resolve(r, 0, int32(factor*v.Y)); res.Colliding() && !res.Teleporting {

			t := res.ShapeB.GetTags()[0]

			// Calculate distance to object
			_, bY := res.ShapeB.GetXY()

			targetUnderneath := v.Y > 0

			if targetUnderneath {
				fac := hb.Max.Y
				pos.Y = float64(bY/factor) - fac

				if g.entities.HasComponents(t, components.BouncyType) {
					v.Y = bouncyConst
				} else {
					v.Y = 0
					// Reset number of jumps
					counter := g.entities.GetUnsafe(e, components.CounterType).(*components.Counter)
					(*counter)["jumps"] = 2
				}

				// Killed!
				if g.entities.HasComponents(t, components.KillableType) {
					g.handleKilled(t)
				}

			} else if v.Y < 0 { // Underneath
				g.handleCollided(e, t)
				v.Y = 0
			}

		} else {
			pos.Y += v.Y
		}

		r = rectToShape(hb.Moved(pos.Vec).Resized(gfx.V(0, 0), scaler))
		if res := space.Resolve(r, int32(factor*v.X), 0); res.Colliding() { //&& !res.Teleporting {
			t := res.ShapeB.GetTags()[0]
			g.handleCollided(e, t)
		} else {
			pos.X += v.X
		}
	}

	g.updateMonsterMovement()

}

func (g *Game) updateMonsterMovement() {
	var space resolv.Space
	// Add possible collision entities
	for _, e := range g.filteredEntities(components.HitboxType) {
		hb := g.entities.GetUnsafe(e, components.HitboxType).(*components.Hitbox)
		if hb.Properties["monsters_only"] != true {
			continue
		}
		pos := g.entities.GetUnsafe(e, components.PosType).(*components.Pos)

		hbMoved := hb.Moved(pos.Vec)
		scaler := hbMoved.Size().Scaled(factor)
		resizedBox := hbMoved.Resized(gfx.V(0, 0), scaler)

		s := rectToShape(resizedBox)
		tags := []string{e}
		s.SetTags(tags...)
		space.AddShape(s)
	}

	for _, e := range g.filteredEntities(components.HitboxType, components.VelocityType) {
		if e == playerID {
			continue
		}
		pos := g.entities.GetUnsafe(e, components.PosType).(*components.Pos)
		v := g.entities.GetUnsafe(e, components.VelocityType).(*components.Velocity)
		hb := g.entities.GetUnsafe(e, components.HitboxType).(*components.Hitbox)
		hbMoved := hb.Moved(pos.Vec)
		scaler := hb.Size().Scaled(factor)
		r := rectToShape(hbMoved.Resized(gfx.V(0, 0), scaler))

		if res := space.Resolve(r, 0, int32(factor*v.Y)); res.Colliding() && !res.Teleporting {
			v.Y = -v.Y
		} else {
			pos.Y += v.Y
		}

		if res := space.Resolve(r, int32(factor*v.X), 0); res.Colliding() && !res.Teleporting {
			v.X = -v.X
		} else {
			pos.X += v.X
		}
	}

}

func (g *Game) handleKilled(t string) {
	g.entities.Remove(t, components.HitboxType)
	g.entities.Add(t, components.Rotated{0.0})

	musicPlayer.PlayAudio(bounceSound)
	g.entities.Add(t, components.Scenario{
		F: func() bool {
			pos := g.entities.GetUnsafe(t, components.PosType).(*components.Pos)
			pos.Y += 2

			rot := g.entities.GetUnsafe(t, components.RotatedType).(*components.Rotated)
			rot.Rotate(0.3)

			return pos.Y > float64(g.Height)
		},
	})

}

func (g *Game) handleCollided(e, t string) {

	v := g.entities.GetUnsafe(e, components.VelocityType).(*components.Velocity)
	if g.entities.HasComponents(t, components.HazardType) {
		g.Reset()
		return
	}

	if g.entities.HasComponents(t, components.TeleportingType) {
		t1 := g.entities.GetUnsafe(t, components.TeleportingType).(*components.Teleporting)

		// // Find matching teleport
		for _, o2 := range g.filteredEntities(components.TeleportingType) {
			t2 := g.entities.GetUnsafe(o2, components.TeleportingType).(*components.Teleporting)
			if t1.Target == t2.Name {
				pos := g.entities.GetUnsafe(e, components.PosType).(*components.Pos)

				pos.Vec = t2.Pos
				v.Vec.X = teleportSlow * v.Vec.X
			}
		}
		return
	}

	// Todo, this will not work well with Y-collisions?
	// Bounce if not jumping or falling
	if v.Y == 0 {
		v.X = -0.5 * v.X
	}

}
