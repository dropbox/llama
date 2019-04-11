package llama

import (
	"net"
	"testing"
	"golang.org/x/time/rate"
)

var exampleCallbackChan = make(chan *Probe)

func TestTestRunnerRun(t *testing.T) {
	// TODO(dmar): A more intensive test of this requires more mocking.
	//     For now, it's at least touched in `TestTestRunnerStop`.
}

func TestTestRunnerStop(t *testing.T) {
	tr := NewTestRunner(exampleCallbackChan, rate.NewLimiter(rate.Inf, 0))
	tr.Run()
	tr.Stop()
	if tr.isStopped() != true {
		t.Error("Stopped the TestRunner, but it didn't stop")
	}
}

func TestAddAndDel(t *testing.T) {
	tr := NewTestRunner(exampleCallbackChan, rate.NewLimiter(rate.Inf, 0))
	target, err := net.ResolveUDPAddr("udp", DefaultAddrStr)
	HandleError(err)
	if len(tr.targets) != 0 {
		t.Error("TestRunner already has targets, but shouldn't")
	}
	// Validate adding a target
	tr.Add(target)
	if len(tr.targets) != 1 {
		t.Error("TestRunner should only have one target")
	}
	if tr.targets[0] != target {
		t.Error("Target in TestRunner isn't the one we added")
	}
	// Now validate deletion
	tr.Del(target)
	if len(tr.targets) != 0 {
		t.Error("TestRunner didn't delete the target")
	}
}

func TestSet(t *testing.T) {
	tr := NewTestRunner(exampleCallbackChan, rate.NewLimiter(rate.Inf, 0))
	target, err := net.ResolveUDPAddr("udp", DefaultAddrStr)
	HandleError(err)
	targets := []*net.UDPAddr{target, target, target}
	tr.Set(targets)
	if len(tr.targets) != len(targets) {
		t.Error("Set targets, but TestRunner len doesn't match")
	}
	for k := range tr.targets {
		if tr.targets[k] != targets[k] {
			t.Error("Set targets, but TestRunner contents don't match")
		}
	}
}

func TestAddNewPort(t *testing.T) {
	// This is all generally tested as part of PortGroup already
	tr := NewTestRunner(exampleCallbackChan, rate.NewLimiter(rate.Inf, 0))
	tr.AddNewPort(
		DefaultAddrStr,
		DefaultTos,
		DefaultCacheTimeout,
		DefaultCacheCleanRate,
		DefaultReadTimeout,
	)
}

func TestNewTestRunner(t *testing.T) {
	// Just test creating one
	tr := NewTestRunner(exampleCallbackChan, rate.NewLimiter(rate.Inf, 0))
	if tr == nil {
		t.Error("New failed to create a TestRunner")
	}
}
