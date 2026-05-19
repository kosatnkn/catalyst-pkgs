package config

import (
	"sort"
	"testing"
)

// TestKeysOfStructSquash verifies that keysOfStruct understands the
// `mapstructure` tag options: a squashed sub-struct contributes its fields to
// the parent prefix, a `-` name is ignored, and trailing options on a named
// field do not leak into the key.
func TestKeysOfStructSquash(t *testing.T) {
	type credentials struct {
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
	}

	type migration struct {
		Credentials credentials `mapstructure:",squash"`
		Schema      string      `mapstructure:"schema"`
	}

	type database struct {
		Host       string    `mapstructure:"host"`
		Migrations migration `mapstructure:"migrations"`
		Internal   string    `mapstructure:"-"`
		Name       string    `mapstructure:"name,omitempty"`
	}

	type root struct {
		DB database `mapstructure:"db"`
	}

	got := keysOfStruct(root{}, "")
	sort.Strings(got)

	want := []string{
		"db.host",
		"db.migrations.password",
		"db.migrations.schema",
		"db.migrations.user",
		"db.name",
	}

	if len(got) != len(want) {
		t.Fatalf("keysOfStruct() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keysOfStruct() = %v, want %v", got, want)
		}
	}
}
