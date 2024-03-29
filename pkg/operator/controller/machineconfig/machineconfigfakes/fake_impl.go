// Code generated by counterfeiter. DO NOT EDIT.
package machineconfigfakes

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeImpl struct {
	ClientCreateStub        func(context.Context, client.Object, ...client.CreateOption) error
	clientCreateMutex       sync.RWMutex
	clientCreateArgsForCall []struct {
		arg1 context.Context
		arg2 client.Object
		arg3 []client.CreateOption
	}
	clientCreateReturns struct {
		result1 error
	}
	clientCreateReturnsOnCall map[int]struct {
		result1 error
	}
	ClientDeleteStub        func(context.Context, client.Object, ...client.DeleteOption) error
	clientDeleteMutex       sync.RWMutex
	clientDeleteArgsForCall []struct {
		arg1 context.Context
		arg2 client.Object
		arg3 []client.DeleteOption
	}
	clientDeleteReturns struct {
		result1 error
	}
	clientDeleteReturnsOnCall map[int]struct {
		result1 error
	}
	ClientGetStub        func(context.Context, types.NamespacedName, client.Object) error
	clientGetMutex       sync.RWMutex
	clientGetArgsForCall []struct {
		arg1 context.Context
		arg2 types.NamespacedName
		arg3 client.Object
	}
	clientGetReturns struct {
		result1 error
	}
	clientGetReturnsOnCall map[int]struct {
		result1 error
	}
	ClientListStub        func(context.Context, client.ObjectList, ...client.ListOption) error
	clientListMutex       sync.RWMutex
	clientListArgsForCall []struct {
		arg1 context.Context
		arg2 client.ObjectList
		arg3 []client.ListOption
	}
	clientListReturns struct {
		result1 error
	}
	clientListReturnsOnCall map[int]struct {
		result1 error
	}
	ClientPatchStub        func(context.Context, client.Object, client.Patch, ...client.PatchOption) error
	clientPatchMutex       sync.RWMutex
	clientPatchArgsForCall []struct {
		arg1 context.Context
		arg2 client.Object
		arg3 client.Patch
		arg4 []client.PatchOption
	}
	clientPatchReturns struct {
		result1 error
	}
	clientPatchReturnsOnCall map[int]struct {
		result1 error
	}
	ClientStatusUpdateStub        func(context.Context, client.Object, ...client.UpdateOption) error
	clientStatusUpdateMutex       sync.RWMutex
	clientStatusUpdateArgsForCall []struct {
		arg1 context.Context
		arg2 client.Object
		arg3 []client.UpdateOption
	}
	clientStatusUpdateReturns struct {
		result1 error
	}
	clientStatusUpdateReturnsOnCall map[int]struct {
		result1 error
	}
	ClientUpdateStub        func(context.Context, client.Object, ...client.UpdateOption) error
	clientUpdateMutex       sync.RWMutex
	clientUpdateArgsForCall []struct {
		arg1 context.Context
		arg2 client.Object
		arg3 []client.UpdateOption
	}
	clientUpdateReturns struct {
		result1 error
	}
	clientUpdateReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeImpl) ClientCreate(arg1 context.Context, arg2 client.Object, arg3 ...client.CreateOption) error {
	fake.clientCreateMutex.Lock()
	ret, specificReturn := fake.clientCreateReturnsOnCall[len(fake.clientCreateArgsForCall)]
	fake.clientCreateArgsForCall = append(fake.clientCreateArgsForCall, struct {
		arg1 context.Context
		arg2 client.Object
		arg3 []client.CreateOption
	}{arg1, arg2, arg3})
	stub := fake.ClientCreateStub
	fakeReturns := fake.clientCreateReturns
	fake.recordInvocation("ClientCreate", []interface{}{arg1, arg2, arg3})
	fake.clientCreateMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeImpl) ClientCreateCallCount() int {
	fake.clientCreateMutex.RLock()
	defer fake.clientCreateMutex.RUnlock()
	return len(fake.clientCreateArgsForCall)
}

func (fake *FakeImpl) ClientCreateCalls(stub func(context.Context, client.Object, ...client.CreateOption) error) {
	fake.clientCreateMutex.Lock()
	defer fake.clientCreateMutex.Unlock()
	fake.ClientCreateStub = stub
}

