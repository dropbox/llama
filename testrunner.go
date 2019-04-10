package llama

import (
	"context"
	"log"
	"net"
	"sync"
	"time"
	"golang.org/x/time/rate"
)

// TestRunner repeatedly runs through a list of targets and passes them down
// to a PortGroup for processing.
type TestRunner struct {
	pg      *PortGroup
	tosend  chan *net.UDPAddr
	rl      *rate.Limiter
	stop    chan bool
	mutex   sync.RWMutex
	targets []*net.UDPAddr
	// deadcode: running is grandfathered in as legacy code
	running bool
}

// Run starts the TestRunner and begins cycling through targets.
func (tr *TestRunner) Run() {
	tr.pg.Run() // Start running the PortGroup and underlying Ports in goroutines
	go tr.run()
}

func (tr *TestRunner) run() {
	for {
		// If we've been signalled to stop, close out
		if tr.isStopped() {
			return
		}
		// Check if we can actually start first
		// If over the rate limit, this will block until permitted
		err := tr.rl.Wait(context.Background())
		HandleError(err)
		// Since we may have been throttled, and possibly stopped in the
		// meantime, check again.
		if tr.isStopped() {
			return
		}
		tr.cycleTargets()
	}
}

// cycleTargets goes through all of the stored targets and passes them off
// to the PortGroup via the tosend channel.
func (tr *TestRunner) cycleTargets() {
	// Acquire the lock for `tr.targets`
	tr.mutex.RLock()
	defer tr.mutex.RUnlock()
	// TODO(dmar): Track how long it takes for cycles to complete.
	//      The tough part here is that there can be multiple TestRunners
	//      with a varying number of targets, so they aren't all treated
	//      equally.
	for _, target := range tr.targets {
		// TODO(dmar): It's probably cleaner to just provide access to this
		//      on `tr.pg` and call that, as opposed to keeping track of
		//      the channel itself.
		// TODO(dmar): It's possible for this to block if things lower
		//      down can't keep up. Leaving it that way for now, however
		//      it may be desirable to allow some kind of "out" in the
		//      future.
		tr.tosend <- target
	}
	// Cycle is complete here.
	// TODO(dmar): This really is just referring to the ability to pass
	//      off the targets. Doesn't actually mean everything below
	//      finished. But it's at least a reasonable signal of the ability
	//      to send.
}

// Stop will stop the TestRunner after the current cycle and any underlying
// PortGroup and Port(s).
func (tr *TestRunner) Stop() {
	log.Println("Initiating Stop in TestRunner")
	close(tr.stop)
	// Release the portgroup
	tr.pg = nil
}

// isStopped evaluates if the TestRunner has been stopped.
func (tr *TestRunner) isStopped() bool {
	select {
	case <-tr.stop:
		return true
	default:
		return false
	}
}

// Add will add a variable number of addrs to the slice of targets for
// processing.
//
// NOTE: This will block during cycles. So it should be avoided when possible.
//       It's better to just use `Set` to replace the whole thing. Either way,
//       this change will only go into effect between cycles.
func (tr *TestRunner) Add(addrs ...*net.UDPAddr) {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()
	tr.targets = append(tr.targets, addrs...)
}

// Del will remove all occurrences of a target addr from the slice of targets.
//
// NOTE: This will block during cycles. It will also take longer as the
//       number of targets increases. So it should be avoided when possible.
//       It's better to just use `Set` to replace the whole thing. Either way,
//       this change will only go into effect between cycles.
func (tr *TestRunner) Del(addr *net.UDPAddr) {
	tr.mutex.Lock()
	defer tr.mutex.Unlock()
	// Find the element
	for i, v := range tr.targets {
		if v == addr {
			// Delete the element
			// This doesn't preserve order because it shouldn't matter.
			// Also it's WAY more efficient, especially at scale.
			tr.targets[i] = tr.targets[len(tr.targets)-1]
			tr.targets[len(tr.targets)-1] = nil
			tr.targets = tr.targets[:len(tr.targets)-1]
		}
	}
}

// Set will replace the current slice of targets with the provided one.
//
// NOTE: This will block during cycles. It is generally advised to use `Set`
//       over `Add` and `Del` in making larger changes or operating on multiple
//       targets. It's just more atomic.
func (tr *TestRunner) Set(targets []*net.UDPAddr) {
	tr.mutex.Lock()
	tr.targets = targets
	defer tr.mutex.Unlock()
}

// AddNewPort will add a new Port to the TestRunner's PortGroup.
//
// See PortGroup.AddNew for more details on these arguments.
//
// NOTE: This is basically just a passthrough for PortGroup.AddNew until
//       the pattern is better understood and this can be cleaned up.
func (tr *TestRunner) AddNewPort(portStr string, tos byte,
	cTimeout time.Duration,
	cCleanRate time.Duration,
	readTimeout time.Duration) {
	// TODO(dmar): This must not be running already. Add enforcement.
	tr.pg.AddNew(portStr, tos, cTimeout, cCleanRate, readTimeout)
}

// New creates and returns a new TestRunner instance.
//
// `cbc` is a channel for accepting completed Probes.
// `rl` is a rate limiter which is used to throttle the number of cycles that
// may be completed per second.
func NewTestRunner(cbc chan *Probe, rl *rate.Limiter) *TestRunner {
	// TODO(dmar): What about providing this on creation? Perhaps an option at
	//      some point, but just use Set for now.
	//targets := make([]*net.UDPAddr)
	var targets []*net.UDPAddr
	tosend := make(chan *net.UDPAddr)
	stop := make(chan bool)
	pg := NewPortGroup(stop, cbc, tosend)
	tr := TestRunner{
		pg:      pg,
		tosend:  tosend,
		rl:      rl,
		stop:    stop,
		targets: targets,
	}
	return &tr
}
