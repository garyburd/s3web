package scratch

import "fmt"

type Scratch struct {
	m map[string]interface{}
}

func New() *Scratch {
	return &Scratch{m: make(map[string]interface{})}
}

// Set sets the value for a key.
func (d *Scratch) Set(key string, value interface{}) string {
	d.m[key] = value
	return ""
}

// Get gets the value for a key. Get returns nil if the there is no value for
// the key.
func (d *Scratch) Get(key string) interface{} {
	return d.m[key]
}

// Has returns whether there is a value for key.
func (d *Scratch) Has(key string) bool {
	_, ok := d.m[key]
	return ok
}

// Delete deletes the value for key.
func (d *Scratch) Delete(key string) string {
	delete(d.m, key)
	return ""
}

// Append appends the value to the slice for key. The function fails if the
// current value for key is not a slice of interface{}.
func (d *Scratch) Append(key string, value interface{}) (string, error) {
	v, ok := d.m[key]
	if !ok {
		d.m[key] = []interface{}{value}
		return "", nil
	}
	s, ok := v.([]interface{})
	if !ok {
		return "", fmt.Errorf("append value that is not a slice, value is %T", v)
	}
	d.m[key] = append(s, value)
	return "", nil
}
