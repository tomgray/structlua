package structluatest

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tomgray/structlua"
)

func ptr[T any](v T) *T {
	return &v
}

type hexColour struct {
	R, G, B int
}

func (c *hexColour) UnmarshalLua(v structlua.Value) error {
	s, ok := v.Any().(string)
	if !ok {
		return fmt.Errorf("expected hex colour string, got %s", structlua.TypeName(v))
	}
	_, err := fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	return err
}

type shape interface {
	Area() float64
}

type circle struct {
	Radius float64 `lua:"radius"`
}

func (c *circle) Area() float64 {
	return 3.14159 * c.Radius * c.Radius
}

type square struct {
	Side float64 `lua:"side"`
}

func (s *square) Area() float64 {
	return s.Side * s.Side
}

type rawCircleConfig struct {
	Radius float64 `lua:"radius"`
}

// Run executes the complete structlua conformance test suite using the provided
// eval function, which evaluates Lua code and returns the result as a structlua.Value.
func Run(t *testing.T, eval func(t *testing.T, luaCode string) structlua.Value) {
	t.Run("Unmarshaler", func(t *testing.T) {
		runTestUnmarshaler(t, eval)
	})
	t.Run("InterfaceHandlers", func(t *testing.T) {
		runTestInterfaceHandlers(t, eval)
	})
	t.Run("ConcreteHandlers", func(t *testing.T) {
		runTestConcreteHandlers(t, eval)
	})
	t.Run("OverrideAnyHandler", func(t *testing.T) {
		runTestOverrideAnyHandler(t, eval)
	})
	t.Run("ZeroValueUnmarshaler", func(t *testing.T) {
		runTestZeroValueUnmarshaler(t, eval)
	})
	t.Run("PolymorphicBuilder", func(t *testing.T) {
		runTestPolymorphicBuilder(t, eval)
	})
}

