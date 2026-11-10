package structlua

import (
	"fmt"
	"iter"
	"math"
	"reflect"
	"strings"
)

// Table is a standard Go range-over-func iterator yielding key-value pairs of a Lua table.
type Table iter.Seq2[Value, Value]

// UnsupportedType represents a Lua value type that cannot be unmarshaled as data
// (e.g. "function", "userdata", "thread", "none").
type UnsupportedType string

// Value holds a Lua value constrained at compile time to nil, bool, float64, string,
// or Table. The zero Value{} represents Lua nil.
type Value struct {
	v any
}

// NewValue constructs a Value from any valid Lua primitive or Table.
func NewValue[T ~bool | ~float64 | ~string | Table](v T) Value {
	return Value{v: v}
}

// Any returns the underlying Go value (nil, bool, float64, string, Table, or UnsupportedType)
// for standard Go type assertions and type switches.
func (v Value) Any() any {
	return v.v
}

// TypeName returns the Lua type name for v (e.g. "nil", "boolean", "number", "string", "table").
func TypeName(v Value) string {
	switch val := v.Any().(type) {
	case nil:
		return "nil"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case Table:
		return "table"
	case UnsupportedType:
		return string(val)
	default:
		return fmt.Sprintf("%T", val)
	}
}

type typeHandler func(v Value) (any, error)

var anyType = reflect.TypeOf((*any)(nil)).Elem()

// LuaUnmarshaler is the interface implemented by types that can unmarshal
// a Lua description of themselves.
type LuaUnmarshaler interface {
	UnmarshalLua(v Value) error
}

// Option configures an Unmarshaler.
type Option interface {
	Apply(*Unmarshaler)
}

type handlerOption[T any] struct {
	handler func(v Value) (T, error)
}

func (h handlerOption[T]) Apply(u *Unmarshaler) {
	_ = u.RegisterHandler[T](h.handler)
}

// WithHandler registers a custom handler for type T (interfaces or concrete types)
// during NewUnmarshaler.
func WithHandler[T any](handler func(v Value) (T, error)) Option {
	return handlerOption[T]{handler: handler}
}

// Polymorphic creates a PolymorphicBuilder for type T using the specified discriminator field.
// It implements Option and lazily registers the polymorphic handler when passed to NewUnmarshaler.
func Polymorphic[T any](field string) *PolymorphicBuilder[T] {
	return &PolymorphicBuilder[T]{
		field:    field,
		mappings: make(map[string]func(v Value) (T, error)),
	}
}

// PolymorphicBuilder builds a polymorphic handler for type T based on a discriminator field.
// It implements the Option interface for NewUnmarshaler.
type PolymorphicBuilder[T any] struct {
	u        *Unmarshaler
	field    string
	mappings map[string]func(v Value) (T, error)
}

// Apply registers the polymorphic handler onto the Unmarshaler.
func (b *PolymorphicBuilder[T]) Apply(u *Unmarshaler) {
	b.u = u
	targetType := reflect.TypeOf((*T)(nil)).Elem()
	_ = u.RegisterHandler[T](func(v Value) (T, error) {
		var zero T
		if b.field == "" {
			return zero, fmt.Errorf("discriminator field cannot be empty")
		}
		tbl, ok := v.Any().(Table)
		if !ok {
			return zero, fmt.Errorf("expected table for %v, got %s", targetType, TypeName(v))
		}

		var (
			found   bool
			discVal Value
		)
		for k, val := range tbl {
			if kStr, ok := k.Any().(string); ok && kStr == b.field {
				found = true
				discVal = val
				break
			}
		}

		if !found {
			return zero, fmt.Errorf("expected string for discriminator field %q, got nil", b.field)
		}
		discriminator, ok := discVal.Any().(string)
		if !ok {
			return zero, fmt.Errorf("expected string for discriminator field %q, got %s", b.field, TypeName(discVal))
		}

		handler, ok := b.mappings[discriminator]
		if !ok {
			return zero, fmt.Errorf("unknown %v type %q", targetType, discriminator)
		}

		return handler(v)
	})
}

// Add registers a strongly-typed mapping function for a discriminator value.
// structlua creates the zero value of C (value or pointer), unmarshals the Lua table into it,
// and invokes fn to produce the domain type T.
func (b *PolymorphicBuilder[T]) Add[C any](discriminator string, fn func(C) (T, error)) *PolymorphicBuilder[T] {
	b.mappings[discriminator] = func(v Value) (T, error) {
		var cfg C
		if err := b.u.Unmarshal(v, &cfg); err != nil {
			var zero T
			return zero, err
		}
		return fn(cfg)
	}
	return b
}

