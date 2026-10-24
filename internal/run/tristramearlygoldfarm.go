package run

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/d2go/pkg/data/quest"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/utils"
)

// TristramEarlyGoldfarm is a struct that represents a new run for early gold farming in Tristram.
// It holds a reference to the current game context.
type TristramEarlyGoldfarm struct {
	ctx *context.Status
}

// NewTristramEarlyGoldfarm is the constructor function for our new run.
func NewTristramEarlyGoldfarm() *TristramEarlyGoldfarm {
	return &TristramEarlyGoldfarm{
		ctx: context.Get(),
	}
}

// Name returns the name of this run, which is used for configuration.
func (t *TristramEarlyGoldfarm) Name() string {
	return string(config.TristramEarlyGoldfarmRun)
}

func (t *TristramEarlyGoldfarm) CheckConditions(parameters *RunParameters) SequencerResult {
	if IsQuestRun(parameters) {
		return SequencerError
	}
	if !t.ctx.Data.Quests[quest.Act1DenOfEvil].Completed() {
		return SequencerSkip
	}
	return SequencerOk
}

// Run contains the logic for the early gold farming run.
// It travels to Stony Field, finds a Cairn Stone, moves to it, and clears the surrounding monsters.
func (t *TristramEarlyGoldfarm) Run(parameters *RunParameters) error {
	// Use waypoint to StonyField
	slog.Info("tristram early goldfarm: traveling to Stony Field")
	err := action.WayPoint(area.StonyField)
	if err != nil {
		return fmt.Errorf("could not travel to Stony Field: %w", err)
	}

	// Find the Cairn Stone Alpha
	var cairnStone data.Object
	found := false
	for _, o := range t.ctx.Data.Objects {
		if o.Name == object.CairnStoneAlpha {
			cairnStone = o
			found = true
			break
		}
	}

	if !found {
		return errors.New("Cairn Stone not found in Stony Field")
	}

	slog.Info("tristram early goldfarm: found Cairn Stone, moving to position")
	// Move to the cairnStone
	action.MoveToCoords(cairnStone.Position)

	// Clear area around the portal
	slog.Info("tristram early goldfarm: clearing area around the stone")
	action.ClearAreaAroundPlayer(40, data.MonsterAnyFilter())

	t.ctx.RefreshGameData()

	utils.Sleep(500)

	action.ItemPickup(-1)
	utils.Sleep(500)

	slog.Info("tristram early goldfarm: run complete")

	return nil
}
