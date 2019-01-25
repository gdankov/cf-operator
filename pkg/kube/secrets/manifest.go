package secrets

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
)

const (
	manifestKeyName   = "manifest"
	versionKeyName    = "version"
	sourceKeyName     = "source-description"
	deploymentKeyName = "deployment-name"
)

// ManifestPersister is the interface to persist manifests in Kubernetes
//
// It uses Kubernetes secrets to persist a manifest. There can only be one
// manifest per deployment. Each update to the manifest results in a new
// persisted version. An existing persisted version of a manifest cannot be
// altered or deleted. The deletion of a manifest will result in the removal
// of all persisted version of that manifest.
//
// The version number is an integer that is incremented with each version of
// the manifest, which the greatest number being the current/latest version.
//
// When persisting a new manifest, a source description is required, which
// should explain the sources of the rendered manifest, e.g. the location of
// the Custom Resource Definition that generated it.
type ManifestPersister interface {
	PersistManifest(manifest manifest.Manifest, sourceDescription string) (*corev1.Secret, error)
	DeleteManifest() error
	DecorateManifest(key string, value string) (*corev1.Secret, error)
	ListAllVersions() ([]corev1.Secret, error)
	RetrieveVersion(version int) (*corev1.Secret, error)
	RetrieveLatestVersion() (*corev1.Secret, error)
}

// ManifestPersisterImpl contains the required fields to persist a manifest
type ManifestPersisterImpl struct {
	kubeClient     client.Client
	namespace      string
	deploymentName string
}

// NewManifestPersister returns a ManifestPersister implementation to be used
// when working with desired manifest secrets
func NewManifestPersister(client client.Client, namespace string, deploymentName string) ManifestPersisterImpl {
	return ManifestPersisterImpl{
		client,
		namespace,
		deploymentName,
	}
}

// PersistManifest creates a new version of the manifest if it already exists,
// or a first one it not. A source description should explain the sources of
// the rendered manifest, e.g. the location of the CRD that generated it
func (p ManifestPersisterImpl) PersistManifest(manifest manifest.Manifest, sourceDescription string) (*corev1.Secret, error) {
	currentVersion, err := getGreatestVersion(p)
	if err != nil {
		return nil, err
	}

	version := currentVersion + 1

	secretName, err := secretName(p, version)
	if err != nil {
		return nil, err
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: p.namespace,
			Labels: map[string]string{
				deploymentKeyName: p.deploymentName,
				versionKeyName:    strconv.Itoa(version),
			},
			Annotations: map[string]string{
				sourceKeyName: sourceDescription,
			},
		},
		Data: map[string][]byte{
			manifestKeyName: data,
		},
	}

	return secret, p.kubeClient.Create(context.TODO(), secret)
}

// RetrieveVersion returns a specific version of the manifest
func (p ManifestPersisterImpl) RetrieveVersion(version int) (*corev1.Secret, error) {
	name, err := secretName(p, version)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{}
	return secret, p.kubeClient.Get(context.TODO(), client.ObjectKey{Namespace: p.namespace, Name: name}, secret)
}

// RetrieveLatestVersion returns the latest version of the manifest
func (p ManifestPersisterImpl) RetrieveLatestVersion() (*corev1.Secret, error) {
	latestVersion, err := getGreatestVersion(p)
	if err != nil {
		return nil, err
	}
	return p.RetrieveVersion(latestVersion)
}

// ListAllVersions returns all versions of the manifest
func (p ManifestPersisterImpl) ListAllVersions() ([]corev1.Secret, error) {
	secrets := &corev1.SecretList{}
	if err := p.kubeClient.List(context.TODO(), client.InNamespace(p.namespace), secrets); err != nil {
		return nil, err
	}

	result := []corev1.Secret{}

	nameRegex := regexp.MustCompile(fmt.Sprintf(`^deployment-%s-\d+$`, p.deploymentName))
	for _, secret := range secrets.Items {
		if nameRegex.MatchString(secret.Name) {
			result = append(result, secret)
		}
	}

	return result, nil
}

// DeleteManifest removes all versions of the manifest and therefore the
// manifest itself.
func (p ManifestPersisterImpl) DeleteManifest() error {
	list, err := p.ListAllVersions()
	if err != nil {
		return err
	}

	for _, secret := range list {
		if err := p.kubeClient.Delete(context.TODO(), &secret); err != nil {
			return err
		}
	}

	return nil
}

// DecorateManifest adds a label to the lastest version of the manifest
func (p ManifestPersisterImpl) DecorateManifest(key string, value string) (*corev1.Secret, error) {
	version, err := getGreatestVersion(p)
	if err != nil {
		return nil, err
	}

	secretName, err := secretName(p, version)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{}
	if err := p.kubeClient.Get(context.TODO(), client.ObjectKey{Namespace: p.namespace, Name: secretName}, secret); err != nil {
		return nil, err
	}

	labels := secret.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[key] = value
	secret.SetLabels(labels)

	return secret, p.kubeClient.Update(context.TODO(), secret)
}

func getGreatestVersion(p ManifestPersisterImpl) (int, error) {
	list, err := p.ListAllVersions()
	if err != nil {
		return -1, err
	}

	var greatestVersion int
	for _, secret := range list {
		version, err := getVersionFromSecretName(p, secret.Name)
		if err != nil {
			return 0, err
		}

		if version > greatestVersion {
			greatestVersion = version
		}
	}

	return greatestVersion, nil
}

func secretName(p ManifestPersisterImpl, version int) (string, error) {
	proposedName := fmt.Sprintf("deployment-%s-%d", p.deploymentName, version)

	// Check for Kubernetes name requirements (length)
	const maxChars = 253
	if len(proposedName) > maxChars {
		return "", fmt.Errorf("secret name exceeds maximum number of allowed characters (actual=%d, allowed=%d)", len(proposedName), maxChars)
	}

	// Check for Kubernetes name requirements (characters)
	if re := regexp.MustCompile(`[^a-z0-9.-]`); re.MatchString(proposedName) {
		return "", fmt.Errorf("secret name contains invalid characters, only lower case, dot and dash are allowed")
	}

	return proposedName, nil
}

func getVersionFromSecretName(p ManifestPersisterImpl, name string) (int, error) {
	nameRegex := regexp.MustCompile(fmt.Sprintf(`^deployment-%s-(\d+)$`, p.deploymentName))
	if captures := nameRegex.FindStringSubmatch(name); len(captures) > 0 {
		number, err := strconv.Atoi(captures[1])
		if err != nil {
			return -1, errors.Wrapf(err, "invalid secret name %s, it does not end with a version number", name)
		}

		return number, nil
	}

	return -1, fmt.Errorf("invalid secret name %s, it does not match the naming schema", name)
}