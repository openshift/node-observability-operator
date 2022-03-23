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
	ctrlruntime "sigs.k8s.io/controller-runtime"

	ignutil "github.com/coreos/ignition/v2/config/util"
	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"
	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
)

const (
	// CrioProfilingConfigName is the name CRI-O MachineConfig CR
	CrioProfilingConfigName = "99-crio-profiling"

	// CrioUnixSocketConfFile is the name of the CRI-O config file
	CrioUnixSocketConfFile = "10-mco-profile-unix-socket.conf"

	// CriounixSocketConfData contains the configuration required
	// for enabling CRI-O profiling
	CriounixSocketConfData = `[Service]
Environment="ENABLE_PROFILE_UNIX_SOCKET=true"`
)

// ensureCrioProfConfigExists checks if CRI-O MachineConfig CR for
// enabling profiling exists, if not creates the resource
func (r *MachineconfigReconciler) ensureCrioProfConfigExists(ctx context.Context) (*mcv1.MachineConfig, bool, error) {
	namespace := types.NamespacedName{Namespace: r.CtrlConfig.Namespace, Name: CrioProfilingConfigName}
	criomc, exist, err := r.fetchCrioProfileConfig(ctx, namespace)
	if err != nil {
		return nil, false, err
	}
	if !exist {
		if err := r.createCrioProfileConfig(ctx); err != nil {
			return nil, false, err
		}

		criomc, exist, err = r.fetchCrioProfileConfig(ctx, namespace)
		if err != nil || !exist {
			return nil, false, fmt.Errorf("failed to fetch just created crio config: %w", err)
		}

		return criomc, true, nil
	}
	return criomc, false, nil
}

// ensureCrioProfConfigNotExists checks if CRI-O MachineConfig CR for
// enabling profiling exists, if exists delete the resource
func (r *MachineconfigReconciler) ensureCrioProfConfigNotExists(ctx context.Context) (bool, error) {
	namespace := types.NamespacedName{Namespace: r.CtrlConfig.Namespace, Name: CrioProfilingConfigName}
	criomc, exists, err := r.fetchCrioProfileConfig(ctx, namespace)
	if err != nil {
		return false, err
	}
	if exists {
		if err := r.deleteCrioProfileConfig(ctx, criomc); err != nil {
			return false, err
		}

		_, exists, err = r.fetchCrioProfileConfig(ctx, namespace)
		if err != nil || exists {
			return false, fmt.Errorf("failed to delete crio profiling config: %w", err)
		}

		return true, nil
	}
	return false, nil
}

// fetchCrioProfileConfig is for fetching the CRI-O MC CR created
// by this controller for enabling profiling
func (r *MachineconfigReconciler) fetchCrioProfileConfig(ctx context.Context, namespace types.NamespacedName) (*mcv1.MachineConfig, bool, error) {
	criomc := &mcv1.MachineConfig{}

	if err := r.Get(ctx, namespace, criomc); err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return criomc, true, nil
}

// createCrioProfileConfig is for creating CRI-O MC CR
func (r *MachineconfigReconciler) createCrioProfileConfig(ctx context.Context) error {
	criomc, err := r.getCrioConfig()
	if err != nil {
		return err
	}

	if err := r.Create(ctx, criomc); err != nil {
		return fmt.Errorf("failed to create crio profiling config %s/%s: %w", criomc.Namespace, criomc.Name, err)
	}

	ctrlruntime.SetControllerReference(r.CtrlConfig, criomc, r.Scheme)
	r.Log.Info("successfully created CRI-O machine config(%s) for enabling profiling", CrioProfilingConfigName)
	return nil
}

// deleteCrioProfileConfig is for deleting CRI-O MC CR
func (r *MachineconfigReconciler) deleteCrioProfileConfig(ctx context.Context, criomc *mcv1.MachineConfig) error {
	if err := r.Delete(ctx, criomc); err != nil {
		return fmt.Errorf("failed to remove crio profiling config %s/%s: %w", criomc.Namespace, criomc.Name, err)
	}

	r.Log.Info("successfully removed CRI-O machine config(%s) to disable profiling", CrioProfilingConfigName)
	return nil
}

// getCrioIgnitionConfig returns the required ignition config
// required for creating CRI-O MC CR
func getCrioIgnitionConfig() igntypes.Config {
	dropins := []igntypes.Dropin{
		igntypes.Dropin{
			Name:     CrioUnixSocketConfFile,
			Contents: ignutil.StrToPtr(CriounixSocketConfData),
		},
	}

	units := []igntypes.Unit{
		igntypes.Unit{
			Dropins: dropins,
		},
	}

	return igntypes.Config{
		Ignition: igntypes.Ignition{
			Version: igntypes.MaxVersion.String(),
		},
		Systemd: igntypes.Systemd{
			Units: units,
		},
	}
}

// convertIgnConfToRawExt converts the CRI-O ignition configuration
// to k8s raw extension form
func convertIgnConfToRawExt(config igntypes.Config) (k8sruntime.RawExtension, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return k8sruntime.RawExtension{}, fmt.Errorf("failed to marshal crio ignition config: %w", err)
	}

	return k8sruntime.RawExtension{
		Raw: data,
	}, nil
}

// getCrioConfig returns the CRI-O MC CR data required for creating it
func (r *MachineconfigReconciler) getCrioConfig() (*mcv1.MachineConfig, error) {
	config := getCrioIgnitionConfig()

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
			Name:   CrioProfilingConfigName,
			Labels: ProfilingMCSelectorLabels,
		},
		Spec: mcv1.MachineConfigSpec{
			Config: rawExt,
		},
	}, nil
}
