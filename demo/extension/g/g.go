package g

import (
	"github.com/scoursen/hotswap"
	"github.com/edwingeng/slog"
)

var (
	Logger = slog.NewDevelopmentConfig().MustBuild()
)

var (
	PluginManagerSwapper *hotswap.PluginManagerSwapper
)

type VaultExtension struct {
	Meow func(repeat int)
}

func NewVaultExtension() interface{} {
	return &VaultExtension{}
}
