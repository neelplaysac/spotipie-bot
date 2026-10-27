package app

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

type blockedDispatcher struct{ release chan struct{} }

func (d blockedDispatcher) Start(*gotgbot.Bot, <-chan json.RawMessage) {}
func (d blockedDispatcher) Stop()                                      { <-d.release }

func TestMonitoringShutdownIsBounded(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	updater := ext.NewUpdater(blockedDispatcher{release: release}, nil)
	if err := stopUpdater(updater, 10*time.Millisecond); err == nil {
		t.Fatal("a stalled dispatcher must time out so monitoring can flush")
	}
}
