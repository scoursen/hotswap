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
