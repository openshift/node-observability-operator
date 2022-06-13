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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ignutil "github.com/coreos/ignition/v2/config/util"
	igntypes "github.com/coreos/ignition/v2/config/v3_2/types"
	mcv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
)

// enableCrioProf checks if CRI-O MachineConfig CR for
// enabling profiling exists, if not creates the resource
func (r *MachineConfigReconciler) enableCrioProf(ctx context.Context) (bool, error) {
	createdNow := false
	namespace := types.NamespacedName{Name: CrioProfilingConfigName}

	err := r.createCrioProfConf(ctx)
	if err != nil && !errors.IsAlreadyExists(err) {
		return createdNow, err
	}
	if err == nil {
		createdNow = true
	}

	// grace time for client cache to get refreshed
	time.Sleep(500 * time.Millisecond)

	criomc, err := r.fetchCrioProfConf(ctx, namespace)
	if err != nil {
		return createdNow, fmt.Errorf("failed to ensure crio profiling config was indeed created: %w", err)
	}

	r.Lock()
	defer r.Unlock()
	r.MachineConfig.PrevReconcileUpd["crio"] = MachineConfigInfo{
		op:     "create",
		config: criomc,
	}

	return createdNow, nil
}

// disableCrioProf checks if CRI-O MachineConfig CR for
// enabling profiling exists, if exists delete the resource
func (r *MachineConfigReconciler) disableCrioProf(ctx context.Context) error {
	namespace := types.NamespacedName{Name: CrioProfilingConfigName}

	criomc, err := r.fetchCrioProfConf(ctx, namespace)
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	if err := r.deleteCrioProfConf(ctx, criomc); err != nil {
		return err
	}

	r.Lock()
	defer r.Unlock()
	r.MachineConfig.PrevReconcileUpd["crio"] = MachineConfigInfo{
		op: "delete",
	}

	return nil
}

// fetchCrioProfConf is for fetching the CRI-O MC CR created
// by this controller for enabling profiling
func (r *MachineConfigReconciler) fetchCrioProfConf(ctx context.Context, namespace types.NamespacedName) (*mcv1.MachineConfig, error) {
	criomc := &mcv1.MachineConfig{}
	if err := r.ClientGet(ctx, namespace, criomc); err != nil {
		return nil, err
	}
	return criomc, nil
}

// createCrioProfConf is for creating CRI-O MC CR
func (r *MachineConfigReconciler) createCrioProfConf(ctx context.Context) error {
	criomc, err := r.getCrioConfig()
	if err != nil {
		return err
	}

	if err := ctrlutil.SetControllerReference(r.CtrlConfig, criomc, r.Scheme); err != nil {
		return fmt.Errorf("failed to update owner info in CRI-O profiling MC resource: %w", err)
	}

	if err := r.ClientCreate(ctx, criomc); err != nil {
		return fmt.Errorf("failed to create crio profiling config %s: %w", criomc.Name, err)
	}

	r.Log.V(1).Info("Successfully created CRI-O MC for enabling profiling", "CrioProfilingConfigName", CrioProfilingConfigName)
	return nil
}

// deleteCrioProfConf is for deleting CRI-O MC CR
func (r *MachineConfigReconciler) deleteCrioProfConf(ctx context.Context, criomc *mcv1.MachineConfig) error {
	if err := r.ClientDelete(ctx, criomc); err != nil {
		return fmt.Errorf("failed to remove crio profiling config %s: %w", criomc.Name, err)
	}

	r.Log.V(1).Info("Successfully removed CRI-O MC to disable profiling", "CrioProfilingConfigName", CrioProfilingConfigName)
	return nil
}

// getCrioIgnitionConfig returns the required ignition config
// required for creating CRI-O MC CR
func getCrioIgnitionConfig() igntypes.Config {
	dropins := []igntypes.Dropin{
		{
			Name:     CrioUnixSocketConfFile,
			Contents: ignutil.StrToPtr(CrioUnixSocketConfData),
		},
	}

	units := []igntypes.Unit{
		{
			Dropins: dropins,
			Name:    CrioServiceFile,
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
func (r *MachineConfigReconciler) getCrioConfig() (*mcv1.MachineConfig, error) {
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
			Labels: MachineConfigLabels,
		},
		Spec: mcv1.MachineConfigSpec{
			Config: rawExt,
		},
	}, nil
}
