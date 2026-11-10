package structlua_test

import (
	"reflect"
	"testing"

	"github.com/tomgray/structlua"
)

type shape interface {
	Area() float64
}

func TestRegisterHandlerValidation(t *testing.T) {
	registrationTests := []struct {
		name    string
		regFunc func(u *structlua.Unmarshaler) error
		wantErr bool
	}{
		{
			name: "concrete type parameter succeeds",
			regFunc: func(u *structlua.Unmarshaler) error {
				return u.RegisterHandler[int](func(v structlua.Value) (int, error) { return 0, nil })
			},
			wantErr: false,
		},
		{
			name: "nil handler",
			regFunc: func(u *structlua.Unmarshaler) error {
				return u.RegisterHandler[shape](nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range registrationTests {
		t.Run(tt.name, func(t *testing.T) {
			u := structlua.NewUnmarshaler()
			err := tt.regFunc(u)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RegisterHandler() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewValueAndTypeName(t *testing.T) {
	tests := []struct {
		name     string
		val      structlua.Value
		wantType string
		wantAny  any
	}{
		{
			name:     "nil value",
			val:      structlua.Value{},
			wantType: "nil",
			wantAny:  nil,
		},
		{
			name:     "bool value",
			val:      structlua.NewValue(true),
			wantType: "boolean",
			wantAny:  true,
		},
		{
			name:     "number value",
			val:      structlua.NewValue(3.14),
			wantType: "number",
			wantAny:  3.14,
		},
		{
			name:     "string value",
			val:      structlua.NewValue("hello"),
			wantType: "string",
			wantAny:  "hello",
		},
		{
			name: "table value",
			val: structlua.NewValue(structlua.Table(func(yield func(k, v structlua.Value) bool) {
			})),
			wantType: "table",
		},
		{
			name:     "unsupported value",
			val:      structlua.NewValue(structlua.UnsupportedType("function")),
			wantType: "function",
			wantAny:  structlua.UnsupportedType("function"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := structlua.TypeName(tt.val); got != tt.wantType {
				t.Errorf("TypeName() = %q, want %q", got, tt.wantType)
			}
			if tt.wantType != "table" {
				if got := tt.val.Any(); got != tt.wantAny {
					t.Errorf("Any() = %v, want %v", got, tt.wantAny)
				}
			}
		})
	}
}

func TestDirectUnmarshaler(t *testing.T) {
	type Config struct {
		Name  string `lua:"name"`
		Count int    `lua:"count"`
	}

	mockTable := structlua.Table(func(yield func(k, v structlua.Value) bool) {
		if !yield(structlua.NewValue("name"), structlua.NewValue("direct")) {
			return
		}
		yield(structlua.NewValue("count"), structlua.NewValue(float64(42)))
	})

	var cfg Config
	err := structlua.Unmarshal(structlua.NewValue(mockTable), &cfg)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	want := Config{Name: "direct", Count: 42}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestEmptyStackError(t *testing.T) {
	var cfg struct{}
	err := structlua.Unmarshal(structlua.NewValue(structlua.UnsupportedType("none")), &cfg)
	if err == nil || err.Error() != "Lua stack is empty" {
		t.Fatalf("expected 'Lua stack is empty' error, got %v", err)
	}
}
