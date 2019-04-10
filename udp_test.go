package llama

import (
	"net"
	"testing"
)

func TestUnpackUdpData(t *testing.T) {
	// Test with good data first
	// "abcdefghij"
	signature := [10]byte{97, 98, 99, 100, 101, 102, 103, 104, 105, 106}
	data := UdpData{
		Signature: signature,
	}
	// staticcheck:  this value of err is never used (SA4006)
	bytes, err := PackUdpData(&data)
	unpacked, err := UnpackUdpData(bytes)
	if err != nil {
		t.Error("Was unable to unpack data successfully")
	}
	// Compare the actual structs
	if *unpacked != data {
		t.Error("Data unpacked, but lost in translation")
	}

	// Now verify that bad data doesn't work
	badData := []byte{1, 2, 3, 4, 5}
	// staticcheck:  this value of unpacked is never used (SA4006)
	unpacked, err = UnpackUdpData(badData)
	if err == nil {
		t.Error("No error returned for bad data")
	}
}

func TestSetTos(t *testing.T) {
	// Resolve a local addr
	myAddr, _ := net.ResolveUDPAddr("udp", ":0")
	// Create a connection
	conn, _ := net.ListenUDP("udp", myAddr)
	// Set the ToS value
	tosVal := 240
	newTos := byte(tosVal)
	SetTos(conn, newTos)
	// Verify the ToS value
	val := GetTos(conn)
	if val != newTos {
		t.Error("New ToS value not set correctly. Set", tosVal, "and got",
			val, "instead.")
	}
}
