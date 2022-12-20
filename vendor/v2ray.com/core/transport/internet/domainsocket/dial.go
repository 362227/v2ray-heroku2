// +build !windows

package domainsocket

import (
	"context"

	"v2ray.com/core/common"
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/transport/internet/tls"
)

func getSettingsFromContext(ctx context.Context) *Config {
	rawSettings := internet.TransportSettingsFromContext(ctx)
	if rawSettings == nil {
		return nil
	}
	return rawSettings.(*Config)
}

func Dial(ctx context.Context, dest net.Destination) (internet.Connection, error) {
	settings := getSettingsFromContext(ctx)
	if settings == nil {
		return nil, newError("domain socket settings is not specified.").AtError()
	}

	addr, err := settings.GetUnixAddr()
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return nil, newError("failed to dial unix: ", settings.Path).Base(err).AtWarning()
	}

	if config := tls.ConfigFromContext(ctx); config != nil {
		return tls.Client(conn, config.GetTLSConfig(tls.WithDestination(dest))), nil
	}

	return conn, nil
}

func init() {
	common.Must(internet.RegisterTransportDialer(internet.TransportProtocol_DomainSocket, Dial))
}
