package secrets

import (
	"strings"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// MemoryClient a local file system based client loading/saving content from the given URL
type MemoryClient struct {
	Data map[string]interface{}
}

// NewMemoryClient create a memory client
func NewMemoryClient() *MemoryClient {
	return &MemoryClient{
		Data: map[string]interface{}{},
	}
}

// Read reads a named secret from the vault
func (c *MemoryClient) Read(secretName string) (map[string]interface{}, error) {
	path := toPath(secretName)
	data := util.GetMapValueViaPath(c.Data, path)

	if data == nil {
		data = map[string]interface{}{}
	}
	util.SetMapValueViaPath(c.Data, path, data)
	mdata, ok := data.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("data for secret %s is not a map[string]interface{}", secretName)
	}

	// lets remove any null entries from the map as the initial default YAML makes for lots of null entries
	// and the json schema questions code asssumes a null value is a valid entry so doesn't prompt for another one
	removes := []string{}
	for k, v := range mdata {
		if v == nil {
			removes = append(removes, k)
		}
	}
	for _, k := range removes {
		delete(mdata, k)
	}
	return mdata, nil
}

// ReadObject reads a generic named object from vault.
// The secret _must_ be serializable to JSON.
func (c *MemoryClient) ReadObject(secretName string, secret interface{}) error {
	m, err := c.Read(secretName)
	if err != nil {
		return errors.Wrapf(err, "reading the secret %q from vault", secretName)
	}
	err = util.ToStructFromMapStringInterface(m, &secret)
	if err != nil {
		return errors.Wrapf(err, "deserializing the secret %q from vault", secretName)
	}
	return nil
}

// Write writes a named secret to the vault with the Data provided. Data can be a generic map of stuff, but at all points
// in the map, keys _must_ be strings (not bool, int or even interface{}) otherwise you'll get an error
func (c *MemoryClient) Write(secretName string, data map[string]interface{}) (map[string]interface{}, error) {
	path := toPath(secretName)
	util.SetMapValueViaPath(c.Data, path, data)
	return c.Read(secretName)
}

// WriteObject writes a generic named object to the vault.
// The secret _must_ be serializable to JSON.
func (c *MemoryClient) WriteObject(secretName string, data interface{}) (map[string]interface{}, error) {
	path := toPath(secretName)
	util.SetMapValueViaPath(c.Data, path, data)
	return c.Read(secretName)
}

// converts '/secrets/foo/bar' to 'secrets.foo.bar' so we can use the helper function to set thedata
func toPath(secretName string) string {
	return strings.ReplaceAll(strings.TrimPrefix(secretName, "/"), "/", ".")
}

// ReplaceURIs will replace any local: URIs in a string
func (c *MemoryClient) ReplaceURIs(s string) (string, error) {
	return s, nil
}

// ToYAML converts the data to YAML
func (c *MemoryClient) ToYAML() (string, error) {
	data, err := yaml.Marshal(c.Data)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal data to YAML")
	}
	return string(data), nil
}
