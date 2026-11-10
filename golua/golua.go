package golua

import (
	"github.com/Shopify/go-lua"
	"github.com/tomgray/structlua"
)

// NewValue wraps the Lua value at the top of the stack (index -1)
// as a structlua.Value.
func NewValue(l *lua.State) structlua.Value {
	if l == nil || l.Top() == 0 {
		return structlua.NewValue(structlua.UnsupportedType("none"))
	}
	return valueAt(l, -1)
}

// NewValueAt wraps the Lua value at the given stack index as a structlua.Value.
func NewValueAt(l *lua.State, idx int) structlua.Value {
	if l == nil {
		return structlua.NewValue(structlua.UnsupportedType("none"))
	}
	return valueAt(l, idx)
}

func valueAt(l *lua.State, idx int) structlua.Value {
	idx = l.AbsIndex(idx)
	switch l.TypeOf(idx) {
	case lua.TypeNil:
		return structlua.Value{}
	case lua.TypeBoolean:
		return structlua.NewValue(l.ToBoolean(idx))
	case lua.TypeNumber:
		f, _ := l.ToNumber(idx)
		return structlua.NewValue(f)
	case lua.TypeString:
		s, _ := l.ToString(idx)
		return structlua.NewValue(s)
	case lua.TypeTable:
		return structlua.NewValue(tableSeq(l, idx))
	default:
		return structlua.NewValue(structlua.UnsupportedType(lua.TypeNameOf(l, idx)))
	}
}

func tableSeq(l *lua.State, tableIdx int) structlua.Table {
	return func(yield func(k, v structlua.Value) bool) {
		tableIdx = l.AbsIndex(tableIdx)
		origTop := l.Top()
		defer l.SetTop(origTop)

		l.PushNil()
		for l.Next(tableIdx) {
			k := valueAt(l, -2)
			v := valueAt(l, -1)
			if !yield(k, v) {
				return
			}
			l.SetTop(origTop + 1)
		}
	}
}
