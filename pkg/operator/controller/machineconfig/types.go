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

// NodeSyncData is for storing the state
// of node operations made for enabling profiling
type NodeSyncData struct {
	PrevReconcileUpd map[string]LabelInfo
}

// LabelInfo is storing for the label changes
// made to the nodes
type LabelInfo struct {
	key   string
	value string
	op    patchOp
}

// ResourcePatchValue is for creating the patch
// request for updating a resource
type ResourcePatchValue struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// MachineConfigSyncData is for storing the state
// of the MC created for enabling profiling of
// requested services
type MachineConfigSyncData struct {
	PrevReconcileUpd map[string]MachineConfigInfo
}

// MachineConfigInfo is for storing the state
// data of MC operations
type MachineConfigInfo struct {
	op     string
	config interface{}
}

// patchOp is defined for patch operation type
type patchOp int
