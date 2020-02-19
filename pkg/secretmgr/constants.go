package secretmgr

const (
	// KindLocal for using a local Secret in Kubernetes
	KindLocal = "local"
	// KindFake for a fake secret manager
	KindFake = "fake"

	// LocalSecret the name of the Kubernetes Secret used to load/store the
	// secrets
	LocalSecret = "helmboot-secrets"

	// LocalSecretKey the key in the local Secret to store the YAML secrets
	LocalSecretKey = "secrets.yaml"

	// DefaultSecretsYaml the default YAML
	DefaultSecretsYaml = `secrets:
  adminUser:
    username: 
    password: 
  hmacToken: 
  pipelineUser:
    username: 
    token: 
    email:
`
)
