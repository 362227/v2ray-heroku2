package domainsocket

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
)

func (c *Config) GetUnixAddr() (*net.UnixAddr, error) {
	path := c.Path
	if len(path) == 0 {
		return nil, newError("empty domain socket path")
	}
	if c.Abstract && path[0] != '\x00' {
		path = "\x00" + path
	}
	return &net.UnixAddr{
		Name: path,
		Net:  "unix",
	}, nil
}

func init() {
	common.Must(internet.RegisterProtocolConfigCreator(internet.TransportProtocol_DomainSocket, func() interface{} {
		return new(Config)
	}))
}
