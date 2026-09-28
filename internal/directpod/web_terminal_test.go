package directpod_test

import (
	"slices"
	"strings"
	"testing"

	clabernetesconstants "github.com/clabernetes/clabernetes/constants"
	clabernetesinternaldirectpod "github.com/clabernetes/clabernetes/internal/directpod"
	k8scorev1 "k8s.io/api/core/v1"
)

func webTerminalOptions() clabernetesinternaldirectpod.Options {
	return clabernetesinternaldirectpod.Options{
		Name: "device-a", Namespace: "lab-a", PlanConfigMapName: "device-a-plan-abc",
		InputConfigMapName:                "device-a-plan-input-abc",
		ConnectivityRevisionConfigMapName: "device-a-connectivity",
		PreparationImage:                  "example/c9s@sha256:1111",
		ConnectivityImage:                 "example/c9s@sha256:1111",
		EnableContainerStopSignals:        true,
		LauncherImage:                     "example/c9s-launcher:1",
		LauncherImagePullPolicy:           "Always",
	}
}

func webTerminalSidecar(
	t *testing.T,
	spec k8scorev1.PodSpec,
) *k8scorev1.Container {
	t.Helper()

	for index := range spec.InitContainers {
		if spec.InitContainers[index].Name == "ttyd" {
			return &spec.InitContainers[index]
		}
	}

	t.Fatal("rendered workload carries no ttyd sidecar")

	return nil
}

func assertWebTerminalSidecar(
	t *testing.T,
	sidecar *k8scorev1.Container,
	options clabernetesinternaldirectpod.Options,
) {
	t.Helper()

	if sidecar.Image != options.LauncherImage {
		t.Fatalf("sidecar image = %q, want %q", sidecar.Image, options.LauncherImage)
	}
	if sidecar.ImagePullPolicy != k8scorev1.PullAlways {
		t.Fatalf("sidecar pull policy = %q, want Always", sidecar.ImagePullPolicy)
	}
	if sidecar.RestartPolicy == nil ||
		*sidecar.RestartPolicy != k8scorev1.ContainerRestartPolicyAlways {
		t.Fatal("sidecar is not a restartable native sidecar")
	}
	if sidecar.SecurityContext == nil ||
		sidecar.SecurityContext.Privileged == nil ||
		!*sidecar.SecurityContext.Privileged {
		t.Fatal("sidecar is not privileged, so it cannot enter the device namespaces")
	}
	if len(sidecar.Ports) != 1 ||
		sidecar.Ports[0].ContainerPort != clabernetesconstants.WebTerminalPort {
		t.Fatalf("sidecar ports = %v, want the web terminal port", sidecar.Ports)
	}

	command := strings.Join(sidecar.Command, " ")
	for _, fragment := range []string{
		"ttyd", "7681", "tmux", "clabernetes", "terminal", "--processIDFile", "node-a.pid",
		"--shell", "bash",
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("sidecar command %q lacks %q", command, fragment)
		}
	}
}

func assertWebTerminalRuntimeVolume(
	t *testing.T,
	spec k8scorev1.PodSpec,
	sidecar *k8scorev1.Container,
) {
	t.Helper()

	if !slices.ContainsFunc(spec.Volumes, func(volume k8scorev1.Volume) bool {
		return volume.Name == "node-web-terminal" && volume.EmptyDir != nil
	}) {
		t.Fatal("terminal runtime volume is missing")
	}

	if !slices.ContainsFunc(sidecar.VolumeMounts, func(mount k8scorev1.VolumeMount) bool {
		return mount.Name == "node-web-terminal"
	}) {
		t.Fatal("sidecar does not mount the terminal runtime volume")
	}
}

func assertWebTerminalProcessPublication(t *testing.T, spec k8scorev1.PodSpec) {
	t.Helper()

	root := containerByImage(
		t,
		spec.Containers,
		"example/device@sha256:"+strings.Repeat("a", 64),
	)
	if !slices.Contains(root.Command, "--processIDFile") {
		t.Fatalf("device container command %v does not publish its process id", root.Command)
	}
	if !slices.ContainsFunc(root.VolumeMounts, func(mount k8scorev1.VolumeMount) bool {
		return mount.Name == "node-web-terminal"
	}) {
		t.Fatal("device container does not mount the terminal runtime volume")
	}

	component := containerByImage(
		t,
		spec.Containers,
		"example/component@sha256:"+strings.Repeat("b", 64),
	)
	if slices.Contains(component.Command, "--processIDFile") {
		t.Fatalf("non-primary container command %v publishes a process id", component.Command)
	}
}

