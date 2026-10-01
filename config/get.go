package config

import (
	"maps"
	"os"
	"slices"

	clabernetesapisv1alpha1 "github.com/clabernetes/clabernetes/apis/v1alpha1"
	clabernetesconstants "github.com/clabernetes/clabernetes/constants"
	k8scorev1 "k8s.io/api/core/v1"
)

func (m *manager) GetGlobalAnnotations() map[string]string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	// we dont want to pass by ref, so make a new map
	outAnnotations := make(map[string]string)

	maps.Copy(outAnnotations, m.config.Metadata.Annotations)

	return outAnnotations
}

func (m *manager) GetGlobalLabels() map[string]string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	// we dont want to pass by ref, so make a new map
	outLabels := make(map[string]string)

	maps.Copy(outLabels, m.config.Metadata.Labels)

	return outLabels
}

func (m *manager) GetAllMetadata() (outAnnotations, outLabels map[string]string) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	outAnnotations = make(map[string]string)

	maps.Copy(outAnnotations, m.config.Metadata.Annotations)

	outLabels = make(map[string]string)

	maps.Copy(outLabels, m.config.Metadata.Labels)

	return outAnnotations, outLabels
}

func (m *manager) GetApplicationImagePullPolicy() string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.config.ImagePull.Policy
}

func (m *manager) GetImagePullSecrets() []string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return slices.Clone(m.config.ImagePull.PullSecrets)
}

func (m *manager) GetDefaultResources() *k8scorev1.ResourceRequirements {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if m.config.Deployment.ResourcesDefault == nil {
		return nil
	}

	return m.config.Deployment.ResourcesDefault.DeepCopy()
}

func (m *manager) GetNodeSelectorsByImage(
	imageName string,
) map[string]string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return GetNodeSelectorsByImage(imageName, m.config.Deployment.NodeSelectorsByImage)
}

func (m *manager) GetRegistryMetadataTrust() []clabernetesapisv1alpha1.RegistryMetadataTrustEntry {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return slices.Clone(m.config.ImagePull.RegistryMetadataTrust)
}

func (m *manager) GetRegistryMetadataMirrors() (
	result []clabernetesapisv1alpha1.RegistryMetadataMirrorEntry,
) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return slices.Clone(m.config.ImagePull.RegistryMetadataMirrors)
}

func (m *manager) GetContainerStopSignals() bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.config.Deployment.ContainerStopSignals
}

// GetLauncherImage returns the Config CR's launcher image, falling back to the manager's
// environment default (the chart's launcher.image) so an install that only sets the Helm value
// still resolves a launcher.
func (m *manager) GetLauncherImage() string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if m.config.Deployment.Launcher == nil || m.config.Deployment.Launcher.Image == "" {
		return os.Getenv(clabernetesconstants.LauncherImageEnv)
	}

	return m.config.Deployment.Launcher.Image
}

func (m *manager) GetLauncherImagePullPolicy() string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if m.config.Deployment.Launcher == nil {
		return ""
	}

	return m.config.Deployment.Launcher.ImagePullPolicy
}

func (m *manager) GetRolloutBatchSize() int32 {
	m.lock.RLock()
	defer m.lock.RUnlock()

	if m.config.Rollout == nil {
		return 0
	}

	return m.config.Rollout.BatchSize
}

func (m *manager) GetRolloutMaxConcurrentPerHost() int32 {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if m.config.Rollout == nil {
		return 0
	}

	return m.config.Rollout.MaxConcurrentPerHost
}
