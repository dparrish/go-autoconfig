// Package autoconfig wraps a JSON or YAML configuration stored on disk that is queryable using the Get* functions.
//
// The configuration file will be watched for changes after the initial load. Whenever the file has changed, each
// validation function will be called in the order they were added.
package autoconfig

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/clbanning/mxj"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
)

var Fs = afero.NewOsFs()

// Config wraps a JSON/YAML configuration stored on disk and provides functions to query it.
type Config struct {
	sync.RWMutex
	filename   string
	mv         mxj.Map
	defaults   mxj.Map
	validators []func(old *Config, new *Config) error
	loaded     bool
}

// New creates a new empty configuration.
func New(filename string) *Config {
	return &Config{
		filename: filename,
		mv:       mxj.Map{},
		defaults: mxj.Map{},
	}
}

// Load loads a configuration file from disk.
func (c *Config) Load() error {
	if err := c.read(); err != nil {
		return fmt.Errorf("unable to read initial config: %v", err)
	}
	return nil
}

// Watch starts a background goroutine to watch for changes in the configuration.
// When changes are detected, the validator functions are called with the new configuration.
func (c *Config) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("couldn't create config watcher: %v", err)
	}
	if err := watcher.Add(c.filename); err != nil {
		return fmt.Errorf("couldn't create config watcher: %v", err)
	}
	go c.background(ctx, watcher)
	return nil
}

// AddValidator adds a function that will be called whenever the config file changes.
// The function will be passed both the old and new configurations. If the function returns an error, the new
// configuration will not be applied.
// The validation function *may* modify the new config but *must not* modify the old config.
func (c *Config) AddValidator(f func(old, new *Config) error) {
	c.Lock()
	c.validators = append(c.validators, f)
	c.Unlock()
}

// GetRaw looks up the raw configuration item and does not do any conversion to a particular type.
// This is generally only used by the other Get* functions but is exposed for convenience.
func (c *Config) GetRaw(path string) interface{} {
	c.RLock()
	defer c.RUnlock()
	values, err := c.mv.ValuesForPath(path)
	if err != nil {
		log.Printf("Error in ValuesForPath(%q): %v", path, err)
	}
	if len(values) != 0 {
		return values[0]
	}

	values, err = c.defaults.ValuesForPath(path)
	if err != nil {
		log.Printf("Error in ValuesForPath(%q): %v", path, err)
	}
	if len(values) != 0 {
		return values[0]
	}

	return nil
}

// Get looks up a configuration item in dotted path notation and returns the first (or only) value in string form.
// Example: c.Get("spanner.database.path")
func (c *Config) Get(path string) string {
	i := c.GetRaw(path)
	if i == nil {
		return ""
	}
	switch t := i.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		log.Printf("Get() Error in value %q, expected string, got %T", path, t)
		return ""
	}
}

// GetFloat looks up a configuration item in dotted path notation and returns the first (or only) value in float64 form.
func (c *Config) GetFloat(path string) float64 {
	i := c.GetRaw(path)
	if i == nil {
		return 0
	}
	switch t := i.(type) {
	case float64:
		return t
	default:
		log.Printf("GetFloat() Error in value %q, expected float64, got %T", path, t)
		return 0
	}
}

// GetFloat looks up a configuration item in dotted path notation and returns the first (or only) value in int form.
func (c *Config) GetInt(path string) int {
	i := c.GetRaw(path)
	if i == nil {
		return 0
	}
	switch t := i.(type) {
	case int:
		return t
	case float64:
		return int(t)
	default:
		log.Printf("GetInt() Error in value %q, expected int, got %T", path, t)
		return 0
	}
}

// Get looks up a configuration item in dotted path notation and returns a list of values.
func (c *Config) GetAll(path string) []string {
	c.RLock()
	defer c.RUnlock()
	values, err := c.mv.ValuesForPath(path)
	if err != nil {
		log.Printf("Error in ValuesForPath(%q): %v", path, err)
	}

	if values == nil || len(values) == 0 {
		values, err = c.defaults.ValuesForPath(path)
		if err != nil {
			log.Printf("Error in ValuesForPath(%q): %v", path, err)
		}
	}

	if values == nil || len(values) == 0 {
		// Return an empty slice instead of nil so that client code doesn't have to check for nil.
		return []string{}
	}

	r := make([]string, 0, len(values))
	for _, v := range values {
		r = append(r, v.(string))
	}
	return r
}

