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
	"fmt"
	"time"
)

const (
	// finalizer name for nodeobservabilitymachineconfig resources
	finalizer = "NodeObservabilityMachineConfig"

	// defaultRequeueTime is the default reconcile requeue time
	defaultRequeueTime = time.Minute

	// empty is defined for empty string
	empty = ""

	// MCAPIVersion is the machine config API version
	MCAPIVersion = "machineconfiguration.openshift.io/v1"

	// MCKind is the machine config resource kind
	MCKind = "MachineConfig"

	// MCPoolKind is the machine config pool resource king
	MCPoolKind = "MachineConfigPool"

	// MCRoleLabelName is the machine config role label name
	MCRoleLabelName = "machineconfiguration.openshift.io/role"

	// NodeObservabilityNodeRoleLabelName is the role label name
	// used for enabling profiling of services on requested nodes
	NodeObservabilityNodeRoleLabelName = "node-role.kubernetes.io/nodeobservability"

	// NodeObservabilityNodeRoleName is the nodeobservability node role name
	NodeObservabilityNodeRoleName = "nodeobservability"

	// ProfilingMCPName is the name of the MCP created for
	// applying nodeobservability related MC changes on
	// nodes with nodeobservability role
	ProfilingMCPName = "nodeobservability"

	// ResourceLabelsPath is the path of Labels in resource
	ResourceLabelsPath = "/metadata/labels"

	// WorkerNodeMCPName is the name of the MCP created for
	// applying required MC changes on nodes with worker role
	WorkerNodeMCPName = "worker"

	// WorkerNodeRoleLabelName is the role label name used
	// for worker nodes
	WorkerNodeRoleLabelName = "node-role.kubernetes.io/worker"

	// WorkerNodeRoleName is the worker node role name
	WorkerNodeRoleName = "worker"
)

var (
	// CrioUnixSocketConfData contains the configuration required
	// for enabling CRI-O profiling
	CrioUnixSocketConfData = fmt.Sprintf(`[Service]
Environment="%s"`, CrioUnixSocketEnvString)

	// NodeSelectorLabels is for storing the labels to
	// match the nodes to include in MCP
	NodeSelectorLabels = map[string]string{
		NodeObservabilityNodeRoleLabelName: empty,
	}

	// MachineConfigLabels is for storing the labels to
	// add in machine config resources
	MachineConfigLabels = map[string]string{
		MCRoleLabelName: NodeObservabilityNodeRoleName,
	}
)
