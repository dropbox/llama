package llama

import (
	"testing"
	"time"
)

func TestResultHandlerRun(t *testing.T) {
	// Create the TestHandler
	in := make(chan *Probe)
	out := make(chan *Result)
	rh := NewResultHandler(in, out)
	// Start it
	rh.Run()
	// Create a probe
	// Use zero values here, but test more intensely further down
	probe := &Probe{}
	// Provide the probe to the input
	select {
	case in <- probe:
	case <-time.After(time.Second):
		t.Error("Was unable to provide the probe to input, blocked channel")
	}
	// Verify it comes out
	select {
	case <-out:
	case <-time.After(time.Second):
		t.Error("Result was not processed/output within 1 second")
	}
}

func TestResultHandlerStop(t *testing.T) {
	in := make(chan *Probe)
	out := make(chan *Result)
	rh := NewResultHandler(in, out)
	// Make sure it's not already closed
	select {
	case <-rh.stop:
		t.Error("ResultHandle stopped before being told to")
	default:
	}
	// Close it and confirm
	rh.Stop()
	select {
	case <-rh.stop:
	default:
		t.Error("ResultHandle was told to stop but doesn't appear to be")
	}
}

func TestNewResultHandler(t *testing.T) {
	in := make(chan *Probe)
	out := make(chan *Result)
	rh := NewResultHandler(in, out)
	if rh.in != in {
		t.Error("Provided input channel doesn't match on ResultHandler")
	}
	if rh.out != out {
		t.Error("Provided output channel doesn't match on ResultHandler")
	}
}

func TestProcess(t *testing.T) {
	pd := &PathDist{}
	probe := &Probe{
		Pd:    pd,
		CSent: uint64(100000),
		CRcvd: uint64(200000),
		Tos:   byte(0),
	}
	// Make sure it processes correctly, but leave specific cases to any tests
	// below.
	result := Process(probe)
	if result.Pd != pd {
		t.Error("PathDist doesn't match between Probe and Result")
	}
	// Make sure the RTT is calculated
	if result.RTT != 100000 {
		t.Error("RTT was not correctly calculated")
	}
	// This shouldn't be marked as lost
	if result.Lost == true {
		t.Error("Result indicates Lost when it shouldn't")
	}
	// The Done time should make the CRcvd time on the Probe
	if result.Done != probe.CRcvd {
		t.Error("CRcvd time wasn't propagated to Result")
	}
}

func TestRTT(t *testing.T) {
	probe := &Probe{
		CSent: uint64(100000),
		CRcvd: uint64(200000),
	}
	result := &Result{}
	// Calculate positive RTT and verify
	err := RTT(probe, result)
	if err != nil {
		t.Error("Hit error unexpectedly calculating positve RTT")
	}
	if result.RTT != 100000 {
		t.Error("RTT calculation incorrect. Got", result.RTT,
			", expected 100000")
	}
	// Handle a lost probe and verify zero RTT and mark as loss
	probe = &Probe{
		CSent: uint64(100000),
	}
	result = &Result{} // Reset the Result
	err = RTT(probe, result)
	if err != nil {
		t.Error("Hit error unexpectedly calculating RTT of lost probe")
	}
	if result.RTT != 0 || result.Lost != true {
		t.Errorf("%+v\n", *probe)
		t.Errorf("%+v\n", *result)
		t.Error("Lost probe with non-zero RTT or not marked as lost")
	}
	// Handle a probe that has mixed up sent/recv times
	probe = &Probe{
		CSent: uint64(200000),
		CRcvd: uint64(100000),
	}
	result = &Result{} // Reset the Result
	// Calculate positive RTT and verify
	err = RTT(probe, result)
	if err == nil {
		t.Error("Didn't hit an error when we should have for mixed up times")
	}
	if result.RTT != 0 || result.Lost != true {
		t.Error("Mixed up probe with non-zero RTT or not marked as lost")
	}
}
