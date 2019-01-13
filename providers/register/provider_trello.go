// +build !no_tello_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/tello"
)

func init() {
	register("tello", initTello)
}

func initTello(cfg InitConfig) (providers.Provider, error) {
	return tello.NewProvider(
		cfg.ConfigPath,
		cfg.ResourceManager,
		cfg.NodeName,
		cfg.OperatingSystem,
	)
}
