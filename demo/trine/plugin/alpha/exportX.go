package alpha

import (
	"github.com/scoursen/hotswap/demo/trine/g"
	"github.com/scoursen/hotswap/demo/trine/plugin/alpha/aimpl"
)

type exportX struct{}

func (_ exportX) One(str1 string, v1 g.Vector) {
	aimpl.One(str1, v1, pluginName, CompileTimeString)
}
