package autoconfig

import (
	"context"
	"log"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validJsonConfig = `
{
	"var1": "value1",
	"hash1": {
	  "hash1var1": "blah",
		"hash2": {
			"hash2var1": ["foo", "bar"]
		},
    "intval": 15
	}
}
`

const validYamlConfig = `
var1: value1
hash1:
  hash1var1: blah
  hash2:
    hash2var1:
      - "foo"
      - "bar"
  intval: 15
`

func loadJsonConfig() *Config {
	Fs = afero.NewMemMapFs()
	afero.WriteFile(Fs, "test.config", []byte(validJsonConfig), 0644)
	c, err := Load(context.Background(), "test.config")
	if err != nil {
		log.Println(err)
		return nil
	}
	return c
}

func loadYamlConfig() *Config {
	Fs = afero.NewMemMapFs()
	afero.WriteFile(Fs, "test.config", []byte(validYamlConfig), 0644)
	c, err := Load(context.Background(), "test.config")
	if err != nil {
		log.Println(err)
		return nil
	}
	return c
}

func TestInvalidConfig(t *testing.T) {
	Fs = afero.NewMemMapFs()
	afero.WriteFile(Fs, "test.config", []byte("blahblah"), 0644)
	c, err := Load(context.Background(), "test.config")
	assert.Nil(t, c)
	assert.NotNil(t, err)
}

func TestLoadJson(t *testing.T) {
	Fs = afero.NewMemMapFs()
	afero.WriteFile(Fs, "test.config", []byte(validJsonConfig), 0644)
	c, err := Load(context.Background(), "test.config")
	assert.Nil(t, err)
	assert.NotNil(t, c)
}

func TestGetJson(t *testing.T) {
	c := loadJsonConfig()
	require.NotNil(t, c)
	assert.Equal(t, "value1", c.Get("var1"))
	assert.Equal(t, "blah", c.Get("hash1.hash1var1"))
}

func TestGetYaml(t *testing.T) {
	c := loadYamlConfig()
	require.NotNil(t, c)
	assert.Equal(t, "value1", c.Get("var1"))
	assert.Equal(t, "blah", c.Get("hash1.hash1var1"))
}

func TestGetIntJson(t *testing.T) {
	c := loadJsonConfig()
	require.NotNil(t, c)
	assert.Equal(t, 15, c.GetInt("hash1.intval"))
}

func TestGetIntYaml(t *testing.T) {
	c := loadYamlConfig()
	require.NotNil(t, c)
	assert.Equal(t, 15, c.GetInt("hash1.intval"))
}

func TestGetAllJson(t *testing.T) {
	c := loadJsonConfig()
	require.NotNil(t, c)
	assert.Equal(t, []string{"foo", "bar"}, c.GetAll("hash1.hash2.hash2var1"))

	c = loadYamlConfig()
	require.NotNil(t, c)
	assert.Equal(t, []string{"foo", "bar"}, c.GetAll("hash1.hash2.hash2var1"))
}

func TestGetAllYaml(t *testing.T) {
	c := loadYamlConfig()
	require.NotNil(t, c)
	assert.Equal(t, []string{"foo", "bar"}, c.GetAll("hash1.hash2.hash2var1"))
}

func TestValidator(t *testing.T) {
}
