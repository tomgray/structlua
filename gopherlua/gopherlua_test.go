package gopherlua_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/tomgray/structlua"
	"github.com/tomgray/structlua/gopherlua"
	"github.com/tomgray/structlua/structluatest"
	lua "github.com/yuin/gopher-lua"
)

func ExampleNewValue() {
	type ServerConfig struct {
		Host    string   `lua:"host"`
		Port    int      `lua:"port"`
		TLS     bool     `lua:"tls"`
		Allowed []string `lua:"allowed_hosts"`
	}

	L := lua.NewState()
	defer L.Close()

	script := `
		return {
			host = "0.0.0.0",
			port = 8080,
			tls = true,
			allowed_hosts = { "example.com", "api.example.com" },
		}
	`
	if err := L.DoString(script); err != nil {
		log.Fatal(err)
	}

	var cfg ServerConfig
	val := gopherlua.NewValue(L.Get(-1))
	if err := structlua.Unmarshal(val, &cfg); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", cfg)
	// Output: {Host:0.0.0.0 Port:8080 TLS:true Allowed:[example.com api.example.com]}
}

func evalLua(t *testing.T, code string) structlua.Value {
	t.Helper()
	L := lua.NewState()
	defer L.Close()
	if err := L.DoString(code); err != nil {
		t.Fatalf("failed to execute Lua code: %v", err)
	}
	top := L.GetTop()
	if top == 0 {
		return structlua.NewValue(structlua.UnsupportedType("none"))
	}
	return gopherlua.NewValue(L.Get(top))
}

func TestConformance(t *testing.T) {
	structluatest.Run(t, evalLua)
}

func TestNewValueNil(t *testing.T) {
	v := gopherlua.NewValue(nil)
	if got := structlua.TypeName(v); got != "nil" {
		t.Errorf("TypeName for nil = %q, want 'nil'", got)
	}
	if v.Any() != nil {
		t.Errorf("Any for nil = %v, want nil", v.Any())
	}
}

func TestNewValueLNil(t *testing.T) {
	v := gopherlua.NewValue(lua.LNil)
	if got := structlua.TypeName(v); got != "nil" {
		t.Errorf("TypeName for LNil = %q, want 'nil'", got)
	}
	if v.Any() != nil {
		t.Errorf("Any for LNil = %v, want nil", v.Any())
	}
}
