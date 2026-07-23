package adapter_test

import (
	"testing"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/adapter"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/mock"
)

func TestViewModelFromMockReport(t *testing.T) {
	report, err := mock.Report()
	if err != nil {
		t.Fatal(err)
	}
	model := adapter.FromReport(report, true)
	if !model.Overview.HasReport {
		t.Fatal("expected report")
	}
	if model.Overview.SchemaVersion != "1.3.0" {
		t.Fatalf("schema = %q", model.Overview.SchemaVersion)
	}
	if model.Overview.Manufacturer != "EffexorWinPE Mock" {
		t.Fatalf("manufacturer = %q", model.Overview.Manufacturer)
	}
	if len(model.Storage.Disks) != 1 {
		t.Fatalf("disks = %d", len(model.Storage.Disks))
	}
	if len(model.Windows.Installs) != 1 {
		t.Fatalf("installs = %d", len(model.Windows.Installs))
	}
}

func TestNullableSMARTFields(t *testing.T) {
	report, err := mock.Report()
	if err != nil {
		t.Fatal(err)
	}
	model := adapter.FromReport(report, false)
	if len(model.Storage.Health) == 0 {
		t.Fatal("expected health rows")
	}
	h := model.Storage.Health[0]
	if h.Temperature.Available {
		t.Fatal("temperature should be unavailable (null)")
	}
	if h.WearPercent.Available {
		t.Fatal("wear should be unavailable (null)")
	}
	if h.ReadErrors.Available {
		t.Fatal("read errors should be unavailable (null)")
	}
	if !h.PowerOnHours.Available || h.PowerOnHours.Value != "1200" {
		t.Fatalf("power on hours = %#v", h.PowerOnHours)
	}
	if !h.WriteErrors.Available || h.WriteErrors.Value != "0" {
		t.Fatalf("write errors = %#v", h.WriteErrors)
	}
	if got := adapter.DisplayOptional(h.Temperature, "н/д"); got != "н/д" {
		t.Fatalf("display = %q", got)
	}
}

func TestBitLockerUnavailableProvider(t *testing.T) {
	report, err := mock.Report()
	if err != nil {
		t.Fatal(err)
	}
	model := adapter.FromReport(report, false)
	if model.BitLocker.InventoryStatus != diagnostics.BitLockerStatusUnavailable {
		t.Fatalf("status = %q", model.BitLocker.InventoryStatus)
	}
	if model.BitLocker.VolumeCountKnown {
		t.Fatal("volume count should be unknown")
	}
	if model.BitLocker.StatusMessageKey != "msg.bitlocker_unavailable" {
		t.Fatalf("message key = %q", model.BitLocker.StatusMessageKey)
	}
	if len(model.BitLocker.Volumes) != 0 {
		t.Fatalf("volumes = %d", len(model.BitLocker.Volumes))
	}
}

func TestEthernetDisconnected(t *testing.T) {
	report, err := mock.Report()
	if err != nil {
		t.Fatal(err)
	}
	model := adapter.FromReport(report, false)
	if model.Network.EthernetConnected {
		t.Fatal("expected ethernet disconnected")
	}
	if model.Network.StatusMessageKey != "msg.ethernet_not_connected" {
		t.Fatalf("key = %q", model.Network.StatusMessageKey)
	}
	if len(model.Network.Adapters) != 2 {
		t.Fatalf("adapters = %d", len(model.Network.Adapters))
	}
}

func TestEmptyWindowsInstalls(t *testing.T) {
	report, err := mock.Report()
	if err != nil {
		t.Fatal(err)
	}
	report.Installations = nil
	model := adapter.FromReport(report, false)
	if model.Windows.EmptyKey != "msg.no_windows_installs" {
		t.Fatalf("empty key = %q", model.Windows.EmptyKey)
	}
}

func TestApplyAssessment(t *testing.T) {
	model, err := mock.AppModel()
	if err != nil {
		t.Fatal(err)
	}
	if !model.Agent.HasAssessment {
		t.Fatal("expected assessment")
	}
	if model.Agent.FindingCount != 2 {
		t.Fatalf("findings = %d", model.Agent.FindingCount)
	}
	if model.Agent.SessionID == "" {
		t.Fatal("expected session id")
	}
}
