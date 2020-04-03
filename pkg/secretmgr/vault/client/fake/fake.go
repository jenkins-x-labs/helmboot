package fake

import (
	"github.com/jenkins-x-labs/helmboot/pkg/secretmgr/vault/client"
)

type FakeClient struct {
	Data map[string]map[string]interface{}
}

// implements interface
var _ client.Client = (*FakeClient)(nil)

func (f *FakeClient) Read(name string) (map[string]interface{}, error) {
	if f.Data == nil {
		f.Data = map[string]map[string]interface{}{}
	}
	return f.Data[name], nil
}

func (f *FakeClient) Write(name string, values map[string]interface{}) error {
	if f.Data == nil {
		f.Data = map[string]map[string]interface{}{}
	}
	f.Data[name] = values
	return nil
}

func (f *FakeClient) String() string {
	return "fake vault"
}
