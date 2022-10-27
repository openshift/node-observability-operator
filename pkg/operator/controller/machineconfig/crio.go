/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package machineconfigcontroller

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ignutil "github.com/coreos/ignition/v2/config/util"
	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	"github.com/openshift/node-observability-operator/api/v1alpha2"
)

// CrioUnixSocketConfData contains the configuration required
// for enabling CRI-O profiling
var crioUnixSocketConfData = fmt.Sprintf(`[Service]
Environment="%s"`, CrioUnixSocketEnvString)

const (
	// crioProfilingConfigName is the name CRI-O MachineConfig CR
	crioProfilingConfigName = "10-crio-nodeobservability"

	// crioServiceFile is the name of the CRI-O systemd service unit
	crioServiceFile = "crio.service"

	// CrioUnixSocketEnvString refers to the environment variable info
	// string that helps in finding if the profiling is enabled by default
	CrioUnixSocketEnvString = "ENABLE_PROFILE_UNIX_SOCKET=true"

	// crioUnixSocketConfFile is the name of the CRI-O config file
	crioUnixSocketConfFile = "10-mco-profile-unix-socket.conf"
)

// enableCrioProf creates MachineConfig CR for CRI-O profiling.
func (r *MachineConfigReconciler) enableCrioProf(ctx context.Context, nomc *v1alpha2.NodeObservabilityMachineConfig) error {
	criomc, err := r.getCrioProfMachineConfig()
	if err != nil {
		return err
	}

	if err := ctrlutil.SetControllerReference(nomc, criomc, r.Scheme); err != nil {
		return fmt.Errorf("failed to update controller reference in crio profiling machine config: %w", err)
	}

	if err := r.ClientCreate(ctx, criomc); err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create crio profiling machine config: %w", err)
	}

	// TODO: check if there is a diff between desired and existing

	r.Log.V(1).Info("successfully created MachineConfig to enable CRI-O profiling", "mc.name", crioProfilingConfigName)
	return nil
}

// deleteCrioProfMC deletes MachineConfig CR for CRI-O profiling if it exists.
func (r *MachineConfigReconciler) deleteCrioProfMC(ctx context.Context) error {
	criomc := &mcv1.MachineConfig{}
	if err := r.ClientGet(ctx, types.NamespacedName{Name: crioProfilingConfigName}, criomc); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if err := r.ClientDelete(ctx, criomc); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to remove crio profiling machineconfig: %w", err)
	}

	r.Log.V(1).Info("successfully removed machineconfig to disable CRI-O profiling", "mc.name", crioProfilingConfigName)

	return nil
}

// getCrioProfMachineConfig returns the MachineConfig CR definition to enable CRI-O profiling.
func (r *MachineConfigReconciler) getCrioProfMachineConfig() (*mcv1.MachineConfig, error) {
	config := igntypes.Config{
		Ignition: igntypes.Ignition{
			Version: igntypes.MaxVersion.String(),
		},
		Systemd: igntypes.Systemd{
			Units: []igntypes.Unit{
				{
					Dropins: []igntypes.Dropin{
						{
							Name:     crioUnixSocketConfFile,
							Contents: ignutil.StrToPtr(crioUnixSocketConfData),
						},
					},
					Name: crioServiceFile,
				},
			},
		},
	}

	rawExt, err := convertIgnConfToRawExt(config)
	if err != nil {
		return nil, err
	}

	return &mcv1.MachineConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       MCKind,
			APIVersion: MCAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   crioProfilingConfigName,
			Labels: MachineConfigLabels,
		},
		Spec: mcv1.MachineConfigSpec{
			Config: rawExt,
		},
	}, nil
}

// convertIgnConfToRawExt converts the CRI-O ignition configuration
// to k8s raw extension form.
func convertIgnConfToRawExt(config igntypes.Config) (k8sruntime.RawExtension, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return k8sruntime.RawExtension{}, fmt.Errorf("failed to marshal crio ignition config: %w", err)
	}

	return k8sruntime.RawExtension{
		Raw: data,
	}, nil
}