// TestRenderAddsWebTerminalSidecarForATerminalNode covers the whole terminal surface: the sidecar
// itself, the shared PID namespace it needs to see the device process, the runtime volume the
// device publishes its process id into, and the launch flag that publishes it.
func TestRenderAddsWebTerminalSidecarForATerminalNode(t *testing.T) {
	t.Parallel()

	options := webTerminalOptions()
	options.WebTerminals = map[string]clabernetesinternaldirectpod.WebTerminal{
		"node-a": {Shell: "bash"},
	}

	deployment, err := clabernetesinternaldirectpod.Render(renderablePlan(), options)
	if err != nil {
		t.Fatal(err)
	}

	spec := deployment.Spec.Template.Spec

	if spec.ShareProcessNamespace == nil || !*spec.ShareProcessNamespace {
		t.Fatal("terminal-enabled workload does not share its process namespace")
	}

	sidecar := webTerminalSidecar(t, spec)
	assertWebTerminalSidecar(t, sidecar, options)
	assertWebTerminalRuntimeVolume(t, spec, sidecar)
	assertWebTerminalProcessPublication(t, spec)
}

// TestRenderLeavesWorkloadsWithoutTerminalsAlone is the negative half: a lab that asks for no
// terminal must not acquire a helper sidecar, a shared PID namespace, or a runtime volume, since
// all three widen the Pod's blast radius.
func TestRenderLeavesWorkloadsWithoutTerminalsAlone(t *testing.T) {
	t.Parallel()

	deployment, err := clabernetesinternaldirectpod.Render(
		renderablePlan(),
		webTerminalOptions(),
	)
	if err != nil {
		t.Fatal(err)
	}

	spec := deployment.Spec.Template.Spec

	if spec.ShareProcessNamespace != nil {
		t.Fatal("workload without a terminal shares its process namespace")
	}

	for _, container := range spec.InitContainers {
		if container.Image == "example/c9s-launcher:1" {
			t.Fatal("workload without a terminal carries a launcher sidecar")
		}
	}

	for _, volume := range spec.Volumes {
		if volume.Name == "node-web-terminal" {
			t.Fatal("workload without a terminal mounts the terminal runtime volume")
		}
	}
}

func TestRenderRejectsTwoGroupedTerminalsInOneWorkload(t *testing.T) {
	t.Parallel()

	options := webTerminalOptions()
	options.WebTerminals = map[string]clabernetesinternaldirectpod.WebTerminal{
		"node-a": {Shell: "bash"},
		"node-b": {Shell: "sh"},
	}

	if _, err := clabernetesinternaldirectpod.Render(
		renderablePlan(), options,
	); err == nil {
		t.Fatal("two grouped terminals rendered instead of being rejected")
	}
}

func TestRenderRejectsTerminalWithoutLauncherImage(t *testing.T) {
	t.Parallel()

	options := webTerminalOptions()
	options.LauncherImage = ""
	options.WebTerminals = map[string]clabernetesinternaldirectpod.WebTerminal{
		"node-a": {Shell: "bash"},
	}

	if _, err := clabernetesinternaldirectpod.Render(
		renderablePlan(), options,
	); err == nil {
		t.Fatal("terminal rendered without a launcher image")
	}
}

// TestRenderPrefersTheTerminalNodesLauncherImage pins the per-Node override: launcher-image on a
// Node wins over the lab-wide launcher image, and a Node without one still gets the lab-wide
// value, so the config hierarchy reads the same here as everywhere else in the runtime.
func TestRenderPrefersTheTerminalNodesLauncherImage(t *testing.T) {
	t.Parallel()

	options := webTerminalOptions()
	options.WebTerminals = map[string]clabernetesinternaldirectpod.WebTerminal{
		"node-a": {Shell: "bash", LauncherImage: "example/lab-launcher:1"},
	}

	deployment, err := clabernetesinternaldirectpod.Render(renderablePlan(), options)
	if err != nil {
		t.Fatal(err)
	}

	sidecar := webTerminalSidecar(t, deployment.Spec.Template.Spec)
	if sidecar.Image != "example/lab-launcher:1" {
		t.Fatalf("sidecar runs the lab-wide launcher image %q", sidecar.Image)
	}

	options.WebTerminals = map[string]clabernetesinternaldirectpod.WebTerminal{
		"node-a": {Shell: "bash"},
	}

	deployment, err = clabernetesinternaldirectpod.Render(renderablePlan(), options)
	if err != nil {
		t.Fatal(err)
	}

	if sidecar = webTerminalSidecar(t, deployment.Spec.Template.Spec); sidecar.Image != "example/c9s-launcher:1" {
		t.Fatalf("sidecar runs image %q, want the lab-wide launcher image", sidecar.Image)
	}
}
