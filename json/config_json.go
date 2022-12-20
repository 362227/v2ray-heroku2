package json

//go:generate go run $GOPATH/src/v2ray.com/core/common/errors/errorgen/main.go -pkg json -path Main,Json

import (
	"io"

	"v2ray.com/core"
	"v2ray.com/core/common"
	"v2ray.com/core/common/platform/ctlcmd"
)

func init() {
	common.Must(core.RegisterConfigLoader(&core.ConfigFormat{
		Name:      "JSON",
		Extension: []string{"json"},
		Loader: func(input io.Reader) (*core.Config, error) {
			jsonContent, err := ctlcmd.Run([]string{"config"}, input)
			if err != nil {
				return nil, newError("failed to execute v2ctl to convert config file.").Base(err).AtWarning()
			}
			return core.LoadConfig("protobuf", "", &jsonContent)
		},
	}))
}
