package ui

import (
	"github.com/hectorgimenez/koolo/internal/context"
)

// GameCoordsToScreenCords converts game world coordinates to virtual screen
// coordinates (1280x720). Click/MovePointer then scale to actual window size.
func GameCoordsToScreenCords(destinationX, destinationY int) (int, int) {
	ctx := context.Get()
	// Calculate diff between current player position and destination
	diffX := destinationX - ctx.Data.PlayerUnit.Position.X
	diffY := destinationY - ctx.Data.PlayerUnit.Position.Y

	// Transform cartesian movement (World) to isometric (screen)
	// Center of virtual 1280x720 screen; Click() scales to actual window.
	screenX := int((float32(diffX-diffY) * 19.8) + 640.0)
	screenY := int((float32(diffX+diffY) * 9.9) + 360.0)

	return screenX, screenY
}