func (fake *FakeImpl) ClientCreateArgsForCall(i int) (context.Context, client.Object, []client.CreateOption) {
	fake.clientCreateMutex.RLock()
	defer fake.clientCreateMutex.RUnlock()
	argsForCall := fake.clientCreateArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeImpl) ClientCreateReturns(result1 error) {
	fake.clientCreateMutex.Lock()
	defer fake.clientCreateMutex.Unlock()
	fake.ClientCreateStub = nil
	fake.clientCreateReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientCreateReturnsOnCall(i int, result1 error) {
	fake.clientCreateMutex.Lock()
	defer fake.clientCreateMutex.Unlock()
	fake.ClientCreateStub = nil
	if fake.clientCreateReturnsOnCall == nil {
		fake.clientCreateReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.clientCreateReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientDelete(arg1 context.Context, arg2 client.Object, arg3 ...client.DeleteOption) error {
	fake.clientDeleteMutex.Lock()
	ret, specificReturn := fake.clientDeleteReturnsOnCall[len(fake.clientDeleteArgsForCall)]
	fake.clientDeleteArgsForCall = append(fake.clientDeleteArgsForCall, struct {
		arg1 context.Context
		arg2 client.Object
		arg3 []client.DeleteOption
	}{arg1, arg2, arg3})
	stub := fake.ClientDeleteStub
	fakeReturns := fake.clientDeleteReturns
	fake.recordInvocation("ClientDelete", []interface{}{arg1, arg2, arg3})
	fake.clientDeleteMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeImpl) ClientDeleteCallCount() int {
	fake.clientDeleteMutex.RLock()
	defer fake.clientDeleteMutex.RUnlock()
	return len(fake.clientDeleteArgsForCall)
}

func (fake *FakeImpl) ClientDeleteCalls(stub func(context.Context, client.Object, ...client.DeleteOption) error) {
	fake.clientDeleteMutex.Lock()
	defer fake.clientDeleteMutex.Unlock()
	fake.ClientDeleteStub = stub
}

func (fake *FakeImpl) ClientDeleteArgsForCall(i int) (context.Context, client.Object, []client.DeleteOption) {
	fake.clientDeleteMutex.RLock()
	defer fake.clientDeleteMutex.RUnlock()
	argsForCall := fake.clientDeleteArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeImpl) ClientDeleteReturns(result1 error) {
	fake.clientDeleteMutex.Lock()
	defer fake.clientDeleteMutex.Unlock()
	fake.ClientDeleteStub = nil
	fake.clientDeleteReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientDeleteReturnsOnCall(i int, result1 error) {
	fake.clientDeleteMutex.Lock()
	defer fake.clientDeleteMutex.Unlock()
	fake.ClientDeleteStub = nil
	if fake.clientDeleteReturnsOnCall == nil {
		fake.clientDeleteReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.clientDeleteReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientGet(arg1 context.Context, arg2 types.NamespacedName, arg3 client.Object) error {
	fake.clientGetMutex.Lock()
	ret, specificReturn := fake.clientGetReturnsOnCall[len(fake.clientGetArgsForCall)]
	fake.clientGetArgsForCall = append(fake.clientGetArgsForCall, struct {
		arg1 context.Context
		arg2 types.NamespacedName
		arg3 client.Object
	}{arg1, arg2, arg3})
	stub := fake.ClientGetStub
	fakeReturns := fake.clientGetReturns
	fake.recordInvocation("ClientGet", []interface{}{arg1, arg2, arg3})
	fake.clientGetMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeImpl) ClientGetCallCount() int {
	fake.clientGetMutex.RLock()
	defer fake.clientGetMutex.RUnlock()
	return len(fake.clientGetArgsForCall)
}

func (fake *FakeImpl) ClientGetCalls(stub func(context.Context, types.NamespacedName, client.Object) error) {
	fake.clientGetMutex.Lock()
	defer fake.clientGetMutex.Unlock()
	fake.ClientGetStub = stub
}

func (fake *FakeImpl) ClientGetArgsForCall(i int) (context.Context, types.NamespacedName, client.Object) {
	fake.clientGetMutex.RLock()
	defer fake.clientGetMutex.RUnlock()
	argsForCall := fake.clientGetArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeImpl) ClientGetReturns(result1 error) {
	fake.clientGetMutex.Lock()
	defer fake.clientGetMutex.Unlock()
	fake.ClientGetStub = nil
	fake.clientGetReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientGetReturnsOnCall(i int, result1 error) {
	fake.clientGetMutex.Lock()
	defer fake.clientGetMutex.Unlock()
	fake.ClientGetStub = nil
	if fake.clientGetReturnsOnCall == nil {
		fake.clientGetReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.clientGetReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientList(arg1 context.Context, arg2 client.ObjectList, arg3 ...client.ListOption) error {
	fake.clientListMutex.Lock()
	ret, specificReturn := fake.clientListReturnsOnCall[len(fake.clientListArgsForCall)]
	fake.clientListArgsForCall = append(fake.clientListArgsForCall, struct {
		arg1 context.Context
		arg2 client.ObjectList
		arg3 []client.ListOption
	}{arg1, arg2, arg3})
	stub := fake.ClientListStub
	fakeReturns := fake.clientListReturns
	fake.recordInvocation("ClientList", []interface{}{arg1, arg2, arg3})
	fake.clientListMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeImpl) ClientListCallCount() int {
	fake.clientListMutex.RLock()
	defer fake.clientListMutex.RUnlock()
	return len(fake.clientListArgsForCall)
}

func (fake *FakeImpl) ClientListCalls(stub func(context.Context, client.ObjectList, ...client.ListOption) error) {
	fake.clientListMutex.Lock()
	defer fake.clientListMutex.Unlock()
	fake.ClientListStub = stub
}

func (fake *FakeImpl) ClientListArgsForCall(i int) (context.Context, client.ObjectList, []client.ListOption) {
	fake.clientListMutex.RLock()
	defer fake.clientListMutex.RUnlock()
	argsForCall := fake.clientListArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeImpl) ClientListReturns(result1 error) {
	fake.clientListMutex.Lock()
	defer fake.clientListMutex.Unlock()
	fake.ClientListStub = nil
	fake.clientListReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientListReturnsOnCall(i int, result1 error) {
	fake.clientListMutex.Lock()
	defer fake.clientListMutex.Unlock()
	fake.ClientListStub = nil
	if fake.clientListReturnsOnCall == nil {
		fake.clientListReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.clientListReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientPatch(arg1 context.Context, arg2 client.Object, arg3 client.Patch, arg4 ...client.PatchOption) error {
	fake.clientPatchMutex.Lock()
	ret, specificReturn := fake.clientPatchReturnsOnCall[len(fake.clientPatchArgsForCall)]
	fake.clientPatchArgsForCall = append(fake.clientPatchArgsForCall, struct {
		arg1 context.Context
		arg2 client.Object
		arg3 client.Patch
		arg4 []client.PatchOption
	}{arg1, arg2, arg3, arg4})
	stub := fake.ClientPatchStub
	fakeReturns := fake.clientPatchReturns
	fake.recordInvocation("ClientPatch", []interface{}{arg1, arg2, arg3, arg4})
	fake.clientPatchMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeImpl) ClientPatchCallCount() int {
	fake.clientPatchMutex.RLock()
	defer fake.clientPatchMutex.RUnlock()
	return len(fake.clientPatchArgsForCall)
}

func (fake *FakeImpl) ClientPatchCalls(stub func(context.Context, client.Object, client.Patch, ...client.PatchOption) error) {
	fake.clientPatchMutex.Lock()
	defer fake.clientPatchMutex.Unlock()
	fake.ClientPatchStub = stub
}

func (fake *FakeImpl) ClientPatchArgsForCall(i int) (context.Context, client.Object, client.Patch, []client.PatchOption) {
	fake.clientPatchMutex.RLock()
	defer fake.clientPatchMutex.RUnlock()
	argsForCall := fake.clientPatchArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeImpl) ClientPatchReturns(result1 error) {
	fake.clientPatchMutex.Lock()
	defer fake.clientPatchMutex.Unlock()
	fake.ClientPatchStub = nil
	fake.clientPatchReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientPatchReturnsOnCall(i int, result1 error) {
	fake.clientPatchMutex.Lock()
	defer fake.clientPatchMutex.Unlock()
	fake.ClientPatchStub = nil
	if fake.clientPatchReturnsOnCall == nil {
		fake.clientPatchReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.clientPatchReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientStatusUpdate(arg1 context.Context, arg2 client.Object, arg3 ...client.UpdateOption) error {
	fake.clientStatusUpdateMutex.Lock()
	ret, specificReturn := fake.clientStatusUpdateReturnsOnCall[len(fake.clientStatusUpdateArgsForCall)]
	fake.clientStatusUpdateArgsForCall = append(fake.clientStatusUpdateArgsForCall, struct {
		arg1 context.Context
		arg2 client.Object
		arg3 []client.UpdateOption
	}{arg1, arg2, arg3})
	stub := fake.ClientStatusUpdateStub
	fakeReturns := fake.clientStatusUpdateReturns
	fake.recordInvocation("ClientStatusUpdate", []interface{}{arg1, arg2, arg3})
	fake.clientStatusUpdateMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeImpl) ClientStatusUpdateCallCount() int {
	fake.clientStatusUpdateMutex.RLock()
	defer fake.clientStatusUpdateMutex.RUnlock()
	return len(fake.clientStatusUpdateArgsForCall)
}

func (fake *FakeImpl) ClientStatusUpdateCalls(stub func(context.Context, client.Object, ...client.UpdateOption) error) {
	fake.clientStatusUpdateMutex.Lock()
	defer fake.clientStatusUpdateMutex.Unlock()
	fake.ClientStatusUpdateStub = stub
}

func (fake *FakeImpl) ClientStatusUpdateArgsForCall(i int) (context.Context, client.Object, []client.UpdateOption) {
	fake.clientStatusUpdateMutex.RLock()
	defer fake.clientStatusUpdateMutex.RUnlock()
	argsForCall := fake.clientStatusUpdateArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeImpl) ClientStatusUpdateReturns(result1 error) {
	fake.clientStatusUpdateMutex.Lock()
	defer fake.clientStatusUpdateMutex.Unlock()
	fake.ClientStatusUpdateStub = nil
	fake.clientStatusUpdateReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientStatusUpdateReturnsOnCall(i int, result1 error) {
	fake.clientStatusUpdateMutex.Lock()
	defer fake.clientStatusUpdateMutex.Unlock()
	fake.ClientStatusUpdateStub = nil
	if fake.clientStatusUpdateReturnsOnCall == nil {
		fake.clientStatusUpdateReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.clientStatusUpdateReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientUpdate(arg1 context.Context, arg2 client.Object, arg3 ...client.UpdateOption) error {
	fake.clientUpdateMutex.Lock()
	ret, specificReturn := fake.clientUpdateReturnsOnCall[len(fake.clientUpdateArgsForCall)]
	fake.clientUpdateArgsForCall = append(fake.clientUpdateArgsForCall, struct {
		arg1 context.Context
		arg2 client.Object
		arg3 []client.UpdateOption
	}{arg1, arg2, arg3})
	stub := fake.ClientUpdateStub
	fakeReturns := fake.clientUpdateReturns
	fake.recordInvocation("ClientUpdate", []interface{}{arg1, arg2, arg3})
	fake.clientUpdateMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeImpl) ClientUpdateCallCount() int {
	fake.clientUpdateMutex.RLock()
	defer fake.clientUpdateMutex.RUnlock()
	return len(fake.clientUpdateArgsForCall)
}

func (fake *FakeImpl) ClientUpdateCalls(stub func(context.Context, client.Object, ...client.UpdateOption) error) {
	fake.clientUpdateMutex.Lock()
	defer fake.clientUpdateMutex.Unlock()
	fake.ClientUpdateStub = stub
}

func (fake *FakeImpl) ClientUpdateArgsForCall(i int) (context.Context, client.Object, []client.UpdateOption) {
	fake.clientUpdateMutex.RLock()
	defer fake.clientUpdateMutex.RUnlock()
	argsForCall := fake.clientUpdateArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeImpl) ClientUpdateReturns(result1 error) {
	fake.clientUpdateMutex.Lock()
	defer fake.clientUpdateMutex.Unlock()
	fake.ClientUpdateStub = nil
	fake.clientUpdateReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) ClientUpdateReturnsOnCall(i int, result1 error) {
	fake.clientUpdateMutex.Lock()
	defer fake.clientUpdateMutex.Unlock()
	fake.ClientUpdateStub = nil
	if fake.clientUpdateReturnsOnCall == nil {
		fake.clientUpdateReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.clientUpdateReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeImpl) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.clientCreateMutex.RLock()
	defer fake.clientCreateMutex.RUnlock()
	fake.clientDeleteMutex.RLock()
	defer fake.clientDeleteMutex.RUnlock()
	fake.clientGetMutex.RLock()
	defer fake.clientGetMutex.RUnlock()
	fake.clientListMutex.RLock()
	defer fake.clientListMutex.RUnlock()
	fake.clientPatchMutex.RLock()
	defer fake.clientPatchMutex.RUnlock()
	fake.clientStatusUpdateMutex.RLock()
	defer fake.clientStatusUpdateMutex.RUnlock()
	fake.clientUpdateMutex.RLock()
	defer fake.clientUpdateMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeImpl) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}
