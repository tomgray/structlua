# structlua

`structlua` is a lightweight Go library for unmarshaling Lua tables and values into strongly-typed Go structs, slices, maps, primitives, and interfaces.

## Why structlua?

Lua is a great choice for a configuration language for Go applications. However, extracting nested configuration tables out of a Lua VM stack into Go types typically requires a bunch of stack operations, type checks and other boilerplate code.

`structlua` solves this by providing familiar `encoding/json`-style unmarshaling directly from Lua values into Go types:

- **Ergonomic Struct Mapping**: Map fields using `lua:"..."` struct tags or rely on automatic exact and case-insensitive field name matching.
- **First-Class Polymorphism**: Unmarshal Lua tables into Go interfaces based on a discriminator field (e.g. `type = "webhook"`), with support for both direct concrete types and DTO-to-domain constructor functions.
- **Extensible Type Handlers**: Implement `LuaUnmarshaler` on your own types, or register custom handlers for standard library and third-party types (like `time.Time` or `time.Duration`).
- **Strict Type & Bounds Checking**: Validates contiguous 1-based Lua arrays when targeting Go slices/arrays, checks integer and float overflow bounds (`int8`, `uint8`, `float32`, etc.), and rejects negative values for unsigned integers.
- **Zero-Dependency Core & Pluggable Lua VMs**: The core `github.com/tomgray/structlua` package relies solely on the Go standard library. Lightweight adapter submodules are provided for [`github.com/yuin/gopher-lua`](https://github.com/yuin/gopher-lua) and [`github.com/Shopify/go-lua`](https://github.com/Shopify/go-lua), so your project only imports the Lua VM it actually uses.

---

## Installation

Choose the adapter matching your Lua VM library:

**Using [`yuin/gopher-lua`](https://github.com/yuin/gopher-lua):**
```bash
go get github.com/tomgray/structlua github.com/tomgray/structlua/gopherlua
```

**Using [`Shopify/go-lua`](https://github.com/Shopify/go-lua):**
```bash
go get github.com/tomgray/structlua github.com/tomgray/structlua/golua
```

---

## Quick Start

### With `yuin/gopher-lua`

```go
package main

import (
	"fmt"
	"log"

	lua "github.com/yuin/gopher-lua"
	"github.com/tomgray/structlua"
	"github.com/tomgray/structlua/gopherlua"
)

type ServerConfig struct {
	Host    string   `lua:"host"`
	Port    int      `lua:"port"`
	TLS     bool     `lua:"tls"`
	Allowed []string `lua:"allowed_hosts"`
}

func main() {
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
```

### With `Shopify/go-lua`

```go
package main

import (
	"fmt"
	"log"

	"github.com/Shopify/go-lua"
	"github.com/tomgray/structlua"
	"github.com/tomgray/structlua/golua"
)

type ServerConfig struct {
	Host    string   `lua:"host"`
	Port    int      `lua:"port"`
	TLS     bool     `lua:"tls"`
	Allowed []string `lua:"allowed_hosts"`
}

func main() {
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
	// golua.NewValue wraps the value at the top of the stack (index -1)
	if err := structlua.Unmarshal(golua.NewValue(l), &cfg); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", cfg)
	// Output: {Host:0.0.0.0 Port:8080 TLS:true Allowed:[example.com api.example.com]}
}
```

---

## User Guide

### Struct Tags & Field Matching

`structlua` inspects exported struct fields and matches them against string keys in a Lua table:

```go
type Config struct {
	// Explicit key mapping via the `lua` tag
	ListenAddr string `lua:"listen_addr"`

	// Untagged fields match "Timeout" (exact) or "timeout" / "TIMEOUT" (case-insensitive)
	Timeout int

	// Ignored field
	Internal string `lua:"-"`
}
```

- **Exact vs. Case-Insensitive Precedence**: Exact key matches always take precedence. A case-insensitive match will never overwrite a field that was already populated by an exact match.
- **Unmatched Keys & Missing Fields**: Extra keys in a Lua table that do not match any struct field are silently ignored. Missing or `nil` keys in Lua leave the corresponding Go struct field at its existing value.

### Supported Types

| Lua Type         | Supported Go Target Types                                                      | Notes                                                                                                                    |
| :--------------- | :----------------------------------------------------------------------------- | :----------------------------------------------------------------------------------------------------------------------- |
| `string`         | `string`, `*string`, `any`                                                     |                                                                                                                          |
| `boolean`        | `bool`, `*bool`, `any`                                                         |                                                                                                                          |
| `number`         | `int`, `int8`..`int64`, `uint`, `uint8`..`uint64`, `float32`, `float64`, `any` | Verifies integer/float range and rejects negative numbers for `uint*`. Unmarshals as `float64` into `any`.               |
| `table` (array)  | `[]T`, `[N]T`, `any`                                                           | Must have contiguous integer keys starting at `1` (`1..N`). Unmarshals non-empty arrays as `[]any` when targeting `any`. |
| `table` (object) | `struct`, `map[string]T`, `any`                                                | Table keys must be strings. Empty tables `{}` unmarshal into `[]T` as `[]T{}` and into `any` as `map[string]any{}`.      |
| `nil`            | `*T`, `any`                                                                    | Sets pointer fields to `nil`.                                                                                            |

### Custom Unmarshaling with `LuaUnmarshaler`

Types you own can implement the `structlua.LuaUnmarshaler` interface to control how they are decoded from a `structlua.Value`:

```go
type HexColor struct {
	R, G, B int
}

func (c *HexColor) UnmarshalLua(v structlua.Value) error {
	s, ok := v.Any().(string)
	if !ok {
		return fmt.Errorf("expected hex color string, got %s", structlua.TypeName(v))
	}
	_, err := fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	return err
}
```

### Custom Type Handlers (`WithHandler` & `RegisterHandler`)

For types you do not own (such as standard library types like `time.Time`) or custom interfaces, register a handler on an `Unmarshaler` instance:

```go
timeHandler := func(v structlua.Value) (time.Time, error) {
	s, ok := v.Any().(string)
	if !ok {
		return time.Time{}, fmt.Errorf("expected RFC3339 string, got %s", structlua.TypeName(v))
	}
	return time.Parse(time.RFC3339, s)
}

u := structlua.NewUnmarshaler(
	structlua.WithHandler[time.Time](timeHandler),
)

type Event struct {
	Name      string     `lua:"name"`
	CreatedAt time.Time  `lua:"created_at"`
	UpdatedAt *time.Time `lua:"updated_at"` // Automatically works for pointer fields too
}

var ev Event
err := u.Unmarshal(val, &ev)
```

### Polymorphic Interfaces (`Polymorphic`)

When a struct or slice contains a Go interface (for example, a list of shapes, notification channels, or pipeline stages), use `structlua.Polymorphic[T](discriminatorField)` to dispatch based on a string field in the Lua table.

#### 1. Direct Concrete Types (`AddType`)

Use `AddType` when your concrete Go struct implements the target interface directly and its fields map 1:1 to the Lua table:

```go
type Shape interface {
	Area() float64
}

type Circle struct {
	Radius float64 `lua:"radius"`
}

func (c *Circle) Area() float64 { return 3.14159 * c.Radius * c.Radius }

type Square struct {
	Side float64 `lua:"side"`
}

func (s *Square) Area() float64 { return s.Side * s.Side }

u := structlua.NewUnmarshaler(
	structlua.Polymorphic[Shape]("type").
		AddType[*Circle]("circle").
		AddType[*Square]("square"),
)
```

#### 2. DTO-to-Domain Mapping Functions (`Add`)

Domain objects often have unexported fields, require constructor validation, or do not match the raw configuration schema 1:1. `Add` unmarshals the Lua table into a plain configuration struct `C` and passes it to your constructor/mapping function `func(C) (T, error)`:

```go
type CircleConfig struct {
	Radius float64 `lua:"radius"`
}

u := structlua.NewUnmarshaler(
	structlua.Polymorphic[Shape]("type").
		Add("circle", func(cfg CircleConfig) (Shape, error) {
			if cfg.Radius <= 0 {
				return nil, fmt.Errorf("radius must be positive, got %v", cfg.Radius)
			}
			return NewCircle(cfg.Radius), nil
		}).
		AddType[*Square]("square"),
)
```

With this unmarshaler, Lua tables such as:

```lua
return {
	shapes = {
		{ type = "circle", radius = 5.0 },
		{ type = "square", side = 4.0 },
	}
}
```

unmarshal directly into `[]Shape`.

---

## Development & Testing

This repository uses Nix Flakes and a Go multi-module workspace:

- **Run all tests**:
  ```bash
  nix run .#test
  ```
- **Format Go and Nix files**:
  ```bash
  nix fmt
  ```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).
