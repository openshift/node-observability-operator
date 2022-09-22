// /*
// Copyright 2021.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package nodeobservabilitycontroller

import (
	"context"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/google/go-cmp/cmp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha2 "github.com/openshift/node-observability-operator/api/v1alpha2"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

func TestReconcile(t *testing.T) {
	managedTypesList := []client.ObjectList{
		&corev1.NamespaceList{},
		&appsv1.DaemonSetList{},
		&operatorv1alpha2.NodeObservabilityList{},
	}

	eventWaitTimeout := time.Duration(1 * time.Second)

	teAdd := test.Event{
		EventType: watch.Added,
		ObjType:   "daemonset",
		NamespacedName: types.NamespacedName{
			Namespace: "",
			Name:      daemonSetName,
		},
	}

	teMod := test.Event{
		EventType: watch.Modified,
		ObjType:   "nodeobservability",
		NamespacedName: types.NamespacedName{
			Namespace: "",
			Name:      "cluster",
		},
	}

	teDel := test.Event{
		EventType: watch.Deleted,
		ObjType:   "nodeobservability",
		NamespacedName: types.NamespacedName{
			Namespace: "",
			Name:      "cluster",
		},
	}

	testCases := []struct {
		name                 string
		existingObjects      []runtime.Object
		Image                string
		Labels               []string
		inputRequest         ctrl.Request
		expectedResult       reconcile.Result
		expectedEvents       []test.Event
		errExpected          bool
		expectReadyCondition metav1.ConditionStatus
	}{
		{
			name:                 "Bootstrapping",
			existingObjects:      []runtime.Object{testNodeObservability(), makeKubeletCACM()},
			inputRequest:         testRequest(),
			expectedResult:       reconcile.Result{},
			expectedEvents:       []test.Event{},
			expectReadyCondition: metav1.ConditionTrue,
		},
		{
			name:                 "Deleted",
			existingObjects:      []runtime.Object{makeKubeletCACM()},
			inputRequest:         testRequest(),
			expectedResult:       reconcile.Result{},
			expectReadyCondition: metav1.ConditionTrue,
		},
		{
			name:                 "Deleting",
			existingObjects:      []runtime.Object{testNodeObservabilityToBeDeleted(), makeKubeletCACM()},
			inputRequest:         testRequest(),
			expectedResult:       reconcile.Result{},
			expectReadyCondition: metav1.ConditionTrue,
		},
		{
			name:            "InvalidName",
			existingObjects: []runtime.Object{},
			inputRequest: ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "xxx",
				},
			},
			expectedResult:       reconcile.Result{},
			expectReadyCondition: metav1.ConditionFalse,
		},
	}

	// loop through test cases (bootstrap and delete)
	for _, tc := range testCases {
		// run the tests
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(test.Scheme).WithRuntimeObjects(tc.existingObjects...).Build()
			// for each run we clear the expected Events
			tc.expectedEvents = nil

			r := &NodeObservabilityReconciler{
				Client:            cl,
				ClusterWideClient: cl,
				Scheme:            test.Scheme,
				Log:               zap.New(zap.UseDevMode(true)),
				AgentImage:        "test",
			}

			// the add and modify events should only be added when there are no 'simulated' errors
			if tc.name == "Bootstrapping" {
				//update finalizer
				tc.expectedEvents = append(tc.expectedEvents, teMod)
				//add daemonset
				tc.expectedEvents = append(tc.expectedEvents, teAdd)
				//update status
				tc.expectedEvents = append(tc.expectedEvents, teMod)
			}
			if tc.name == "Deleting" {
				//update finalizer
				tc.expectedEvents = append(tc.expectedEvents, teDel)
			}
			if tc.name == "Bootstrapping" {
				//update finalizer
				tc.expectedEvents = append(tc.expectedEvents, teMod)
			}

			c := test.NewEventCollector(t, cl, managedTypesList, len(tc.expectedEvents))

			ctx := context.TODO()
			// get watch interfaces from all the types managed by the operator
			c.Start(ctx)
			defer c.Stop()

			// TEST FUNCTION
			gotResult, err := r.Reconcile(ctx, tc.inputRequest)

			res := &operatorv1alpha2.NodeObservability{}
			if err := cl.Get(ctx, tc.inputRequest.NamespacedName, res); err != nil && !kerrors.IsNotFound(err) {
				t.Fatalf("unexpected error while getting %v", tc.inputRequest.NamespacedName)
			}
			if err != nil { // nodeObs is found
				cond := res.Status.ConditionalStatus.GetCondition(operatorv1alpha2.DebugReady)
				if cond != nil {
					isReady := cond.Status
					if tc.expectReadyCondition != isReady {
						t.Fatalf("expecting condition DebugRead=%v but was DebugReady=%v", tc.expectReadyCondition, isReady)
					}
				}
			}

			// error check
			if err != nil {
				if !tc.errExpected {
					t.Fatalf("got unexpected error: %v", err)
				}
			} else if tc.errExpected {
				t.Fatalf("error expected but not received")
			}

			// result check
			if !reflect.DeepEqual(gotResult, tc.expectedResult) {
				t.Fatalf("expected result %v, got %v", tc.expectedResult, gotResult)
			}

			// collect the events received from Reconcile()
			collectedEvents := c.Collect(len(tc.expectedEvents), eventWaitTimeout)

			// compare collected and expected events
			idxExpectedEvents := test.IndexEvents(tc.expectedEvents)
			idxCollectedEvents := test.IndexEvents(collectedEvents)
			if diff := cmp.Diff(idxExpectedEvents, idxCollectedEvents); diff != "" {
				t.Fatalf("found diff between expected and collected events: %s", diff)
			}
		})
	}
}

