package turn

import (
	"fmt"
	"net"

	"github.com/pion/randutil"
	"github.com/pion/transport/v2"
	"github.com/pion/transport/v2/stdnet"
)

// CustomRelayAddressGenerator can be used to only allocate connections inside a defined port range.
// Similar to the RelayAddressGeneratorStatic a static ip address can be set.
type CustomRelayAddressGenerator struct {
	// RelayAddress is the IP returned to the user when the relay is created
	RelayAddress net.IP

	MinPort uint16
	MaxPort uint16
	Rand    randutil.MathRandomGenerator
	Address string
	Net     transport.Net
}

func NewGenerator(min, max int, ip string) (*CustomRelayAddressGenerator, error) {
	Net, err := stdnet.NewNet()
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	return &CustomRelayAddressGenerator{
		MinPort:      uint16(min),
		MaxPort:      uint16(max),
		Rand:         randutil.NewMathRandomGenerator(),
		RelayAddress: net.ParseIP(ip),
		Address:      "0.0.0.0",
		Net:          Net,
	}, nil
}

// Validate is called on server startup and confirms the RelayAddressGenerator is properly configured
func (r *CustomRelayAddressGenerator) Validate() error {
	return nil
}

// AllocatePacketConn generates a new PacketConn to receive traffic on and the IP/Port to populate the allocation response with
func (r *CustomRelayAddressGenerator) AllocatePacketConn(network string, requestedPort int) (net.PacketConn, net.Addr, error) {
	if requestedPort != 0 {
		conn, err := r.Net.ListenPacket(network, fmt.Sprintf("%s:%d", r.Address, requestedPort))
		if err != nil {
			return nil, nil, err
		}
		relayAddr, ok := conn.LocalAddr().(*net.UDPAddr)
		if !ok {
			return nil, nil, fmt.Errorf("nil conn")
		}

		relayAddr.IP = r.RelayAddress
		return conn, relayAddr, nil
	}

	for try := 0; try < 16; try++ {
		port := r.MinPort + uint16(r.Rand.Intn(int((r.MaxPort+1)-r.MinPort)))
		conn, err := r.Net.ListenPacket(network, fmt.Sprintf("%s:%d", r.Address, port))
		if err != nil {
			continue
		}

		relayAddr, ok := conn.LocalAddr().(*net.UDPAddr)
		if !ok {
			return nil, nil, fmt.Errorf("errNilConn")
		}

		relayAddr.IP = r.RelayAddress
		return conn, relayAddr, nil
	}

	return nil, nil, fmt.Errorf("errMaxRetriesExceeded")
}

// AllocateConn generates a new Conn to receive traffic on and the IP/Port to populate the allocation response with
func (r *CustomRelayAddressGenerator) AllocateConn(string, int) (net.Conn, net.Addr, error) {
	return nil, nil, fmt.Errorf("errTODO")
}
