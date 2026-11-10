package gopherlua

import (
	"github.com/tomgray/structlua"
	lua "github.com/yuin/gopher-lua"
)

// NewValue wraps a gopher-lua LValue as a structlua.Value.
func NewValue(v lua.LValue) structlua.Value {
	if v == nil {
		return structlua.Value{}
	}
	switch val := v.(type) {
	case *lua.LNilType:
		return structlua.Value{}
	case lua.LBool:
		return structlua.NewValue(bool(val))
	case lua.LNumber:
		return structlua.NewValue(float64(val))
	case lua.LString:
		return structlua.NewValue(string(val))
	case *lua.LTable:
		return structlua.NewValue(structlua.Table(func(yield func(k, v structlua.Value) bool) {
			k := lua.LValue(lua.LNil)
			for {
				nextK, nextV := val.Next(k)
				if nextK == lua.LNil {
					break
				}
				if !yield(NewValue(nextK), NewValue(nextV)) {
					return
				}
				k = nextK
			}
		}))
	default:
		return structlua.NewValue(structlua.UnsupportedType(val.Type().String()))
	}
}