func TestIsClusterNodeObservability(t *testing.T) {

	testCases := []struct {
		name            string
		existingObjects []runtime.Object
		nodeObsToTest   *operatorv1alpha2.NodeObservability
		errExpected     bool
	}{
		{
			name:            "new NodeObservability cluster passes",
			existingObjects: []runtime.Object{},
			nodeObsToTest:   testNodeObservability(),
			errExpected:     false,
		},
		{
			name:            "new NodeObservability xxx fails",
			existingObjects: []runtime.Object{testNodeObservability()},
			nodeObsToTest:   testNodeObservabilityInvalidName(),
			errExpected:     true,
		},
		{
			name:            "NodeObservability cluster, in update, passes",
			existingObjects: []runtime.Object{testNodeObservability()},
			nodeObsToTest:   testNodeObservability(),
			errExpected:     false,
		},
		{
			name:            "NodeObservability xxx, with existing NodeObservability, fails",
			existingObjects: []runtime.Object{testNodeObservability()},
			nodeObsToTest:   testNodeObservabilityInvalidName(),
			errExpected:     true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := isClusterNodeObservability(context.TODO(), tc.nodeObsToTest)
			if err != nil && !tc.errExpected {
				t.Fatalf("unexpected error : %v", err)
			}
			if tc.errExpected && err == nil {
				t.Fatal("expecting error but found none")
			}
		})
	}
}

// testRquest - used to create request
func testRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "",
			Name:      "cluster",
		},
	}
}

// testNodeObservability - minimal CR for the test
func testNodeObservability() *operatorv1alpha2.NodeObservability {
	return &operatorv1alpha2.NodeObservability{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "",
			Name:      "cluster",
		},
	}
}

// testNodeObservabilityDeleted - test for deletion
func testNodeObservabilityToBeDeleted() *operatorv1alpha2.NodeObservability {
	nobs := testNodeObservability()
	nobs.Finalizers = append(nobs.Finalizers, finalizer)
	nobs.DeletionTimestamp = &metav1.Time{
		Time: time.Now(),
	}
	return nobs
}

func testNodeObservabilityInvalidName() *operatorv1alpha2.NodeObservability {
	o := testNodeObservability()
	o.Name = "xxx"
	return o
}
