package secretmgr

const (
	// KindLocal for using a local Secret in Kubernetes
	KindLocal = "local"

	// KindGoogleSecretManager for using Google Secret Manager
	KindGoogleSecretManager = "gsm"

	// KindFake for a fake secret manager
	KindFake = "fake"

	// KindVault for a vault based secret manager
	KindVault = "vault"

	// LocalSecret the name of the Kubernetes Secret used to load/store the
	// secrets
	/* #nosec */
	LocalSecret = "jx-boot-secrets"

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

var (
	// KindValues the kind of secret managers we support
	KindValues = []string{KindGoogleSecretManager, KindLocal}
)
