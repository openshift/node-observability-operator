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

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/openshift/node-observability-operator/api/v1alpha1"
	"github.com/openshift/node-observability-operator/pkg/operator/controller/test"
)

func TestReconcile(t *testing.T) {
	managedTypesList := []client.ObjectList{
		&corev1.NamespaceList{},
		&appsv1.DaemonSetList{},
		&operatorv1alpha1.NodeObservabilityList{},
	}

	eventWaitTimeout := time.Duration(1 * time.Second)

	// used to simulate errors for all objects
	ErrRuns := []ErrTestObject{
		{
			Set:      nil,
			NotFound: nil,
		},
		{
			Set: map[string]bool{
				sccObj: true,
			},
			NotFound: map[string]bool{
				sccObj: false,
			},
		},
		{
			Set: map[string]bool{
				saObj: true,
			},
			NotFound: map[string]bool{
				saObj: false,
			},
		},
		{
			Set: map[string]bool{
				crObj: true,
			},
			NotFound: map[string]bool{
				crObj: false,
			},
		},
		{
			Set: map[string]bool{
				crbObj: true,
			},
			NotFound: map[string]bool{
				crbObj: false,
			},
		},
		{
			Set: map[string]bool{
				dsObj: true,
			},
			NotFound: map[string]bool{
				dsObj: false,
			},
		},
		{
			Set: map[string]bool{
				sccObj: true,
			},
			NotFound: map[string]bool{
				sccObj: true,
			},
		},
		{
			Set: map[string]bool{
				saObj: true,
			},
			NotFound: map[string]bool{
				saObj: true,
			},
		},
		{
			Set: map[string]bool{
				crObj: true,
			},
			NotFound: map[string]bool{
				crObj: true,
			},
		},
		{
			Set: map[string]bool{
				crbObj: true,
			},
			NotFound: map[string]bool{
				crbObj: true,
			},
		},
		{
			Set: map[string]bool{
				dsObj: true,
			},
			NotFound: map[string]bool{
				dsObj: true,
			},
		},
	}

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
			Name:      "test",
		},
	}

	teDel := test.Event{
		EventType: watch.Deleted,
		ObjType:   "nodeobservability",
		NamespacedName: types.NamespacedName{
			Namespace: "",
			Name:      "test",
		},
	}

	testCases := []struct {
		name            string
		existingObjects []runtime.Object
		Image           string
		Labels          []string
		inputRequest    ctrl.Request
		expectedResult  reconcile.Result
		expectedEvents  []test.Event
		errExpected     bool
	}{
		{
			name:            "Bootstrapping",
			existingObjects: []runtime.Object{testNodeObservability(), makeKubeletCACM()},
			inputRequest:    testRequest(),
			expectedResult:  reconcile.Result{},
			expectedEvents:  []test.Event{},
		},
		{
			name:            "Deleted",
			existingObjects: []runtime.Object{makeKubeletCACM()},
			inputRequest:    testRequest(),
			expectedResult:  reconcile.Result{},
		},
		{
			name:            "Deleting",
			existingObjects: []runtime.Object{testNodeObservabilityToBeDeleted(), makeKubeletCACM()},
			inputRequest:    testRequest(),
			expectedResult:  reconcile.Result{},
		},
	}

	// loop through test cases (bootstrap and delete)
	for _, tc := range testCases {
		// loop through error objects
		for _, errTest := range ErrRuns {
			// run the tests
			t.Run(tc.name, func(t *testing.T) {
				cl := fake.NewClientBuilder().WithScheme(test.Scheme).WithRuntimeObjects(tc.existingObjects...).Build()
				// for each run we clear the expected Events
				tc.expectedEvents = nil

				r := &NodeObservabilityReconciler{
					Client: cl,
					Scheme: test.Scheme,
					Log:    zap.New(zap.UseDevMode(true)),
					Err:    errTest,
				}
				// only check for errors when the ErrorTestObject Set and NotFound maps are nill
				tc.errExpected = (errTest.Set != nil || errTest.NotFound != nil) && tc.name == "Bootstrapping"

				// the add and modify events should only be added when there are no 'simulated' errors
				if (errTest.Set == nil && errTest.NotFound == nil) && tc.name == "Bootstrapping" {
					//update finalizer
					tc.expectedEvents = append(tc.expectedEvents, teMod)
					//add daemonset
					tc.expectedEvents = append(tc.expectedEvents, teAdd)
					//update status
					tc.expectedEvents = append(tc.expectedEvents, teMod)
				}
				if /*(errTest.Set == nil && errTest.NotFound == nil) &&*/ tc.name == "Deleting" {
					//update finalizer
					tc.expectedEvents = append(tc.expectedEvents, teDel)
				}
				if (errTest.Set != nil || errTest.NotFound != nil) && tc.name == "Bootstrapping" {
					//update finalizer
					tc.expectedEvents = append(tc.expectedEvents, teMod)
				}

				// special case for daemonset
				if errTest.Set[dsObj] && tc.name == "Bootstrapping" {
					tc.expectedEvents = append(tc.expectedEvents, teAdd)
				}

				c := test.NewEventCollector(t, cl, managedTypesList, len(tc.expectedEvents))

				// get watch interfaces from all the types managed by the operator
				c.Start(context.TODO())
				defer c.Stop()

				// TEST FUNCTION
				gotResult, err := r.Reconcile(context.TODO(), tc.inputRequest)

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
}

// // testRquest - used to create request
func testRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "",
			Name:      "test",
		},
	}
}

// testNodeObservability - minimal CR for the test
func testNodeObservability() *operatorv1alpha1.NodeObservability {
	return &operatorv1alpha1.NodeObservability{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "",
			Name:      "test",
		},
		Spec: operatorv1alpha1.NodeObservabilitySpec{
			Image: "test",
		},
	}
}

func testNodeObservabilityToBeDeleted() *operatorv1alpha1.NodeObservability {
	nobs := testNodeObservability()
	nobs.Finalizers = append(nobs.Finalizers, finalizer)
	nobs.DeletionTimestamp = &metav1.Time{
		Time: time.Now(),
	}
	return nobs
}
