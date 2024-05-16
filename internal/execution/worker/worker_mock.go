// Code generated by mockery v2.43.0. DO NOT EDIT.

package worker

import (
	context "context"
	time "time"

	mock "github.com/stretchr/testify/mock"
)

// MockWorker is an autogenerated mock type for the Worker type
type MockWorker[I interface{}, O interface{}] struct {
	mock.Mock
}

type MockWorker_Expecter[I interface{}, O interface{}] struct {
	mock *mock.Mock
}

func (_m *MockWorker[I, O]) EXPECT() *MockWorker_Expecter[I, O] {
	return &MockWorker_Expecter[I, O]{mock: &_m.Mock}
}

// Exit provides a mock function with given fields:
func (_m *MockWorker[I, O]) Exit() <-chan ExitEvent {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Exit")
	}

	var r0 <-chan ExitEvent
	if rf, ok := ret.Get(0).(func() <-chan ExitEvent); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan ExitEvent)
		}
	}

	return r0
}

// MockWorker_Exit_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Exit'
type MockWorker_Exit_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Exit is a helper method to define mock.On call
func (_e *MockWorker_Expecter[I, O]) Exit() *MockWorker_Exit_Call[I, O] {
	return &MockWorker_Exit_Call[I, O]{Call: _e.mock.On("Exit")}
}

func (_c *MockWorker_Exit_Call[I, O]) Run(run func()) *MockWorker_Exit_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWorker_Exit_Call[I, O]) Return(_a0 <-chan ExitEvent) *MockWorker_Exit_Call[I, O] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockWorker_Exit_Call[I, O]) RunAndReturn(run func() <-chan ExitEvent) *MockWorker_Exit_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// Read provides a mock function with given fields: _a0, _a1
func (_m *MockWorker[I, O]) Read(_a0 context.Context, _a1 ReadConfig) (O, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Read")
	}

	var r0 O
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, ReadConfig) (O, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, ReadConfig) O); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Get(0).(O)
	}

	if rf, ok := ret.Get(1).(func(context.Context, ReadConfig) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWorker_Read_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Read'
type MockWorker_Read_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Read is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 ReadConfig
func (_e *MockWorker_Expecter[I, O]) Read(_a0 interface{}, _a1 interface{}) *MockWorker_Read_Call[I, O] {
	return &MockWorker_Read_Call[I, O]{Call: _e.mock.On("Read", _a0, _a1)}
}

func (_c *MockWorker_Read_Call[I, O]) Run(run func(_a0 context.Context, _a1 ReadConfig)) *MockWorker_Read_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(ReadConfig))
	})
	return _c
}

func (_c *MockWorker_Read_Call[I, O]) Return(_a0 O, _a1 error) *MockWorker_Read_Call[I, O] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWorker_Read_Call[I, O]) RunAndReturn(run func(context.Context, ReadConfig) (O, error)) *MockWorker_Read_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// Send provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockWorker[I, O]) Send(_a0 context.Context, _a1 I, _a2 SendConfig) (O, error) {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for Send")
	}

	var r0 O
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, I, SendConfig) (O, error)); ok {
		return rf(_a0, _a1, _a2)
	}
	if rf, ok := ret.Get(0).(func(context.Context, I, SendConfig) O); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Get(0).(O)
	}

	if rf, ok := ret.Get(1).(func(context.Context, I, SendConfig) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWorker_Send_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Send'
type MockWorker_Send_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Send is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 I
//   - _a2 SendConfig
func (_e *MockWorker_Expecter[I, O]) Send(_a0 interface{}, _a1 interface{}, _a2 interface{}) *MockWorker_Send_Call[I, O] {
	return &MockWorker_Send_Call[I, O]{Call: _e.mock.On("Send", _a0, _a1, _a2)}
}

func (_c *MockWorker_Send_Call[I, O]) Run(run func(_a0 context.Context, _a1 I, _a2 SendConfig)) *MockWorker_Send_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(I), args[2].(SendConfig))
	})
	return _c
}

func (_c *MockWorker_Send_Call[I, O]) Return(_a0 O, _a1 error) *MockWorker_Send_Call[I, O] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWorker_Send_Call[I, O]) RunAndReturn(run func(context.Context, I, SendConfig) (O, error)) *MockWorker_Send_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// Start provides a mock function with given fields: _a0, _a1
func (_m *MockWorker[I, O]) Start(_a0 context.Context, _a1 StartConfig) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Start")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, StartConfig) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockWorker_Start_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Start'
type MockWorker_Start_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Start is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 StartConfig
func (_e *MockWorker_Expecter[I, O]) Start(_a0 interface{}, _a1 interface{}) *MockWorker_Start_Call[I, O] {
	return &MockWorker_Start_Call[I, O]{Call: _e.mock.On("Start", _a0, _a1)}
}

func (_c *MockWorker_Start_Call[I, O]) Run(run func(_a0 context.Context, _a1 StartConfig)) *MockWorker_Start_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(StartConfig))
	})
	return _c
}

