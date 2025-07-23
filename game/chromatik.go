package game

import "net"

type Chromatik struct {
	Connection *net.UDPConn
	Node       *net.UDPAddr
}

func (c *Chromatik) Send(bytes []byte) (int, error) {
	return c.Connection.WriteTo(bytes, c.Node)
}
