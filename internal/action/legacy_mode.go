package action

import (
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/utils"
)

const (
	legacySwitchPollAttempts = 4
	legacySwitchPollDelayMs  = 300
)

func SwitchToLegacyMode() {
	ctx := context.Get()
	ctx.SetLastAction("SwitchToLegacyMode")

	if !ctx.CharacterCfg.ClassicMode {
		return
	}

	enableLegacyMode(ctx, true)
}

func EnableLegacyMode() bool {
	ctx := context.Get()
	ctx.SetLastAction("EnableLegacyMode")

	return enableLegacyMode(ctx, false)
}

func enableLegacyMode(ctx *context.Status, closeMiniPanel bool) bool {
	if ctx == nil || ctx.GameReader == nil {
		return false
	}

	// D2R 2.4+/3.x: classic graphics mode has been removed.
	// LegacyGraphics offset is also unknown for 3.3, so just report success
	// to avoid blocking the bot in an infinite retry loop.
	return true
}

func waitForLegacyGraphicsState(ctx *context.Status, expected bool) bool {
	for i := 0; i < legacySwitchPollAttempts; i++ {
		utils.Sleep(legacySwitchPollDelayMs)
		ctx.RefreshGameData()
		if ctx.Data.LegacyGraphics == expected {
			return true
		}
	}

	return false
}