func (_c *MockWorker_Start_Call[I, O]) Return(_a0 error) *MockWorker_Start_Call[I, O] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockWorker_Start_Call[I, O]) RunAndReturn(run func(context.Context, StartConfig) error) *MockWorker_Start_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// Terminate provides a mock function with given fields:
func (_m *MockWorker[I, O]) Terminate() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Terminate")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockWorker_Terminate_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Terminate'
type MockWorker_Terminate_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Terminate is a helper method to define mock.On call
func (_e *MockWorker_Expecter[I, O]) Terminate() *MockWorker_Terminate_Call[I, O] {
	return &MockWorker_Terminate_Call[I, O]{Call: _e.mock.On("Terminate")}
}

func (_c *MockWorker_Terminate_Call[I, O]) Run(run func()) *MockWorker_Terminate_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWorker_Terminate_Call[I, O]) Return(_a0 error) *MockWorker_Terminate_Call[I, O] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockWorker_Terminate_Call[I, O]) RunAndReturn(run func() error) *MockWorker_Terminate_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// Wait provides a mock function with given fields: _a0
func (_m *MockWorker[I, O]) Wait(_a0 context.Context) (ExitEvent, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Wait")
	}

	var r0 ExitEvent
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (ExitEvent, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(context.Context) ExitEvent); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(ExitEvent)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWorker_Wait_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Wait'
type MockWorker_Wait_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Wait is a helper method to define mock.On call
//   - _a0 context.Context
func (_e *MockWorker_Expecter[I, O]) Wait(_a0 interface{}) *MockWorker_Wait_Call[I, O] {
	return &MockWorker_Wait_Call[I, O]{Call: _e.mock.On("Wait", _a0)}
}

func (_c *MockWorker_Wait_Call[I, O]) Run(run func(_a0 context.Context)) *MockWorker_Wait_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockWorker_Wait_Call[I, O]) Return(_a0 ExitEvent, _a1 error) *MockWorker_Wait_Call[I, O] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWorker_Wait_Call[I, O]) RunAndReturn(run func(context.Context) (ExitEvent, error)) *MockWorker_Wait_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// WaitFor provides a mock function with given fields: _a0, _a1
func (_m *MockWorker[I, O]) WaitFor(_a0 context.Context, _a1 time.Duration) (ExitEvent, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for WaitFor")
	}

	var r0 ExitEvent
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, time.Duration) (ExitEvent, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, time.Duration) ExitEvent); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Get(0).(ExitEvent)
	}

	if rf, ok := ret.Get(1).(func(context.Context, time.Duration) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWorker_WaitFor_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'WaitFor'
type MockWorker_WaitFor_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// WaitFor is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 time.Duration
func (_e *MockWorker_Expecter[I, O]) WaitFor(_a0 interface{}, _a1 interface{}) *MockWorker_WaitFor_Call[I, O] {
	return &MockWorker_WaitFor_Call[I, O]{Call: _e.mock.On("WaitFor", _a0, _a1)}
}

func (_c *MockWorker_WaitFor_Call[I, O]) Run(run func(_a0 context.Context, _a1 time.Duration)) *MockWorker_WaitFor_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(time.Duration))
	})
	return _c
}

func (_c *MockWorker_WaitFor_Call[I, O]) Return(_a0 ExitEvent, _a1 error) *MockWorker_WaitFor_Call[I, O] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWorker_WaitFor_Call[I, O]) RunAndReturn(run func(context.Context, time.Duration) (ExitEvent, error)) *MockWorker_WaitFor_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// Write provides a mock function with given fields: _a0, _a1
func (_m *MockWorker[I, O]) Write(_a0 context.Context, _a1 I) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Write")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, I) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockWorker_Write_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Write'
type MockWorker_Write_Call[I interface{}, O interface{}] struct {
	*mock.Call
}

// Write is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 I
func (_e *MockWorker_Expecter[I, O]) Write(_a0 interface{}, _a1 interface{}) *MockWorker_Write_Call[I, O] {
	return &MockWorker_Write_Call[I, O]{Call: _e.mock.On("Write", _a0, _a1)}
}

func (_c *MockWorker_Write_Call[I, O]) Run(run func(_a0 context.Context, _a1 I)) *MockWorker_Write_Call[I, O] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(I))
	})
	return _c
}

func (_c *MockWorker_Write_Call[I, O]) Return(_a0 error) *MockWorker_Write_Call[I, O] {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockWorker_Write_Call[I, O]) RunAndReturn(run func(context.Context, I) error) *MockWorker_Write_Call[I, O] {
	_c.Call.Return(run)
	return _c
}

// NewMockWorker creates a new instance of MockWorker. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockWorker[I interface{}, O interface{}](t interface {
	mock.TestingT
	Cleanup(func())
}) *MockWorker[I, O] {
	mock := &MockWorker[I, O]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
