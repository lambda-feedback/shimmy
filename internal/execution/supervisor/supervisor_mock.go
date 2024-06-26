// Code generated by mockery v2.43.0. DO NOT EDIT.

package supervisor

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockSupervisor is an autogenerated mock type for the Supervisor type
type MockSupervisor[I interface{}, O interface{}] struct {
	mock.Mock
}

type MockSupervisor_Expecter[I interface{}, O interface{}] struct {
	mock *mock.Mock
}

func (_m *MockSupervisor[I, O]) EXPECT() *MockSupervisor_Expecter[I, O] {
	return &MockSupervisor_Expecter[I, O]{mock: &_m.Mock}
}

// Send provides a mock function with given fields: ctx, data
func (_m *MockSupervisor[I, O]) Send(ctx context.Context, data I) (*Result[O], error) {
	ret := _m.Called(ctx, data)

	if len(ret) == 0 {
		panic("no return value specified for Send")
	}

	var r0 *Result[O]
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, I) (*Result[O], error)); ok {
		return rf(ctx, data)
	}
	if rf, ok := ret.Get(0).(func(context.Context, I) *Result[O]); ok {
		r0 = rf(ctx, data)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*Result[O])
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, I) error); ok {
		r1 = rf(ctx, data)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSupervisor_Send_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Send'
type MockSupervisor_Send_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Send is a helper method to define mock.On call
//   - ctx context.Context
//   - data I
func (_e *MockSupervisor_Expecter[I, O]) Send(ctx interface{}, data interface{}) *MockSupervisor_Send_Call[I, O] {
	return &MockSupervisor_Send_Call[I, O]{Call: _e.mock.On("Send", ctx, data)}
}

func (_c *MockSupervisor_Send_Call[I, O]) Run(run func(ctx context.Context, data I)) *MockSupervisor_Send_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(I))
	})
	return _c
}

func (_c *MockSupervisor_Send_Call[I, O]) Return(_a0 *Result[O], _a1 error) *MockSupervisor_Send_Call[I, O] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSupervisor_Send_Call[I, O]) RunAndReturn(run func(context.Context, I) (*Result[O], error)) *MockSupervisor_Send_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// Shutdown provides a mock function with given fields: ctx
func (_m *MockSupervisor[I, O]) Shutdown(ctx context.Context) (WaitFunc, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Shutdown")
	}

	var r0 WaitFunc
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (WaitFunc, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) WaitFunc); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(WaitFunc)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSupervisor_Shutdown_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Shutdown'
type MockSupervisor_Shutdown_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Shutdown is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockSupervisor_Expecter[I, O]) Shutdown(ctx interface{}) *MockSupervisor_Shutdown_Call[I, O] {
	return &MockSupervisor_Shutdown_Call[I, O]{Call: _e.mock.On("Shutdown", ctx)}
}

func (_c *MockSupervisor_Shutdown_Call[I, O]) Run(run func(ctx context.Context)) *MockSupervisor_Shutdown_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockSupervisor_Shutdown_Call[I, O]) Return(_a0 WaitFunc, _a1 error) *MockSupervisor_Shutdown_Call[I, O] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSupervisor_Shutdown_Call[I, O]) RunAndReturn(run func(context.Context) (WaitFunc, error)) *MockSupervisor_Shutdown_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// Start provides a mock function with given fields: ctx
func (_m *MockSupervisor[I, O]) Start(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Start")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSupervisor_Start_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Start'
type MockSupervisor_Start_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Start is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockSupervisor_Expecter[I, O]) Start(ctx interface{}) *MockSupervisor_Start_Call[I, O] {
	return &MockSupervisor_Start_Call[I, O]{Call: _e.mock.On("Start", ctx)}
}

func (_c *MockSupervisor_Start_Call[I, O]) Run(run func(ctx context.Context)) *MockSupervisor_Start_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockSupervisor_Start_Call[I, O]) Return(_a0 error) *MockSupervisor_Start_Call[I, O] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSupervisor_Start_Call[I, O]) RunAndReturn(run func(context.Context) error) *MockSupervisor_Start_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// Suspend provides a mock function with given fields: ctx
func (_m *MockSupervisor[I, O]) Suspend(ctx context.Context) (WaitFunc, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Suspend")
	}

	var r0 WaitFunc
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (WaitFunc, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) WaitFunc); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(WaitFunc)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockSupervisor_Suspend_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Suspend'
type MockSupervisor_Suspend_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Suspend is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockSupervisor_Expecter[I, O]) Suspend(ctx interface{}) *MockSupervisor_Suspend_Call[I, O] {
	return &MockSupervisor_Suspend_Call[I, O]{Call: _e.mock.On("Suspend", ctx)}
}

func (_c *MockSupervisor_Suspend_Call[I, O]) Run(run func(ctx context.Context)) *MockSupervisor_Suspend_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockSupervisor_Suspend_Call[I, O]) Return(_a0 WaitFunc, _a1 error) *MockSupervisor_Suspend_Call[I, O] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockSupervisor_Suspend_Call[I, O]) RunAndReturn(run func(context.Context) (WaitFunc, error)) *MockSupervisor_Suspend_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// NewMockSupervisor creates a new instance of MockSupervisor. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSupervisor[I interface{}, O interface{}](t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSupervisor[I, O] {
	mock := &MockSupervisor[I, O]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