func runTestUnmarshaler(t *testing.T, eval func(t *testing.T, luaCode string) structlua.Value) {
	type BasicConfig struct {
		Name   string `lua:"name"`
		Answer int    `lua:"answer"`
		Active bool   `lua:"active"`
	}

	tests := []struct {
		name    string
		luaCode string
		target  any
		want    any
		wantErr bool
	}{
		{
			name: "basic table with primitive fields",
			luaCode: `
				return {
					name = "Arthur Dent",
					answer = 42,
					active = true
				}
			`,
			target: &BasicConfig{},
			want: &BasicConfig{
				Name:   "Arthur Dent",
				Answer: 42,
				Active: true,
			},
			wantErr: false,
		},
		{
			name: "empty table",
			luaCode: `
				return {}
			`,
			target:  &BasicConfig{},
			want:    &BasicConfig{},
			wantErr: false,
		},
		{
			name: "nested struct and ignored field",
			luaCode: `
				return {
					title = "Deep Thought",
					ignored = "should not be set",
					details = {
						owner = "Slartibartfast",
						retries = 3
					}
				}
			`,
			target: &struct {
				Title   string `lua:"title"`
				Ignored string `lua:"-"`
				Details struct {
					Owner   string `lua:"owner"`
					Retries int    `lua:"retries"`
				} `lua:"details"`
			}{},
			want: &struct {
				Title   string `lua:"title"`
				Ignored string `lua:"-"`
				Details struct {
					Owner   string `lua:"owner"`
					Retries int    `lua:"retries"`
				} `lua:"details"`
			}{
				Title: "Deep Thought",
				Details: struct {
					Owner   string `lua:"owner"`
					Retries int    `lua:"retries"`
				}{
					Owner:   "Slartibartfast",
					Retries: 3,
				},
			},
			wantErr: false,
		},
		{
			name: "non-table on stack",
			luaCode: `
				return "not a table"
			`,
			target:  &BasicConfig{},
			wantErr: true,
		},
		{
			name: "type mismatch: string field with number",
			luaCode: `
				return {
					name = 123,
					answer = 42,
					active = true
				}
			`,
			target:  &BasicConfig{},
			wantErr: true,
		},
		{
			name: "type mismatch: string field with boolean",
			luaCode: `
				return {
					name = true,
					answer = 42,
					active = true
				}
			`,
			target:  &BasicConfig{},
			wantErr: true,
		},
		{
			name: "type mismatch: int field with string",
			luaCode: `
				return {
					name = "Arthur Dent",
					answer = "forty-two",
					active = true
				}
			`,
			target:  &BasicConfig{},
			wantErr: true,
		},
		{
			name: "type mismatch: int field with boolean",
			luaCode: `
				return {
					name = "Arthur Dent",
					answer = false,
					active = true
				}
			`,
			target:  &BasicConfig{},
			wantErr: true,
		},
		{
			name: "type mismatch: bool field with string",
			luaCode: `
				return {
					name = "Arthur Dent",
					answer = 42,
					active = "true"
				}
			`,
			target:  &BasicConfig{},
			wantErr: true,
		},
		{
			name: "type mismatch: bool field with number",
			luaCode: `
				return {
					name = "Arthur Dent",
					answer = 42,
					active = 1
				}
			`,
			target:  &BasicConfig{},
			wantErr: true,
		},
		{
			name: "type mismatch: field with nested table",
			luaCode: `
				return {
					name = { first = "Arthur" },
					answer = 42,
					active = true
				}
			`,
			target:  &BasicConfig{},
			wantErr: true,
		},
		{
			name: "invalid target: non-pointer",
			luaCode: `
				return { name = "test" }
			`,
			target:  BasicConfig{},
			wantErr: true,
		},
		{
			name: "invalid target: nil pointer",
			luaCode: `
				return { name = "test" }
			`,
			target:  (*BasicConfig)(nil),
			wantErr: true,
		},
		{
			name: "invalid target: pointer to non-struct",
			luaCode: `
				return { name = "test" }
			`,
			target:  new(int),
			wantErr: true,
		},
		{
			name: "valid uint fields",
			luaCode: `
				return {
					u = 10,
					u8 = 250,
					u16 = 65000,
					u32 = 4000000000,
					u64 = 10000000000
				}
			`,
			target: &struct {
				U   uint   `lua:"u"`
				U8  uint8  `lua:"u8"`
				U16 uint16 `lua:"u16"`
				U32 uint32 `lua:"u32"`
				U64 uint64 `lua:"u64"`
			}{},
			want: &struct {
				U   uint   `lua:"u"`
				U8  uint8  `lua:"u8"`
				U16 uint16 `lua:"u16"`
				U32 uint32 `lua:"u32"`
				U64 uint64 `lua:"u64"`
			}{
				U:   10,
				U8:  250,
				U16: 65000,
				U32: 4000000000,
				U64: 10000000000,
			},
			wantErr: false,
		},
		{
			name: "negative number into uint field",
			luaCode: `
				return { val = -1 }
			`,
			target: &struct {
				Val uint `lua:"val"`
			}{},
			wantErr: true,
		},
		{
			name: "negative number into uint8 field",
			luaCode: `
				return { val = -1 }
			`,
			target: &struct {
				Val uint8 `lua:"val"`
			}{},
			wantErr: true,
		},
		{
			name: "overflow uint8 field",
			luaCode: `
				return { val = 256 }
			`,
			target: &struct {
				Val uint8 `lua:"val"`
			}{},
			wantErr: true,
		},
		{
			name: "overflow int8 field positive",
			luaCode: `
				return { val = 128 }
			`,
			target: &struct {
				Val int8 `lua:"val"`
			}{},
			wantErr: true,
		},
		{
			name: "overflow int8 field negative",
			luaCode: `
				return { val = -129 }
			`,
			target: &struct {
				Val int8 `lua:"val"`
			}{},
			wantErr: true,
		},
		{
			name: "untagged struct: exact case match",
			luaCode: `
				return {
					Name = "Ford Prefect",
					Age = 40,
					IsAdmin = true
				}
			`,
			target: &struct {
				Name    string
				Age     int
				IsAdmin bool
			}{},
			want: &struct {
				Name    string
				Age     int
				IsAdmin bool
			}{
				Name:    "Ford Prefect",
				Age:     40,
				IsAdmin: true,
			},
			wantErr: false,
		},
		{
			name: "untagged struct: lowercase key match",
			luaCode: `
				return {
					name = "Ford Prefect",
					age = 40,
					isadmin = true
				}
			`,
			target: &struct {
				Name    string
				Age     int
				IsAdmin bool
			}{},
			want: &struct {
				Name    string
				Age     int
				IsAdmin bool
			}{
				Name:    "Ford Prefect",
				Age:     40,
				IsAdmin: true,
			},
			wantErr: false,
		},
		{
			name: "untagged struct: uppercase key match",
			luaCode: `
				return {
					NAME = "Ford Prefect",
					AGE = 40,
					ISADMIN = true
				}
			`,
			target: &struct {
				Name    string
				Age     int
				IsAdmin bool
			}{},
			want: &struct {
				Name    string
				Age     int
				IsAdmin bool
			}{
				Name:    "Ford Prefect",
				Age:     40,
				IsAdmin: true,
			},
			wantErr: false,
		},
		{
			name: "exact case preferred over case-insensitive",
			luaCode: `
				return {
					name = "lowercase",
					NAME = "uppercase"
				}
			`,
			target: &struct {
				Name string `lua:"name"`
				NAME string `lua:"NAME"`
			}{},
			want: &struct {
				Name string `lua:"name"`
				NAME string `lua:"NAME"`
			}{
				Name: "lowercase",
				NAME: "uppercase",
			},
			wantErr: false,
		},
		{
			name: "untagged nested struct",
			luaCode: `
				return {
					Title = "Guide",
					Meta = {
						Author = "Adams",
						Edition = 2
					}
				}
			`,
			target: &struct {
				Title string
				Meta  struct {
					Author  string
					Edition int
				}
			}{},
			want: &struct {
				Title string
				Meta  struct {
					Author  string
					Edition int
				}
			}{
				Title: "Guide",
				Meta: struct {
					Author  string
					Edition int
				}{
					Author:  "Adams",
					Edition: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "slice of strings",
			luaCode: `
				return {
					tags = { "golang", "lua", "nix" }
				}
			`,
			target: &struct {
				Tags []string `lua:"tags"`
			}{},
			want: &struct {
				Tags []string `lua:"tags"`
			}{
				Tags: []string{"golang", "lua", "nix"},
			},
			wantErr: false,
		},
		{
			name: "slice of integers",
			luaCode: `
				return {
					scores = { 100, 95, 88 }
				}
			`,
			target: &struct {
				Scores []int `lua:"scores"`
			}{},
			want: &struct {
				Scores []int `lua:"scores"`
			}{
				Scores: []int{100, 95, 88},
			},
			wantErr: false,
		},
		{
			name: "slice of structs",
			luaCode: `
				return {
					items = {
						{ name = "Widget", price = 10 },
						{ name = "Gadget", price = 25 }
					}
				}
			`,
			target: &struct {
				Items []struct {
					Name  string `lua:"name"`
					Price int    `lua:"price"`
				} `lua:"items"`
			}{},
			want: &struct {
				Items []struct {
					Name  string `lua:"name"`
					Price int    `lua:"price"`
				} `lua:"items"`
			}{
				Items: []struct {
					Name  string `lua:"name"`
					Price int    `lua:"price"`
				}{
					{Name: "Widget", Price: 10},
					{Name: "Gadget", Price: 25},
				},
			},
			wantErr: false,
		},
		{
			name: "fixed-size array",
			luaCode: `
				return {
					coords = { 10, 20, 30 }
				}
			`,
			target: &struct {
				Coords [3]int `lua:"coords"`
			}{},
			want: &struct {
				Coords [3]int `lua:"coords"`
			}{
				Coords: [3]int{10, 20, 30},
			},
			wantErr: false,
		},
		{
			name: "top-level slice",
			luaCode: `
				return { "alpha", "beta", "gamma" }
			`,
			target:  &[]string{},
			want:    &[]string{"alpha", "beta", "gamma"},
			wantErr: false,
		},
		{
			name: "empty array into slice",
			luaCode: `
				return {
					tags = {}
				}
			`,
			target: &struct {
				Tags []string `lua:"tags"`
			}{},
			want: &struct {
				Tags []string `lua:"tags"`
			}{
				Tags: []string{},
			},
			wantErr: false,
		},
		{
			name: "type mismatch: object table into slice field",
			luaCode: `
				return {
					tags = { key = "not-an-array" }
				}
			`,
			target: &struct {
				Tags []string `lua:"tags"`
			}{},
			wantErr: true,
		},
		{
			name: "type mismatch: non-contiguous array table into slice field",
			luaCode: `
				return {
					tags = { [1] = "first", [3] = "third" }
				}
			`,
			target: &struct {
				Tags []string `lua:"tags"`
			}{},
			wantErr: true,
		},
		{
			name: "type mismatch: array element type mismatch",
			luaCode: `
				return {
					scores = { 10, "twenty", 30 }
				}
			`,
			target: &struct {
				Scores []int `lua:"scores"`
			}{},
			wantErr: true,
		},
		{
			name: "type mismatch: array table into nested struct field",
			luaCode: `
				return {
					config = { "unexpected", "array" }
				}
			`,
			target: &struct {
				Config struct {
					Name string `lua:"name"`
				} `lua:"config"`
			}{},
			wantErr: true,
		},
		{
			name:    "top-level string",
			luaCode: `return "hello world"`,
			target:  new(string),
			want:    ptr("hello world"),
			wantErr: false,
		},
		{
			name:    "top-level integer",
			luaCode: `return 42`,
			target:  new(int),
			want:    ptr(42),
			wantErr: false,
		},
		{
			name:    "top-level float",
			luaCode: `return 3.14`,
			target:  new(float64),
			want:    ptr(3.14),
			wantErr: false,
		},
		{
			name:    "top-level boolean",
			luaCode: `return true`,
			target:  new(bool),
			want:    ptr(true),
			wantErr: false,
		},
		{
			name:    "top-level type mismatch: string into int",
			luaCode: `return "not a number"`,
			target:  new(int),
			wantErr: true,
		},
		{
			name:    "invalid target: unsupported type",
			luaCode: `return 42`,
			target:  new(chan int),
			wantErr: true,
		},
		{
			name: "top-level map[string]any with primitive values",
			luaCode: `
				return {
					title = "Guide",
					version = 2,
					active = true
				}
			`,
			target: &map[string]any{},
			want: &map[string]any{
				"title":   "Guide",
				"version": float64(2),
				"active":  true,
			},
			wantErr: false,
		},
		{
			name: "top-level map[string]any with nested table and array",
			luaCode: `
				return {
					name = "Galaxy",
					tags = { "scifi", "comedy" },
					author = {
						first = "Douglas",
						last = "Adams"
					}
				}
			`,
			target: &map[string]any{},
			want: &map[string]any{
				"name": "Galaxy",
				"tags": []any{"scifi", "comedy"},
				"author": map[string]any{
					"first": "Douglas",
					"last":  "Adams",
				},
			},
			wantErr: false,
		},
		{
			name:    "empty table into map[string]any",
			luaCode: `return {}`,
			target:  &map[string]any{},
			want:    &map[string]any{},
			wantErr: false,
		},
		{
			name: "struct with map[string]any field",
			luaCode: `
				return {
					id = 42,
					metadata = {
						env = "production",
						debug = false
					}
				}
			`,
			target: &struct {
				ID       int            `lua:"id"`
				Metadata map[string]any `lua:"metadata"`
			}{},
			want: &struct {
				ID       int            `lua:"id"`
				Metadata map[string]any `lua:"metadata"`
			}{
				ID: 42,
				Metadata: map[string]any{
					"env":   "production",
					"debug": false,
				},
			},
			wantErr: false,
		},
		{
			name: "typed map: map[string]string",
			luaCode: `
				return {
					env = "prod",
					region = "us-east"
				}
			`,
			target: &map[string]string{},
			want: &map[string]string{
				"env":    "prod",
				"region": "us-east",
			},
			wantErr: false,
		},
		{
			name: "type mismatch: non-string key into map[string]any",
			luaCode: `
				return {
					[1] = "numeric key"
				}
			`,
			target:  &map[string]any{},
			wantErr: true,
		},
		{
			name:    "type mismatch: non-table into map[string]any",
			luaCode: `return "not a table"`,
			target:  &map[string]any{},
			wantErr: true,
		},
		{
			name: "struct field implementing LuaUnmarshaler",
			luaCode: `
				return {
					fg = "#ff8800",
					bg = "#000000"
				}
			`,
			target: &struct {
				FG hexColour `lua:"fg"`
				BG hexColour `lua:"bg"`
			}{},
			want: &struct {
				FG hexColour `lua:"fg"`
				BG hexColour `lua:"bg"`
			}{
				FG: hexColour{R: 255, G: 136, B: 0},
				BG: hexColour{R: 0, G: 0, B: 0},
			},
			wantErr: false,
		},
		{
			name: "pointer field implementing LuaUnmarshaler",
			luaCode: `
				return {
					colour = "#112233"
				}
			`,
			target: &struct {
				Colour *hexColour `lua:"colour"`
			}{},
			want: &struct {
				Colour *hexColour `lua:"colour"`
			}{
				Colour: &hexColour{R: 17, G: 34, B: 51},
			},
			wantErr: false,
		},
		{
			name:    "top-level type implementing LuaUnmarshaler",
			luaCode: `return "#aabbcc"`,
			target:  &hexColour{},
			want:    &hexColour{R: 170, G: 187, B: 204},
			wantErr: false,
		},
		{
			name:    "custom unmarshaler error",
			luaCode: `return 12345`,
			target:  &hexColour{},
			wantErr: true,
		},
		{
			name: "interface field with concrete value implementing LuaUnmarshaler",
			luaCode: `
				return {
					colour = "#00ff00"
				}
			`,
			target: &struct {
				Colour structlua.LuaUnmarshaler `lua:"colour"`
			}{
				Colour: &hexColour{},
			},
			want: &struct {
				Colour structlua.LuaUnmarshaler `lua:"colour"`
			}{
				Colour: &hexColour{R: 0, G: 255, B: 0},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := eval(t, tt.luaCode)
			u := structlua.NewUnmarshaler()
			err := u.Unmarshal(val, tt.target)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.want != nil {
				if !reflect.DeepEqual(tt.target, tt.want) {
					t.Errorf("Unmarshal() target = %+v, want %+v", tt.target, tt.want)
				}
			}
		})
	}
}

func runTestInterfaceHandlers(t *testing.T, eval func(t *testing.T, luaCode string) structlua.Value) {
	type Doc struct {
		Shape shape `lua:"shape"`
	}
	type MultiDoc struct {
		Shapes []shape `lua:"shapes"`
	}

	newShapeUnmarshaler := func() *structlua.Unmarshaler {
		return structlua.NewUnmarshaler(
			structlua.Polymorphic[shape]("type").
				AddType[*circle]("circle").
				AddType[*square]("square"),
		)
	}

	tests := []struct {
		name        string
		luaCode     string
		unmarshaler *structlua.Unmarshaler
		target      any
		want        any
		wantErr     bool
	}{
		{
			name:    "struct field polymorphic circle",
			luaCode: `return { shape = { type = "circle", radius = 5.0 } }`,
			target:  &Doc{},
			want:    &Doc{Shape: &circle{Radius: 5.0}},
		},
		{
			name:    "struct field polymorphic square",
			luaCode: `return { shape = { type = "square", side = 4.0 } }`,
			target:  &Doc{},
			want:    &Doc{Shape: &square{Side: 4.0}},
		},
		{
			name:    "top-level interface",
			luaCode: `return { type = "circle", radius = 3.5 }`,
			target:  ptr[shape](nil),
			want:    ptr[shape](&circle{Radius: 3.5}),
		},
		{
			name: "slice of interfaces",
			luaCode: `return {
				shapes = {
					{ type = "circle", radius = 1.0 },
					{ type = "square", side = 2.0 }
				}
			}`,
			target: &MultiDoc{},
			want:   &MultiDoc{Shapes: []shape{&circle{Radius: 1.0}, &square{Side: 2.0}}},
		},
		{
			name:    "with_handler option",
			luaCode: `return { radius = 7.0 }`,
			unmarshaler: func() *structlua.Unmarshaler {
				var u *structlua.Unmarshaler
				u = structlua.NewUnmarshaler(
					structlua.WithHandler[shape](func(v structlua.Value) (shape, error) {
						var c circle
						if err := u.Unmarshal(v, &c); err != nil {
							return nil, err
						}
						return &c, nil
					}),
				)
				return u
			}(),
			target: ptr[shape](nil),
			want:   ptr[shape](&circle{Radius: 7.0}),
		},
		{
			name:    "handler error propagation",
			luaCode: `return { type = "triangle" }`,
			target:  ptr[shape](nil),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := eval(t, tt.luaCode)
			u := tt.unmarshaler
			if u == nil {
				u = newShapeUnmarshaler()
			}
			err := u.Unmarshal(val, tt.target)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.want != nil {
				if !reflect.DeepEqual(tt.target, tt.want) {
					t.Errorf("got %+v, want %+v", tt.target, tt.want)
				}
			}
		})
	}
}

func runTestConcreteHandlers(t *testing.T, eval func(t *testing.T, luaCode string) structlua.Value) {
	fixedTime, err := time.Parse(time.RFC3339, "2026-09-13T12:00:00Z")
	if err != nil {
		t.Fatalf("failed to parse fixed time: %v", err)
	}

	timeHandler := func(v structlua.Value) (time.Time, error) {
		s, ok := v.Any().(string)
		if !ok {
			return time.Time{}, fmt.Errorf("expected string for time, got %s", structlua.TypeName(v))
		}
		return time.Parse(time.RFC3339, s)
	}

	type Event struct {
		Name      string     `lua:"name"`
		CreatedAt time.Time  `lua:"created_at"`
		UpdatedAt *time.Time `lua:"updated_at"`
	}

	tests := []struct {
		name        string
		luaCode     string
		unmarshaler *structlua.Unmarshaler
		target      any
		want        any
		wantErr     bool
	}{
		{
			name: "concrete struct field and pointer field",
			luaCode: `return {
				name = "Deployment",
				created_at = "2026-09-13T12:00:00Z",
				updated_at = "2026-09-13T12:00:00Z"
			}`,
			unmarshaler: structlua.NewUnmarshaler(
				structlua.WithHandler[time.Time](timeHandler),
			),
			target: &Event{},
			want: &Event{
				Name:      "Deployment",
				CreatedAt: fixedTime,
				UpdatedAt: &fixedTime,
			},
		},
		{
			name:    "top-level concrete type",
			luaCode: `return "2026-09-13T12:00:00Z"`,
			unmarshaler: func() *structlua.Unmarshaler {
				u := structlua.NewUnmarshaler()
				_ = u.RegisterHandler[time.Time](timeHandler)
				return u
			}(),
			target: new(time.Time),
			want:   &fixedTime,
		},
		{
			name:    "concrete handler error propagation",
			luaCode: `return { created_at = "invalid-date" }`,
			unmarshaler: structlua.NewUnmarshaler(
				structlua.WithHandler[time.Time](timeHandler),
			),
			target:  &Event{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := eval(t, tt.luaCode)
			err := tt.unmarshaler.Unmarshal(val, tt.target)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.want != nil {
				if !reflect.DeepEqual(tt.target, tt.want) {
					t.Errorf("got %+v, want %+v", tt.target, tt.want)
				}
			}
		})
	}
}

func runTestOverrideAnyHandler(t *testing.T, eval func(t *testing.T, luaCode string) structlua.Value) {
	customAnyHandler := func(v structlua.Value) (any, error) {
		if f, ok := v.Any().(float64); ok {
			return int64(f), nil
		}
		return nil, fmt.Errorf("custom any handler only handles numbers")
	}

	u := structlua.NewUnmarshaler(
		structlua.WithHandler[any](customAnyHandler),
	)

	val := eval(t, `return 42`)
	var result any
	if err := u.Unmarshal(val, &result); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if result != int64(42) {
		t.Errorf("got %v (%T), want 42 (int64)", result, result)
	}
}

func runTestZeroValueUnmarshaler(t *testing.T, eval func(t *testing.T, luaCode string) structlua.Value) {
	u := &structlua.Unmarshaler{}
	val := eval(t, `return { hello = "world", count = 5 }`)

	var result map[string]any
	if err := u.Unmarshal(val, &result); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if result["hello"] != "world" || result["count"] != float64(5) {
		t.Errorf("unexpected result: %+v", result)
	}
}

func runTestPolymorphicBuilder(t *testing.T, eval func(t *testing.T, luaCode string) structlua.Value) {
	type nonShape struct {
		Name string `lua:"name"`
	}

	newBuilderUnmarshaler := func() *structlua.Unmarshaler {
		return structlua.NewUnmarshaler(
			structlua.Polymorphic[shape]("type").
				Add("circle", func(cfg rawCircleConfig) (shape, error) {
					if cfg.Radius <= 0 {
						return nil, fmt.Errorf("radius must be positive")
					}
					return &circle{Radius: cfg.Radius}, nil
				}).
				AddType[*square]("square").
				AddType[*nonShape]("non_shape"),
		)
	}

	tests := []struct {
		name        string
		luaCode     string
		unmarshaler *structlua.Unmarshaler
		want        shape
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name:    "circle via Add mapping function",
			luaCode: `return { type = "circle", radius = 6.0 }`,
			want:    &circle{Radius: 6.0},
		},
		{
			name:    "square via AddType",
			luaCode: `return { type = "square", side = 4.0 }`,
			want:    &square{Side: 4.0},
		},
		{
			name:       "mapping function error propagation",
			luaCode:    `return { type = "circle", radius = -1.0 }`,
			wantErr:    true,
			wantErrMsg: "radius must be positive",
		},
		{
			name:    "non-table Lua value",
			luaCode: `return "not-a-table"`,
			wantErr: true,
		},
		{
			name:    "missing discriminator field",
			luaCode: `return { radius = 5.0 }`,
			wantErr: true,
		},
		{
			name:    "non-string discriminator field",
			luaCode: `return { type = 123, radius = 5.0 }`,
			wantErr: true,
		},
		{
			name:    "unknown discriminator value",
			luaCode: `return { type = "triangle", radius = 5.0 }`,
			wantErr: true,
		},
		{
			name:       "type does not implement interface error",
			luaCode:    `return { type = "non_shape", name = "foo" }`,
			wantErr:    true,
			wantErrMsg: "does not implement target interface",
		},
		{
			name:    "empty discriminator field error",
			luaCode: `return { radius = 5.0 }`,
			unmarshaler: structlua.NewUnmarshaler(
				structlua.Polymorphic[shape]("").AddType[*circle]("circle"),
			),
			wantErr:    true,
			wantErrMsg: "discriminator field cannot be empty",
		},
	}

	defaultUnmarshaler := newBuilderUnmarshaler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := eval(t, tt.luaCode)
			u := tt.unmarshaler
			if u == nil {
				u = defaultUnmarshaler
			}

			var s shape
			err := u.Unmarshal(val, &s)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.wantErrMsg != "" {
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrMsg, err)
				}
			}
			if !tt.wantErr && tt.want != nil {
				if !reflect.DeepEqual(s, tt.want) {
					t.Errorf("got %+v, want %+v", s, tt.want)
				}
			}
		})
	}
}
