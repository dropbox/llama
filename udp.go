package llama

import (
	"bytes"
	"encoding/binary"
	"errors"

	"golang.org/x/sys/unix" // The successor to syscall
	"net"
)

type UdpData struct {
	Signature [10]byte
	Tos       byte
	Sent      uint64
	Rcvd      uint64
	RTT       uint64
	Lost      uint8 // binary.Read doesn't handle bool correctly
}

// LocalUDPAddr returns the UDPAddr and net for the provided UDPConn.
//
// For UDPConn instances, net is generaly 'udp'.
func LocalUDPAddr(conn *net.UDPConn) (*net.UDPAddr, string, error) {
	addr := conn.LocalAddr()
	network := addr.Network()
	udpAddr, err := net.ResolveUDPAddr(network, addr.String())
	if err != nil {
		return udpAddr, network, err
	}
	return udpAddr, network, nil
}

// SetTos will set the IP_TOS value for the unix socket for the provided conn.
//
// TODO(dmar): May want to have these return the err, or actually handle.
// Could dedup there a bit. Maybe.
func SetTos(conn *net.UDPConn, tos byte) {
	file, err := conn.File()
	defer FileCloseHandler(file)
	HandleError(err)
	err = unix.SetsockoptByte(int(file.Fd()), unix.IPPROTO_IP,
		unix.IP_TOS, tos)
	HandleError(err)
}

// GetTos will get the IP_TOS value for the unix socket for the provided conn.
func GetTos(conn *net.UDPConn) byte {
	file, err := conn.File()
	defer FileCloseHandler(file)
	HandleError(err)
	value, err := unix.GetsockoptInt(int(file.Fd()), unix.IPPROTO_IP,
		unix.IP_TOS)
	HandleError(err)
	// Convert it to a byte and return
	return byte(value)
}

// EnableTimestamps enables kernel receive timestamping of packets on the
// provided conn.
//
// The timestamp values can later be extracted in the oob data from
// Receive.
func EnableTimestamps(conn *net.UDPConn) {
	file, err := conn.File()
	defer FileCloseHandler(file)
	HandleError(err)
	err = unix.SetsockoptInt(int(file.Fd()), unix.SOL_SOCKET,
		unix.SO_TIMESTAMPNS, 1)
	HandleError(err)
}

// TODO(dmar): These should be functions attached to `UdpData`
// PackUdpData takes a UdpData instances and converts it to a byte array.
func PackUdpData(data *UdpData) ([]byte, error) {
	byteBuffer := bytes.Buffer{}
	err := binary.Write(&byteBuffer, binary.LittleEndian, data)
	HandleError(err)
	packedData := byteBuffer.Bytes()
	return packedData, nil
}

// UnpackUdpData takes data and unpacks it into a UdpData struct, returning an
// error if the data was not compatible.
func UnpackUdpData(data []byte) (*UdpData, error) {
	/*
	   TODO(dmar): Using protocol buffers would make this easier and more
	   language agnostic in the future.
	*/
	// Cast it into the data struct
	unpackedData := UdpData{}
	// Need LittleEndian because network
	err := binary.Read(bytes.NewBuffer(data), binary.LittleEndian,
		&unpackedData)
	// If the data isn't properly formatted, skip it, return an error
	if err != nil {
		errMsg := errors.New("Invalid data received")
		return &unpackedData, errMsg
	}
	return &unpackedData, nil
}
