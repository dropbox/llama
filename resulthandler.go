package llama

import (
	"errors"
	"log"
)

// Result defines characteristics of a single completed Probe.
type Result struct {
	Pd   *PathDist // Characteristics that make this path unique
	RTT  uint64    // Round trip time in nanoseconds
	Done uint64    // When the test completed (was received by Port) in ns
	Lost bool      // If the Probe was lost and never actually completed
}

// ResultHandler is a post-processor for Probes and converts them to Results.
type ResultHandler struct {
	in   chan *Probe  // Probes come in
	out  chan *Result // Results come out
	stop chan bool
}

// Run will start the ResultHandler in a new goroutine, and cause it to forever
// receive Probes, process them and pass their results out.
func (rh *ResultHandler) Run() {
	go rh.run()
}

func (rh *ResultHandler) run() {
	var result *Result
	for {
		// Get the probes
		select {
		case <-rh.stop:
			return // We're done here
		case probe := <-rh.in:
			result = Process(probe)
		}
		// Hand them off to the summarizer
		select {
		case <-rh.stop:
			return // We're done here
		case rh.out <- result:
		}
	}
}

// Stop will stop the rh.
func (rh *ResultHandler) Stop() {
	log.Println("Stopping ResultHandler")
	close(rh.stop)
}

// New creates a new ResultHandler that utilizes the provided in and out
// channels.
func NewResultHandler(in chan *Probe, out chan *Result) *ResultHandler {
	stop := make(chan bool)
	rh := ResultHandler{in: in, out: out, stop: stop}
	return &rh
}

// Process takes in a probe, performs calculations on it, and returns a
// Result.
func Process(probe *Probe) *Result {
	result := &Result{
		Pd:   probe.Pd,
		Done: probe.CRcvd,
	}
	// Add additional calculations here
	err := RTT(probe, result)
	HandleMinorError(err)
	return result
}

// RTT calculates the round trip time for a probe and updates the Result.
func RTT(probe *Probe, result *Result) error {
	if probe.CRcvd == 0 {
		// Probe timed out and was never received
		// Leave RTT as the zero value (0)
		result.Lost = true
		return nil
	}
	if probe.CSent > probe.CRcvd {
		// This should definitely not be happening
		err := errors.New("Probe send/recv times appear to be out of order")
		result.Lost = true
		return err
	}
	// result.Lost will retain the zero value of false
	rtt := probe.CRcvd - probe.CSent
	result.RTT = rtt
	return nil
}
