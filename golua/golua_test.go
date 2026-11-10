package golua_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/Shopify/go-lua"
	"github.com/tomgray/structlua"
	"github.com/tomgray/structlua/golua"
	"github.com/tomgray/structlua/structluatest"
)

func ExampleNewValue() {
	type ServerConfig struct {
		Host    string   `lua:"host"`
		Port    int      `lua:"port"`
		TLS     bool     `lua:"tls"`
		Allowed []string `lua:"allowed_hosts"`
	}

	l := lua.NewState()
	lua.OpenLibraries(l)

	script := `
		return {
			host = "0.0.0.0",
			port = 8080,
			tls = true,
			allowed_hosts = { "example.com", "api.example.com" },
		}
	`
	if err := lua.DoString(l, script); err != nil {
		log.Fatal(err)
	}

	var cfg ServerConfig
	if err := structlua.Unmarshal(golua.NewValue(l), &cfg); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", cfg)
	// Output: {Host:0.0.0.0 Port:8080 TLS:true Allowed:[example.com api.example.com]}
}

func evalLua(t *testing.T, code string) structlua.Value {
	t.Helper()
	l := lua.NewState()
	lua.OpenLibraries(l)
	if err := lua.DoString(l, code); err != nil {
		t.Fatalf("failed to execute Lua code: %v", err)
	}
	return golua.NewValue(l)
}

func TestConformance(t *testing.T) {
	structluatest.Run(t, evalLua)
}

func TestNewValueEmptyStack(t *testing.T) {
	l := lua.NewState()
	v := golua.NewValue(l)
	if got := structlua.TypeName(v); got != "none" {
		t.Errorf("TypeName for empty stack = %q, want 'none'", got)
	}

	var cfg struct{}
	err := structlua.Unmarshal(v, &cfg)
	if err == nil || err.Error() != "Lua stack is empty" {
		t.Fatalf("expected 'Lua stack is empty' error, got %v", err)
	}
}

func TestNewValueNilState(t *testing.T) {
	v := golua.NewValue(nil)
	if got := structlua.TypeName(v); got != "none" {
		t.Errorf("TypeName for nil state = %q, want 'none'", got)
	}
}