// AddType registers a concrete implementation type C that implements T,
// without requiring a mapping function.
// structlua creates the zero value of C (by value or pointer) and unmarshals the
// Lua table directly into it.
func (b *PolymorphicBuilder[T]) AddType[C any](discriminator string) *PolymorphicBuilder[T] {
	return b.Add(discriminator, func(target C) (T, error) {
		if val, ok := any(target).(T); ok {
			return val, nil
		}
		var zero T
		return zero, fmt.Errorf("type %T does not implement target interface", target)
	})
}

// Unmarshaler unmarshals Lua values into Go values and manages custom type handlers.
type Unmarshaler struct {
	handlers map[reflect.Type]typeHandler
}

func (u *Unmarshaler) init() {
	if u.handlers == nil {
		u.handlers = make(map[reflect.Type]typeHandler)
		u.handlers[anyType] = func(v Value) (any, error) {
			return u.unmarshalAny(v)
		}
	}
}

// NewUnmarshaler creates a new Unmarshaler instance with optional configuration options.
func NewUnmarshaler(opts ...Option) *Unmarshaler {
	u := &Unmarshaler{}
	for _, opt := range opts {
		if opt != nil {
			opt.Apply(u)
		}
	}
	return u
}

// RegisterHandler registers a custom handler for type T (interfaces or concrete types).
func (u *Unmarshaler) RegisterHandler[T any](handler func(v Value) (T, error)) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}
	u.init()
	targetType := reflect.TypeOf((*T)(nil)).Elem()
	u.handlers[targetType] = func(v Value) (any, error) {
		return handler(v)
	}
	return nil
}

// Unmarshal extracts a Lua value into target pointer v.
func (u *Unmarshaler) Unmarshal(val Value, target any) error {
	if ut, ok := val.Any().(UnsupportedType); ok && ut == "none" {
		return fmt.Errorf("Lua stack is empty")
	}

	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("Unmarshal requires a non-nil pointer, got %T", target)
	}

	u.init()
	return u.unmarshalValue(val, rv.Elem(), "")
}

type structFieldInfo struct {
	index int
	name  string
}

func getStructFields(structType reflect.Type) []structFieldInfo {
	fields := make([]structFieldInfo, 0, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}

		tag := field.Tag.Get("lua")
		if tag == "-" {
			continue
		}

		key := tag
		if idx := strings.Index(key, ","); idx != -1 {
			key = key[:idx]
		}
		if key == "" {
			key = field.Name
		}

		fields = append(fields, structFieldInfo{
			index: i,
			name:  key,
		})
	}
	return fields
}

func findField(fields []structFieldInfo, luaKey string) (structFieldInfo, bool, bool) {
	// First pass: exact match
	for _, f := range fields {
		if f.name == luaKey {
			return f, true, true
		}
	}
	// Second pass: case-insensitive match
	for _, f := range fields {
		if strings.EqualFold(f.name, luaKey) {
			return f, true, false
		}
	}
	return structFieldInfo{}, false, false
}

