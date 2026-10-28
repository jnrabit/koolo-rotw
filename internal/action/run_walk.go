package action

import (
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/mode"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/utils"
)

const (
	runWalkSampleTimeout = 350 * time.Millisecond
	runWalkSampleStep    = 40 * time.Millisecond
)

// EnsureRunMode tries to reset the run/walk toggle to running at game start.
// It samples a short forced step to detect walking before toggling to avoid flipping when already running.
func EnsureRunMode() {
	ctx := context.Get()
	ctx.SetLastAction("EnsureRunMode")

	if ctx.Data.PlayerUnit.ID == 0 || !ctx.Manager.InGame() {
		return
	}

	if ctx.Data.OpenMenus.LoadingScreen {
		ctx.WaitForGameToLoad()
	}
	if ctx.Data.OpenMenus.IsMenuOpen() {
		return
	}
	if !ctx.Data.PlayerUnit.Area.IsTown() {
		return
	}

	toggleKB, ok := runWalkBinding(ctx.Data.KeyBindings)
	if !ok {
		return
	}
	if !hasKeyBinding(ctx.Data.KeyBindings.ForceMove) {
		return
	}

	// Virtual 1280x720 center; Click scales to actual window.
	// Left-click moves the character (D2R default); we sample the mode after.
	screenX := 680
	screenY := 350

	ctx.HID.Click(game.LeftButton, screenX, screenY)
	utils.PingSleep(utils.Light, 80)

	deadline := time.Now().Add(runWalkSampleTimeout)
	for time.Now().Before(deadline) {
		ctx.RefreshGameData()
		switch ctx.Data.PlayerUnit.Mode {
		case mode.Running:
			return
		case mode.Walking, mode.WalkingInTown:
			ctx.HID.PressKeyBinding(toggleKB)
			utils.PingSleep(utils.Light, 120)
			ctx.Logger.Debug("Run/walk toggle reset to running")
			return
		}
		time.Sleep(runWalkSampleStep)
	}
}

func runWalkBinding(bindings data.KeyBindings) (data.KeyBinding, bool) {
	if hasKeyBinding(bindings.ToggleRunWalk) {
		return bindings.ToggleRunWalk, true
	}
	if hasKeyBinding(bindings.Run) {
		return bindings.Run, true
	}

	return data.KeyBinding{}, false
}

func hasKeyBinding(kb data.KeyBinding) bool {
	return (kb.Key1[0] != 0 && kb.Key1[0] != 255) || (kb.Key2[0] != 0 && kb.Key2[0] != 255)
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