// GetMapList looks up a configuration item and returns a list of maps for each.
func (c *Config) GetMapList(path string) []map[string]interface{} {
	c.RLock()
	defer c.RUnlock()
	values, err := c.mv.ValuesForPath(path)
	if err != nil {
		log.Printf("Error in ValuesForPath(%q): %v", path, err)
	}

	if values == nil || len(values) == 0 {
		values, err = c.defaults.ValuesForPath(path)
		if err != nil {
			log.Printf("Error in ValuesForPath(%q): %v", path, err)
		}
	}

	if values == nil || len(values) == 0 {
		// Return an empty slice instead of nil so that client code doesn't have to check for nil.
		return []map[string]interface{}{}
	}

	r := make([]map[string]interface{}, 0, len(values))
	for _, v := range values {
		m := make(map[string]interface{})
		for key, value := range v.(map[interface{}]interface{}) {
			m[key.(string)] = value
		}
		r = append(r, m)
	}
	return r
}

func (c *Config) read() error {
	body, err := afero.ReadFile(Fs, c.filename)
	if err != nil {
		return fmt.Errorf("couldn't read config file %q: %v", c.filename, err)
	}

	mv, err := mxj.NewMapJson(body)
	if err != nil {
		mv, err = c.readYAML(body)
		if err != nil {
			return fmt.Errorf("couldn't parse config: %v", err)
		}
	}

	newConfig := &Config{
		filename: c.filename,
		mv:       mv,
	}
	for _, f := range c.validators {
		if err := f(c, newConfig); err != nil {
			log.Printf("Config validation failed: %v", err)
			return err
		}
	}

	c.Lock()
	c.mv = mv
	c.loaded = true
	c.Unlock()
	return nil
}

func (c *Config) readYAML(body []byte) (mxj.Map, error) {
	mv := mxj.Map{}
	if err := yaml.Unmarshal(body, &mv); err != nil {
		return nil, err
	}

	// This is nasty. yaml.Unmarshal returns maps as map[interface{}]interface{},
	// where mxj expects them to be map[string]interface{} and won't find nested
	// values unless it's the correct type. This horrible code converts the
	// former to the latter.
	//
	// TODO(dparrish): Get rid of it.
	for k, v := range mv {
		switch t := v.(type) {
		case map[interface{}]interface{}:
			mv[k] = convertInterfaceToString(t)
		}
	}

	return mv, nil
}

func convertInterfaceToString(mv map[interface{}]interface{}) map[string]interface{} {
	r := map[string]interface{}{}
	for k, v := range mv {
		r[k.(string)] = v
		switch t := v.(type) {
		case map[interface{}]interface{}:
			r[k.(string)] = convertInterfaceToString(t)
		}
	}
	return r
}

func (c *Config) background(ctx context.Context, watcher *fsnotify.Watcher) {
	defer watcher.Close()
	t := make(<-chan time.Time)
	for {
		select {
		case <-ctx.Done():
			// Stop watching when the context is cancelled.
			return
		case _, ok := <-watcher.Events:
			if !ok {
				log.Printf("Watcher ended for %q", c.filename)
				return
			}
			// Create a timer to re-read the config file one second after noticing an event. This prevents the config file
			// being read multiple times for a single file change.
			t = time.After(1 * time.Second)
			// Re-watch the file for further changes.
			watcher.Add(c.filename)
		case <-t:
			if err := c.read(); err != nil {
				log.Printf("Error re-reading config file, keeping existing config: %v", err)
			} else {
				log.Printf("Read changed config file %q", c.filename)
			}
		}
	}
}

// Required marks a configuration entry as required.
// If the value is missing when the configuration changes, the new configuration will be rejected.
func (c *Config) Required(key string) {
	c.AddValidator(func(old, new *Config) error {
		if new.GetRaw(key) == nil {
			return fmt.Errorf("%q is missing from the configuration", key)
		}
		return nil
	})
}

// Immutable marks a configuration entry as immutable.
// If the value changes when the configuration is updated, the new configuration will be rejected.
func (c *Config) Immutable(key string) {
	c.AddValidator(func(old, new *Config) error {
		if !old.loaded {
			// Don't validate changes on the initial load.
			return nil
		}
		if !reflect.DeepEqual(new.GetRaw(key), old.GetRaw(key)) {
			return fmt.Errorf("%q is marked as immutable and has changed, rejecting new configuration", key)
		}
		return nil
	})
}

// Default sets the default value of an entry.
func (c *Config) Default(key string, value interface{}) {
	c.defaults.SetValueForPath(value, key)
}
