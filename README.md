[![Documentation](https://godoc.org/github.com/dparrish/go-autoconfig?status.svg)](http://godoc.org/github.com/dparrish/go-autoconfig)
[![Build Status](https://travis-ci.com/dparrish/go-autoconfig.svg?branch=master)](https://travis-ci.com/dparrish/go-autoconfig)

# go-autoconfig
JSON & YAML based configuration library with automatic reload.

## Example Usage
```golang
import "github.com/dparrish/go-autoconfig"

func main() {
	// Load the initial configuration.
	config := autoconfig.New("config.yaml")

	// Ensure that hash.value.here is always set in configuration, even after it changes.
	config.Required("hash.value.here")

	// Ensure that hash.value.cannot.change doesn't change on configuration reload.
	config.Immutable("hash.value.cannot.change")

	// Validate values whenever configuration is loaded.
	config.AddValidator(func(old, new *autoconfig.Config) error {
		if new.GetInt("intvalue") > 1000 {
			return errors.New("intvalue cannot be above 100")
		}
		return nil
	})

	if err := config.Load(); err != nil {
		panic("Configuration is invalid")
	}

	// Start a background goroutine that will reload configuration whenever it changes.
	if err := config.Watch(context.Background()); err != nil {
		panic(err)
	}

	...

	// Read config values in your code. These will always return the latest loaded value.
	log.Printf("String value: %s", config.Get("hash.stringvalue"))
	log.Printf("Int value: %d", config.GetInt("hash.intvalue"))
	log.Printf("List value: %v", config.GetAll("hash.listvalue"))
}
```
