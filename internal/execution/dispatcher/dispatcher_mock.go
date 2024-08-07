// Code generated by mockery v2.43.2. DO NOT EDIT.

package dispatcher

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockDispatcher is an autogenerated mock type for the Dispatcher type
type MockDispatcher struct {
	mock.Mock
}

type MockDispatcher_Expecter struct {
	mock *mock.Mock
}

func (_m *MockDispatcher) EXPECT() *MockDispatcher_Expecter {
	return &MockDispatcher_Expecter{mock: &_m.Mock}
}

// Send provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockDispatcher) Send(_a0 context.Context, _a1 string, _a2 map[string]interface{}) (map[string]interface{}, error) {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for Send")
	}

	var r0 map[string]interface{}
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]interface{}) (map[string]interface{}, error)); ok {
		return rf(_a0, _a1, _a2)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]interface{}) map[string]interface{}); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]interface{})
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, map[string]interface{}) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockDispatcher_Send_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Send'
type MockDispatcher_Send_Call struct {
	*mock.Call
}

// Send is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 string
//   - _a2 map[string]interface{}
func (_e *MockDispatcher_Expecter) Send(_a0 interface{}, _a1 interface{}, _a2 interface{}) *MockDispatcher_Send_Call {
	return &MockDispatcher_Send_Call{Call: _e.mock.On("Send", _a0, _a1, _a2)}
}

func (_c *MockDispatcher_Send_Call) Run(run func(_a0 context.Context, _a1 string, _a2 map[string]interface{})) *MockDispatcher_Send_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(map[string]interface{}))
	})
	return _c
}

func (_c *MockDispatcher_Send_Call) Return(_a0 map[string]interface{}, _a1 error) *MockDispatcher_Send_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockDispatcher_Send_Call) RunAndReturn(run func(context.Context, string, map[string]interface{}) (map[string]interface{}, error)) *MockDispatcher_Send_Call {
	_c.Call.Return(run)
	return _c
}

// Shutdown provides a mock function with given fields: _a0
func (_m *MockDispatcher) Shutdown(_a0 context.Context) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Shutdown")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockDispatcher_Shutdown_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Shutdown'
type MockDispatcher_Shutdown_Call struct {
	*mock.Call
}

// Shutdown is a helper method to define mock.On call
//   - _a0 context.Context
func (_e *MockDispatcher_Expecter) Shutdown(_a0 interface{}) *MockDispatcher_Shutdown_Call {
	return &MockDispatcher_Shutdown_Call{Call: _e.mock.On("Shutdown", _a0)}
}

func (_c *MockDispatcher_Shutdown_Call) Run(run func(_a0 context.Context)) *MockDispatcher_Shutdown_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockDispatcher_Shutdown_Call) Return(_a0 error) *MockDispatcher_Shutdown_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockDispatcher_Shutdown_Call) RunAndReturn(run func(context.Context) error) *MockDispatcher_Shutdown_Call {
	_c.Call.Return(run)
	return _c
}

// Start provides a mock function with given fields: _a0
func (_m *MockDispatcher) Start(_a0 context.Context) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Start")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockDispatcher_Start_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Start'
type MockDispatcher_Start_Call struct {
	*mock.Call
}

// Start is a helper method to define mock.On call
//   - _a0 context.Context
func (_e *MockDispatcher_Expecter) Start(_a0 interface{}) *MockDispatcher_Start_Call {
	return &MockDispatcher_Start_Call{Call: _e.mock.On("Start", _a0)}
}

func (_c *MockDispatcher_Start_Call) Run(run func(_a0 context.Context)) *MockDispatcher_Start_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockDispatcher_Start_Call) Return(_a0 error) *MockDispatcher_Start_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockDispatcher_Start_Call) RunAndReturn(run func(context.Context) error) *MockDispatcher_Start_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockDispatcher creates a new instance of MockDispatcher. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockDispatcher(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockDispatcher {
	mock := &MockDispatcher{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
