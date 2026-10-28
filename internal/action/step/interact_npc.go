package step

import (
	"fmt"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/pather"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/hectorgimenez/koolo/internal/utils"
)

func InteractNPC(npcID npc.ID) error {
	ctx := context.Get()
	ctx.SetLastStep("InteractNPC")

	const (
		maxAttempts     = 8
		minMenuOpenWait = 300 * time.Millisecond
		maxDistance     = 15
		hoverWait       = 800 * time.Millisecond
	)

	var targetNPCID data.UnitID

	for attempts := 0; attempts < maxAttempts; attempts++ {
		// Pause the execution if the priority is not the same as the execution priority
		ctx.PauseIfNotPriority()

		// Check if interaction succeeded and menu is open
		if ctx.Data.OpenMenus.NPCInteract || ctx.Data.OpenMenus.NPCShop {
			// Find current NPC position
			if targetNPCID != 0 {
				if currentNPC, found := ctx.Data.Monsters.FindByID(targetNPCID); found {
					currentDistance := pather.DistanceFromPoint(currentNPC.Position, ctx.Data.PlayerUnit.Position)
					if currentDistance <= maxDistance {
						time.Sleep(minMenuOpenWait)
						return nil
					}
				}
			}

			// Wrong NPC, too far, or NPC moved - close menu and retry
			CloseAllMenus()
			time.Sleep(200 * time.Millisecond)
			targetNPCID = 0
			continue
		}

		townNPC, found := ctx.Data.Monsters.FindOne(npcID, data.MonsterTypeNone)
		if !found {
			if attempts == maxAttempts-1 {
				return fmt.Errorf("NPC %d not found after %d attempts", npcID, maxAttempts)
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if attempts == 0 || attempts == 4 {
			ctx.Logger.Debug("InteractNPC unit data",
				"npc", int(npcID),
				"unitID", int(townNPC.UnitID),
				"npcPos", fmt.Sprintf("%d,%d", townNPC.Position.X, townNPC.Position.Y),
				"playerPos", fmt.Sprintf("%d,%d", ctx.Data.PlayerUnit.Position.X, ctx.Data.PlayerUnit.Position.Y),
				"type", townNPC.Type,
				"hovered", townNPC.IsHovered)
		}

		distance := ctx.PathFinder.DistanceFromMe(townNPC.Position)
		if distance > maxDistance {
			return fmt.Errorf("NPC %d is too far away (distance: %d)", npcID, distance)
		}

		// Calculate click position
		x, y := ui.GameCoordsToScreenCords(townNPC.Position.X, townNPC.Position.Y)
		if npcID == npc.Tyrael2 || npcID == npc.Tyrael {
			y = y - 40 // Tyrael has a super weird hitbox
		}

		// Move mouse and click directly. Waiting for the IsHovered unit flag
		// never succeeds under Wine (the flag is not set even with real
		// SendInput moves), but D2R opens the NPC menu when the click
		// position hits the NPC hitbox - same path as movement clicks.
		ctx.HID.MovePointer(x, y)
		utils.Sleep(100)
		targetNPCID = townNPC.UnitID
		ctx.HID.Click(game.LeftButton, x, y)
		time.Sleep(minMenuOpenWait)
		ctx.Logger.Debug("InteractNPC attempt",
			"npc", int(npcID),
			"attempts", attempts,
			"menuOpen", ctx.Data.OpenMenus.NPCInteract || ctx.Data.OpenMenus.NPCShop,
			"clickPos", fmt.Sprintf("%d,%d", x, y))
	}

	return fmt.Errorf("failed to interact with NPC after %d attempts", maxAttempts)
}
