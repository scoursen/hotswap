package export

import "github.com/scoursen/hotswap/demo/trine/g"

type BetaExport interface {
	Two(str1 string, v1 g.Vector, str2 string, v2 g.Vector)
}
