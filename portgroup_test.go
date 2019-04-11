package llama

import (
	"net"
	"testing"
)

var stopChan = make(chan bool)
var cbChan = make(chan *Probe)
var sendChan = make(chan *net.UDPAddr)

func TestNewPortGroup(t *testing.T) {
	// Create a new one
	pg := NewPortGroup(stopChan, cbChan, sendChan)
	if pg == nil {
		t.Error("New didn't return a PortGroup")
	}
	// Type checking is handled by building
}

func TestAdd(t *testing.T) {
	pg := NewPortGroup(stopChan, cbChan, sendChan)
	// Create the port and chan
	p := Port{}
	c := make(chan *net.UDPAddr)
	// Add them
	pg.Add(&p, c)
	// Make sure they're there
	if pg.ports[&p] != c {
		t.Error("Port and channel not correctly added")
	}
}

func TestAddNew(t *testing.T) {
	pg := NewPortGroup(stopChan, cbChan, sendChan)
	// Add a new port
	p, c := pg.AddNew(DefaultAddrStr, DefaultTos,
		DefaultCacheTimeout, DefaultCacheCleanRate,
		DefaultReadTimeout)
	// Make sure it's in the PortGroup
	if pg.ports[p] != c {
		t.Error("New port/channel was not added correctly")
	}
}

func TestDel(t *testing.T) {
	pg := NewPortGroup(stopChan, cbChan, sendChan)
	// Create the port and chan
	p := Port{}
	c := make(chan *net.UDPAddr)
	// Add them
	pg.Add(&p, c)
	// Delete them
	pg.Del(&p)
	// Make sure it's gone
	if pg.ports[&p] != nil {
		t.Error("Port still exists after deletion")
	}
}

func TestPortGroupRun(t *testing.T) {
	// TODO(dmar): This needs more complex mocking
}

func TestMux(t *testing.T) {
	pg := NewPortGroup(stopChan, cbChan, sendChan)
	// Create the port and chan
	p1 := Port{}
	p2 := Port{}
	// If these aren't buffered, they'll just fail later
	c1 := make(chan *net.UDPAddr, 10)
	c2 := make(chan *net.UDPAddr, 10)
	// Add them
	pg.Add(&p1, c1)
	pg.Add(&p2, c2)
	// Send an addr
	addr, err := net.ResolveUDPAddr("udp", DefaultAddrStr)
	if err != nil {
		t.Error("Hit error resolving addr")
	}
	// And forcibly mux it
	pg.mux(addr)
	// Make sure we get both
	select {
	case addr1 := <-c1:
		if addr1 != addr {
			t.Error("Muxed value doesn't match what was passed in")
		}
	default:
		t.Error("Passed in value wasn't muxed")
	}
	select {
	case addr2 := <-c2:
		if addr2 != addr {
			t.Error("Muxed value doesn't match what was passed in")
		}
	default:
		t.Error("Passed in value wasn't muxed")
	}

}

func TestPortGroupStop(t *testing.T) {
	pg := NewPortGroup(stopChan, cbChan, sendChan)
	pg.Stop()
	// Make sure stop actually closes it
	select {
	case <-stopChan:
	default:
		t.Error("Channel wasn't closed after calling Stop")
	}
}
