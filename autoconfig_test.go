package autoconfig

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var JSONConfigs = []string{
	`{
    "var1": "value1",
    "hash1": {
      "hash1var1": "blah",
      "hash2": {
        "hash2var1": ["foo", "bar"]
      },
      "intval1": 15
    }
  }`,
	`{
    "var1": "value2",
    "hash1": {
      "hash2": {
        "hash2var1": ["bar", "baz"]
      },
      "intval1": 15
    }
	}`,
	`{
    "var1": "value1",
    "hash1": {
      "hash1var1": "blah",
      "hash2": {
        "hash2var1": ["foo", "bar"]
      },
      "intval1": 16
    }
  }`,
}

const validYAMLConfig = `
var1: value1
hash1:
  hash1var1: blah
  hash2:
    hash2var1:
      - "foo"
      - "bar"
    hash2var2: ["foo", "bar"]
    hashlist:
      - key: value
      - key: value2
  intval1: 15
  floatval: 15.5
`

func TestMain(m *testing.M) {
	Fs = afero.NewMemMapFs()
	os.Exit(m.Run())
}

func loadJSONConfig() *Config {
	afero.WriteFile(Fs, "test.config", []byte(JSONConfigs[0]), 0644)
	c, err := Load(context.Background(), "test.config")
	if err != nil {
		log.Println(err)
		return nil
	}
	return c
}

func loadYAMLConfig() *Config {
	afero.WriteFile(Fs, "test.config", []byte(validYAMLConfig), 0644)
	c, err := Load(context.Background(), "test.config")
	if err != nil {
		log.Println(err)
		return nil
	}
	return c
}

func TestInvalidConfig(t *testing.T) {
	afero.WriteFile(Fs, "test.config", []byte("blahblah"), 0644)
	c, err := Load(context.Background(), "test.config")
	assert.Nil(t, c)
	assert.NotNil(t, err)
}

func TestLoadJSON(t *testing.T) {
	afero.WriteFile(Fs, "test.config", []byte(JSONConfigs[0]), 0644)
	c, err := Load(context.Background(), "test.config")
	assert.Nil(t, err)
	assert.NotNil(t, c)
}

func TestGetJSON(t *testing.T) {
	c := loadJSONConfig()
	require.NotNil(t, c)
	assert.Equal(t, "value1", c.Get("var1"))
	assert.Equal(t, "blah", c.Get("hash1.hash1var1"))
}

func TestGetYAML(t *testing.T) {
	c := loadYAMLConfig()
	require.NotNil(t, c)
	assert.Equal(t, "value1", c.Get("var1"))
	assert.Equal(t, "blah", c.Get("hash1.hash1var1"))
}

func TestGetIntJSON(t *testing.T) {
	c := loadJSONConfig()
	require.NotNil(t, c)
	assert.Equal(t, 15, c.GetInt("hash1.intval1"))
}

func TestGetIntYAML(t *testing.T) {
	c := loadYAMLConfig()
	require.NotNil(t, c)
	assert.Equal(t, 15, c.GetInt("hash1.intval1"))
}

func TestGetFloatJSON(t *testing.T) {
	t.Skip("Floats aren't supported in JSON")
}

func TestGetFloatYAML(t *testing.T) {
	c := loadYAMLConfig()
	require.NotNil(t, c)
	assert.Equal(t, 15.5, c.GetFloat("hash1.floatval"))
}

func TestGetAllJSON(t *testing.T) {
	c := loadJSONConfig()
	require.NotNil(t, c)
	assert.Equal(t, []string{"foo", "bar"}, c.GetAll("hash1.hash2.hash2var1"))

	c = loadYAMLConfig()
	require.NotNil(t, c)
	assert.Equal(t, []string{"foo", "bar"}, c.GetAll("hash1.hash2.hash2var1"))
}

func TestGetAllYAML(t *testing.T) {
	c := loadYAMLConfig()
	require.NotNil(t, c)
	assert.Equal(t, []string{"foo", "bar"}, c.GetAll("hash1.hash2.hash2var1"))
}

func TestValidator(t *testing.T) {
	c := loadJSONConfig()
	require.NotNil(t, c)

	c.AddValidator(func(old, new *Config) error {
		l := new.GetAll("hash1.hash2.hash2var1")
		if l[0] != "foo" {
			return errors.New("Invalid list entry")
		}
		return nil
	})

	// Second read is the same config, it should still be valid.
	afero.WriteFile(Fs, "test.config", []byte(JSONConfigs[0]), 0644)
	assert.Nil(t, c.read())

	// Third read is an invalid config, validation should fail and the original config will be loaded.
	afero.WriteFile(Fs, "test.config", []byte(JSONConfigs[1]), 0644)
	assert.NotNil(t, c.read())
}

func TestImmutable(t *testing.T) {
	c := loadJSONConfig()
	require.NotNil(t, c)

	// Second config changes var1 which is marked as immutable.
	c.Immutable("var1")
	afero.WriteFile(Fs, "test.config", []byte(JSONConfigs[1]), 0644)
	assert.NotNil(t, c.read())

	// Third config changes hash1.intval1 which is marked as immutable.
	c.Immutable("hash1.intval1")
	afero.WriteFile(Fs, "test.config", []byte(JSONConfigs[2]), 0644)
	assert.NotNil(t, c.read())
}

func TestRequiredOnInitialLoad(t *testing.T) {
	c := loadJSONConfig()
	require.NotNil(t, c)
	assert.NotPanics(t, func() { c.Required("hash1.intval1") })
	assert.Panics(t, func() { c.Required("hash1.intval2") })
}

func TestRequiredOnUpdate(t *testing.T) {
	c := loadJSONConfig()
	require.NotNil(t, c)
	assert.NotPanics(t, func() { c.Required("hash1.hash1var1") })

	// Second config is missing hash1.hash1var1 so the reload should fail.
	afero.WriteFile(Fs, "test.config", []byte(JSONConfigs[1]), 0644)
	assert.NotNil(t, c.read())
}