func (u *Unmarshaler) unmarshalStruct(v Value, structVal reflect.Value, fieldName string) error {
	tbl, ok := v.Any().(Table)
	if !ok {
		return fmt.Errorf("type mismatch for field %q: expected table, got %s", fieldName, TypeName(v))
	}

	fields := getStructFields(structVal.Type())
	matchedExact := make(map[int]bool)

	for k, val := range tbl {
		luaKey, ok := k.Any().(string)
		if !ok {
			return fmt.Errorf("expected object table with string keys, got %s key", TypeName(k))
		}
		if fInfo, found, isExact := findField(fields, luaKey); found {
			if !isExact && matchedExact[fInfo.index] {
				// Do not overwrite an exact match with a case-insensitive match
				continue
			}
			if isExact {
				matchedExact[fInfo.index] = true
			}
			if err := u.unmarshalField(val, structVal.Field(fInfo.index), fInfo.name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (u *Unmarshaler) unmarshalField(v Value, fieldVal reflect.Value, fieldName string) error {
	if v.Any() == nil {
		return nil
	}
	return u.unmarshalValue(v, fieldVal, fieldName)
}

var luaUnmarshalerType = reflect.TypeOf((*LuaUnmarshaler)(nil)).Elem()

func (u *Unmarshaler) unmarshalCustom(v Value, fieldVal reflect.Value) (bool, error) {
	var target LuaUnmarshaler

	if fieldVal.CanAddr() && fieldVal.Addr().Type().Implements(luaUnmarshalerType) {
		target = fieldVal.Addr().Interface().(LuaUnmarshaler)
	} else if fieldVal.Type().Implements(luaUnmarshalerType) {
		if fieldVal.Kind() == reflect.Pointer && fieldVal.IsNil() {
			fieldVal.Set(reflect.New(fieldVal.Type().Elem()))
		} else if fieldVal.Kind() == reflect.Interface && fieldVal.IsNil() {
			return false, nil
		}
		target = fieldVal.Interface().(LuaUnmarshaler)
	} else {
		return false, nil
	}

	return true, target.UnmarshalLua(v)
}

func (u *Unmarshaler) callHandler(v Value, fieldVal reflect.Value, handler typeHandler) error {
	val, err := handler(v)
	if err != nil {
		return err
	}
	if val != nil {
		fieldVal.Set(reflect.ValueOf(val))
	} else {
		fieldVal.Set(reflect.Zero(fieldVal.Type()))
	}
	return nil
}

func (u *Unmarshaler) unmarshalValue(v Value, fieldVal reflect.Value, fieldName string) error {
	if handler := u.handlers[fieldVal.Type()]; handler != nil {
		return u.callHandler(v, fieldVal, handler)
	}

	if ok, err := u.unmarshalCustom(v, fieldVal); ok {
		return err
	}

	switch fieldVal.Kind() {
	case reflect.String:
		return u.unmarshalString(v, fieldVal, fieldName)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return u.unmarshalInt(v, fieldVal, fieldName)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return u.unmarshalUint(v, fieldVal, fieldName)

	case reflect.Float32, reflect.Float64:
		return u.unmarshalFloat(v, fieldVal, fieldName)

	case reflect.Bool:
		return u.unmarshalBool(v, fieldVal, fieldName)

	case reflect.Slice, reflect.Array:
		return u.unmarshalArray(v, fieldVal, fieldName)

	case reflect.Struct:
		return u.unmarshalStruct(v, fieldVal, fieldName)

	case reflect.Pointer:
		return u.unmarshalPointer(v, fieldVal, fieldName)

	case reflect.Map:
		return u.unmarshalMap(v, fieldVal, fieldName)

	case reflect.Interface:
		return u.unmarshalInterface(v, fieldVal, fieldName)

	default:
		return fmt.Errorf("unsupported type %s for field %q", fieldVal.Type(), fieldName)
	}
}

func (u *Unmarshaler) unmarshalArray(v Value, val reflect.Value, fieldName string) error {
	tbl, ok := v.Any().(Table)
	if !ok {
		return fmt.Errorf("type mismatch for field %q: expected array table, got %s", fieldName, TypeName(v))
	}

	count := 0
	maxKey := 0

	for k := range tbl {
		count++
		f, ok := k.Any().(float64)
		if !ok {
			return fmt.Errorf("type mismatch for field %q: expected array table, got %s key", fieldName, TypeName(k))
		}
		idx := int(f)
		if f < 1 || float64(idx) != f {
			return fmt.Errorf("type mismatch for field %q: expected contiguous positive integer key, got %v", fieldName, f)
		}
		if idx > maxKey {
			maxKey = idx
		}
	}

	if count != maxKey {
		return fmt.Errorf("type mismatch for field %q: expected contiguous array table, got %d keys with max index %d", fieldName, count, maxKey)
	}

	n := maxKey
	target := val
	if val.Kind() == reflect.Slice {
		target = reflect.MakeSlice(val.Type(), maxKey, maxKey)
	} else if n > val.Len() {
		n = val.Len()
	}

	for k, elem := range tbl {
		idx := int(k.Any().(float64))
		if idx >= 1 && idx <= n {
			if err := u.unmarshalField(elem, target.Index(idx-1), fmt.Sprintf("%s[%d]", fieldName, idx-1)); err != nil {
				return err
			}
		}
	}

	if val.Kind() == reflect.Slice {
		val.Set(target)
	}
	return nil
}

func (u *Unmarshaler) unmarshalString(v Value, fieldVal reflect.Value, fieldName string) error {
	str, ok := v.Any().(string)
	if !ok {
		return fmt.Errorf("type mismatch for field %q: expected string, got %s", fieldName, TypeName(v))
	}
	fieldVal.SetString(str)
	return nil
}

func toNumber(v Value, fieldVal reflect.Value, fieldName string) (float64, error) {
	f, ok := v.Any().(float64)
	if !ok {
		return 0, fmt.Errorf("type mismatch for field %q: expected %s, got %s", fieldName, fieldVal.Type(), TypeName(v))
	}
	return f, nil
}

func (u *Unmarshaler) unmarshalInt(v Value, fieldVal reflect.Value, fieldName string) error {
	f, err := toNumber(v, fieldVal, fieldName)
	if err != nil {
		return err
	}
	if f < math.MinInt64 || f > math.MaxInt64 || math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("number %v out of range for %s field %q", f, fieldVal.Type(), fieldName)
	}
	n := int64(f)
	if fieldVal.OverflowInt(n) {
		return fmt.Errorf("number %v overflows %s field %q", f, fieldVal.Type(), fieldName)
	}
	fieldVal.SetInt(n)
	return nil
}

func (u *Unmarshaler) unmarshalUint(v Value, fieldVal reflect.Value, fieldName string) error {
	f, err := toNumber(v, fieldVal, fieldName)
	if err != nil {
		return err
	}
	if f < 0 {
		return fmt.Errorf("cannot unmarshal negative number %v into uint field %q", f, fieldName)
	}
	if f > math.MaxUint64 || math.IsNaN(f) || math.IsInf(f, 0) {
		return fmt.Errorf("number %v out of range for %s field %q", f, fieldVal.Type(), fieldName)
	}
	uVal := uint64(f)
	if fieldVal.OverflowUint(uVal) {
		return fmt.Errorf("number %v overflows %s field %q", f, fieldVal.Type(), fieldName)
	}
	fieldVal.SetUint(uVal)
	return nil
}

func (u *Unmarshaler) unmarshalFloat(v Value, fieldVal reflect.Value, fieldName string) error {
	f, err := toNumber(v, fieldVal, fieldName)
	if err != nil {
		return err
	}
	if fieldVal.Kind() == reflect.Float32 && fieldVal.OverflowFloat(f) {
		return fmt.Errorf("number %v overflows float32 field %q", f, fieldName)
	}
	fieldVal.SetFloat(f)
	return nil
}

func (u *Unmarshaler) unmarshalBool(v Value, fieldVal reflect.Value, fieldName string) error {
	b, ok := v.Any().(bool)
	if !ok {
		return fmt.Errorf("type mismatch for field %q: expected bool, got %s", fieldName, TypeName(v))
	}
	fieldVal.SetBool(b)
	return nil
}

func (u *Unmarshaler) unmarshalPointer(v Value, fieldVal reflect.Value, fieldName string) error {
	if v.Any() == nil {
		fieldVal.Set(reflect.Zero(fieldVal.Type()))
		return nil
	}
	elemType := fieldVal.Type().Elem()
	if fieldVal.IsNil() {
		fieldVal.Set(reflect.New(elemType))
	}
	return u.unmarshalValue(v, fieldVal.Elem(), fieldName)
}

func (u *Unmarshaler) unmarshalMap(v Value, mapVal reflect.Value, fieldName string) error {
	tbl, ok := v.Any().(Table)
	if !ok {
		return fmt.Errorf("type mismatch for field %q: expected table for map, got %s", fieldName, TypeName(v))
	}

	mapType := mapVal.Type()
	keyType := mapType.Key()
	elemType := mapType.Elem()

	if keyType.Kind() != reflect.String {
		return fmt.Errorf("unsupported map key type %s for field %q (only string keys supported)", keyType, fieldName)
	}

	if mapVal.IsNil() {
		mapVal.Set(reflect.MakeMap(mapType))
	}

	for k, val := range tbl {
		luaKey, ok := k.Any().(string)
		if !ok {
			return fmt.Errorf("type mismatch for field %q: expected string map key, got %s", fieldName, TypeName(k))
		}
		keyVal := reflect.ValueOf(luaKey)

		elemVal := reflect.New(elemType).Elem()
		if err := u.unmarshalField(val, elemVal, fmt.Sprintf("%s[%s]", fieldName, luaKey)); err != nil {
			return err
		}
		mapVal.SetMapIndex(keyVal, elemVal)
	}
	return nil
}

func (u *Unmarshaler) unmarshalInterface(v Value, fieldVal reflect.Value, fieldName string) error {
	if !fieldVal.IsNil() && fieldVal.Elem().Kind() == reflect.Pointer {
		return u.unmarshalValue(v, fieldVal.Elem(), fieldName)
	}
	return fmt.Errorf("unsupported interface type %s for field %q", fieldVal.Type(), fieldName)
}

func (u *Unmarshaler) unmarshalAny(v Value) (any, error) {
	switch val := v.Any().(type) {
	case nil:
		return nil, nil
	case bool:
		return val, nil
	case float64:
		return val, nil
	case string:
		return val, nil
	case Table:
		return u.unmarshalTableToAny(v)
	default:
		return nil, fmt.Errorf("cannot unmarshal Lua %s into any", TypeName(v))
	}
}

func (u *Unmarshaler) unmarshalTableToAny(v Value) (any, error) {
	var arr []any
	if err := u.unmarshalArray(v, reflect.ValueOf(&arr).Elem(), ""); err == nil && len(arr) > 0 {
		return arr, nil
	}

	var m map[string]any
	if err := u.unmarshalMap(v, reflect.ValueOf(&m).Elem(), ""); err != nil {
		return nil, err
	}
	return m, nil
}

// Unmarshal is a package-level helper that unmarshals using the default Unmarshaler.
func Unmarshal(val Value, target any) error {
	return NewUnmarshaler().Unmarshal(val, target)
}
