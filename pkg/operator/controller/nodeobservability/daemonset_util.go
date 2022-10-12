package nodeobservabilitycontroller

import (
	"reflect"
	"sort"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

// indexedVolumeMount is the standard core volume mount with additional index field
type indexedVolumeMount struct {
	corev1.VolumeMount
	Index int
}

// indexedContainer is the standard core container with additional index field
type indexedContainer struct {
	corev1.Container
	Index int
}

// indexedVolume is the standard core volume with additional index field
type indexedVolume struct {
	corev1.Volume
	Index int
}

// labelsForNodeObservability returns the labels for selecting the resources
// belonging to the given node observability CR name.
func labelsForNodeObservability(name string) map[string]string {
	return map[string]string{"app": "nodeobservability", "nodeobs_cr": name}
}

// buildIndexedVolumeMountMap builds a map from the given list of volume mounts,
// key is the volume name,
// value is the indexed volume mount with the index being the sequence number of the given list.
func buildIndexedVolumeMountMap(volumeMounts []corev1.VolumeMount) map[string]indexedVolumeMount {
	m := map[string]indexedVolumeMount{}
	for i, vol := range volumeMounts {
		m[vol.Name] = indexedVolumeMount{
			VolumeMount: vol,
			Index:       i,
		}
	}
	return m
}

// volumeMountsChanged checks that the current volume mounts have all expected ones,
// returns true if the current volume mounts had to be changed to match the expected.
func volumeMountsChanged(current, expected []corev1.VolumeMount) (bool, []corev1.VolumeMount) {
	updated := make([]corev1.VolumeMount, len(expected))
	copy(updated, expected)

	if len(current) == 0 {
		if len(expected) == 0 {
			return false, updated
		}
		return true, expected
	}

	changed := false

	currentVolumeMountMap := buildIndexedVolumeMountMap(current)
	expectedVolumeMountMap := buildIndexedVolumeMountMap(expected)

	// ensure all expected volume mounts are present,
	// unsolicited ones are kept (e.g. kube api token)
	for currName, currVol := range currentVolumeMountMap {
		if expVol, expExists := expectedVolumeMountMap[currName]; !expExists {
			updated = append(updated, currVol.VolumeMount)
			changed = true
		} else {
			if !reflect.DeepEqual(currVol.VolumeMount, expVol.VolumeMount) {
				updated[expVol.Index] = expVol.VolumeMount
				changed = true
			}
		}
	}

	return changed, updated
}

// value is the indexed volume with the index being the sequence number of the given list.
func buildIndexedVolumeMap(volumes []corev1.Volume) map[string]indexedVolume {
	m := map[string]indexedVolume{}
	for i, vol := range volumes {
		m[vol.Name] = indexedVolume{
			Volume: vol,
			Index:  i,
		}
	}
	return m
}

// volumeMountsChanged checks that the current volume have all expected volumes,
// returns true if the current volumes had to be changed to match the expected.
func volumesChanged(current []corev1.Volume, desired []corev1.Volume) (bool, []corev1.Volume) {
	updated := make([]corev1.Volume, len(desired))
	copy(updated, desired)

	if len(current) == 0 {
		if len(desired) == 0 {
			return false, nil
		}
		updated = desired
		return true, updated
	}

	changed := false

	currentVolumeMap := buildIndexedVolumeMap(current)
	expectedVolumeMap := buildIndexedVolumeMap(desired)

	// ensure all expected volumes are present,
	// unsolicited ones are kept (e.g. kube api token)
	for expName, expVol := range expectedVolumeMap {
		if currVol, currExists := currentVolumeMap[expName]; !currExists {
			updated = append(updated, expVol.Volume)
			changed = true
		} else {
			// deepequal is fine here as we don't have more than 1 item
			// neither in the secret nor in the configmap
			if !reflect.DeepEqual(currVol.Volume, expVol.Volume) {
				updated[currVol.Index] = expVol.Volume
				changed = true
			}
		}
	}

	return changed, updated
}

// equalEnvVars returns true if 2 env variable slices have the same content (order doesn't matter).
func equalEnvVars(current, expected []corev1.EnvVar) bool {
	var currentSorted, expectedSorted []string
	for _, env := range current {
		currentSorted = append(currentSorted, env.Name+" "+env.Value)
	}
	for _, env := range expected {
		expectedSorted = append(expectedSorted, env.Name+" "+env.Value)
	}
	sort.Strings(currentSorted)
	sort.Strings(expectedSorted)
	return cmp.Equal(currentSorted, expectedSorted)
}

// buildIndexedContainerMap builds a map from the given list of containers,
// key is the container name,
// value is the indexed container with the index being the sequence number of the given list.
func buildIndexedContainerMap(containers []corev1.Container) map[string]indexedContainer {
	m := map[string]indexedContainer{}
	for i, cont := range containers {
		m[cont.Name] = indexedContainer{
			Container: cont,
			Index:     i,
		}
	}
	return m
}

// containersChanged checks that the current containers have all expected containers,
// returns true if the current containers have to be changed to match the expected.
func containersChanged(current, expected []corev1.Container) (bool, []corev1.Container) {
	changed := false

	updatedContainers := make([]corev1.Container, len(current))
	copy(updatedContainers, current)

	currentContMap := buildIndexedContainerMap(current)
	expectedContMap := buildIndexedContainerMap(expected)

	// ensure all expected containers are present,
	// unsolicited ones are kept (e.g. service mesh proxy injection)
	for expName, expCont := range expectedContMap {
		// expected container is present
		if currCont, found := currentContMap[expName]; found {
			if currCont.Image != expCont.Image {
				updatedContainers[currCont.Index].Image = expCont.Image
				changed = true
			}
			cmpOpts := cmpopts.SortSlices(func(a, b string) bool { return a < b })
			if !cmp.Equal(currCont.Args, expCont.Args, cmpOpts) {
				updatedContainers[currCont.Index].Args = expCont.Args
				changed = true
			}
			if !cmp.Equal(currCont.Command, expCont.Command) {
				updatedContainers[currCont.Index].Command = expCont.Command
				changed = true
			}
			if !equalEnvVars(currCont.Env, expCont.Env) {
				updatedContainers[currCont.Index].Env = expCont.Env
				changed = true
			}
			if vmChanged, updatedVolumeMounts := volumeMountsChanged(currCont.VolumeMounts, expCont.VolumeMounts); vmChanged {
				updatedContainers[currCont.Index].VolumeMounts = updatedVolumeMounts
				changed = true
			}
			if hasSecurityContextChanged(currCont.SecurityContext, expCont.SecurityContext) {
				updatedContainers[currCont.Index].SecurityContext = expCont.SecurityContext
				changed = true
			}

		} else {
			// expected container is not present - add it
			updatedContainers = append(updatedContainers, expCont.Container)
			changed = true
		}
	}

	return changed, updatedContainers
}

func hasSecurityContextChanged(current, desired *corev1.SecurityContext) bool {
	if desired == nil {
		return false
	}

	if current == nil {
		return true
	}

	return !equalBoolPtr(current.Privileged, desired.Privileged)
}

func equalBoolPtr(current, desired *bool) bool {
	if desired == nil {
		return true
	}

	if current == nil {
		return false
	}

	return *current == *desired
}
