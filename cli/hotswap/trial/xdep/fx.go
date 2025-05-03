package xdep

import (
	"github.com/scoursen/hotswap/cli/hotswap/trial/export/arya"
	"github.com/scoursen/hotswap/cli/hotswap/trial/export/importall"
	"github.com/scoursen/hotswap/cli/hotswap/trial/export/snow"
)

var fx struct {
	ImportAll importall.Export
	Arya      arya.Export
}

var fxMini struct {
	Arya arya.Export
}

var fxIgnore struct {
	ImportAll importall.Export
	Arya      arya.Export

	Xtypo1 interface {
		Xtypo()
	} `hotswap:"-"`
}

var fxUnknown struct {
	Xtypo2 interface {
		Xtypo()
	}
}

var fxNameless struct {
	arya.Export
}

var fxStark struct {
	Arya arya.Export
	Snow snow.Export
}
