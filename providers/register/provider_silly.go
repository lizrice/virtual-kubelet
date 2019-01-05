// +build !no_silly_provider

package register

import (
	"github.com/virtual-kubelet/virtual-kubelet/providers"
	"github.com/virtual-kubelet/virtual-kubelet/providers/silly"
)

func init() {
	register("silly", initSilly)
}

func initSilly(cfg InitConfig) (providers.Provider, error) {
	return silly.NewProvider(
		cfg.ConfigPath,
		cfg.ResourceManager,
		cfg.NodeName,
		cfg.OperatingSystem,
	)
}
