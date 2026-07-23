package winui

import (
	"github.com/effexorxruser/EffexorWinPE/internal/shell/i18n"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/journal"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/orchestrator"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/viewmodel"
)

// Config launches the EffexorWinPE shell UI.
type Config struct {
	Bundle       *i18n.Bundle
	Model        viewmodel.AppModel
	Orchestrator *orchestrator.Orchestrator
	Journal      *journal.Journal
	Mock         bool
	Kiosk        bool // fullscreen / kiosk-style startup (default on WinPE builds)
}
