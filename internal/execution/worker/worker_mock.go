// Code generated by mockery v2.43.0. DO NOT EDIT.

package worker

import (
	context "context"
	io "io"

	mock "github.com/stretchr/testify/mock"

	time "time"
)

// MockWorker is an autogenerated mock type for the Worker type
type MockWorker struct {
	mock.Mock
}

type MockWorker_Expecter struct {
	mock *mock.Mock
}

func (_m *MockWorker) EXPECT() *MockWorker_Expecter {
	return &MockWorker_Expecter{mock: &_m.Mock}
}

// DuplexPipe provides a mock function with given fields:
func (_m *MockWorker) DuplexPipe() (io.ReadWriteCloser, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for DuplexPipe")
	}

	var r0 io.ReadWriteCloser
	var r1 error
	if rf, ok := ret.Get(0).(func() (io.ReadWriteCloser, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() io.ReadWriteCloser); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadWriteCloser)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWorker_DuplexPipe_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DuplexPipe'
type MockWorker_DuplexPipe_Call struct {
	*mock.Call
}

// DuplexPipe is a helper method to define mock.On call
func (_e *MockWorker_Expecter) DuplexPipe() *MockWorker_DuplexPipe_Call {
	return &MockWorker_DuplexPipe_Call{Call: _e.mock.On("DuplexPipe")}
}

func (_c *MockWorker_DuplexPipe_Call) Run(run func()) *MockWorker_DuplexPipe_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWorker_DuplexPipe_Call) Return(_a0 io.ReadWriteCloser, _a1 error) *MockWorker_DuplexPipe_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWorker_DuplexPipe_Call) RunAndReturn(run func() (io.ReadWriteCloser, error)) *MockWorker_DuplexPipe_Call {
	_c.Call.Return(run)
	return _c
}

// ReadPipe provides a mock function with given fields:
func (_m *MockWorker) ReadPipe() (io.ReadCloser, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ReadPipe")
	}

	var r0 io.ReadCloser
	var r1 error
	if rf, ok := ret.Get(0).(func() (io.ReadCloser, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() io.ReadCloser); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWorker_ReadPipe_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ReadPipe'
type MockWorker_ReadPipe_Call struct {
	*mock.Call
}

// ReadPipe is a helper method to define mock.On call
func (_e *MockWorker_Expecter) ReadPipe() *MockWorker_ReadPipe_Call {
	return &MockWorker_ReadPipe_Call{Call: _e.mock.On("ReadPipe")}
}

func (_c *MockWorker_ReadPipe_Call) Run(run func()) *MockWorker_ReadPipe_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWorker_ReadPipe_Call) Return(_a0 io.ReadCloser, _a1 error) *MockWorker_ReadPipe_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWorker_ReadPipe_Call) RunAndReturn(run func() (io.ReadCloser, error)) *MockWorker_ReadPipe_Call {
	_c.Call.Return(run)
	return _c
}

// Start provides a mock function with given fields: _a0
func (_m *MockWorker) Start(_a0 context.Context) error {
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

// MockWorker_Start_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Start'
type MockWorker_Start_Call struct {
	*mock.Call
}

// Start is a helper method to define mock.On call
//   - _a0 context.Context
func (_e *MockWorker_Expecter) Start(_a0 interface{}) *MockWorker_Start_Call {
	return &MockWorker_Start_Call{Call: _e.mock.On("Start", _a0)}
}

func (_c *MockWorker_Start_Call) Run(run func(_a0 context.Context)) *MockWorker_Start_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockWorker_Start_Call) Return(_a0 error) *MockWorker_Start_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockWorker_Start_Call) RunAndReturn(run func(context.Context) error) *MockWorker_Start_Call {
	_c.Call.Return(run)
	return _c
}

// Stop provides a mock function with given fields:
func (_m *MockWorker) Stop() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Stop")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockWorker_Stop_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Stop'
type MockWorker_Stop_Call struct {
	*mock.Call
}

// Stop is a helper method to define mock.On call
func (_e *MockWorker_Expecter) Stop() *MockWorker_Stop_Call {
	return &MockWorker_Stop_Call{Call: _e.mock.On("Stop")}
}

func (_c *MockWorker_Stop_Call) Run(run func()) *MockWorker_Stop_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWorker_Stop_Call) Return(_a0 error) *MockWorker_Stop_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockWorker_Stop_Call) RunAndReturn(run func() error) *MockWorker_Stop_Call {
	_c.Call.Return(run)
	return _c
}

// Wait provides a mock function with given fields: _a0
func (_m *MockWorker) Wait(_a0 context.Context) (ExitEvent, error) {
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
type MockWorker_Wait_Call struct {
	*mock.Call
}

// Wait is a helper method to define mock.On call
//   - _a0 context.Context
func (_e *MockWorker_Expecter) Wait(_a0 interface{}) *MockWorker_Wait_Call {
	return &MockWorker_Wait_Call{Call: _e.mock.On("Wait", _a0)}
}

func (_c *MockWorker_Wait_Call) Run(run func(_a0 context.Context)) *MockWorker_Wait_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockWorker_Wait_Call) Return(_a0 ExitEvent, _a1 error) *MockWorker_Wait_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWorker_Wait_Call) RunAndReturn(run func(context.Context) (ExitEvent, error)) *MockWorker_Wait_Call {
	_c.Call.Return(run)
	return _c
}

// WaitFor provides a mock function with given fields: _a0, _a1
func (_m *MockWorker) WaitFor(_a0 context.Context, _a1 time.Duration) (ExitEvent, error) {
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
type MockWorker_WaitFor_Call struct {
	*mock.Call
}

// WaitFor is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 time.Duration
func (_e *MockWorker_Expecter) WaitFor(_a0 interface{}, _a1 interface{}) *MockWorker_WaitFor_Call {
	return &MockWorker_WaitFor_Call{Call: _e.mock.On("WaitFor", _a0, _a1)}
}

func (_c *MockWorker_WaitFor_Call) Run(run func(_a0 context.Context, _a1 time.Duration)) *MockWorker_WaitFor_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(time.Duration))
	})
	return _c
}

func (_c *MockWorker_WaitFor_Call) Return(_a0 ExitEvent, _a1 error) *MockWorker_WaitFor_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWorker_WaitFor_Call) RunAndReturn(run func(context.Context, time.Duration) (ExitEvent, error)) *MockWorker_WaitFor_Call {
	_c.Call.Return(run)
	return _c
}

// WritePipe provides a mock function with given fields:
func (_m *MockWorker) WritePipe() (io.WriteCloser, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for WritePipe")
	}

	var r0 io.WriteCloser
	var r1 error
	if rf, ok := ret.Get(0).(func() (io.WriteCloser, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() io.WriteCloser); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.WriteCloser)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockWorker_WritePipe_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'WritePipe'
type MockWorker_WritePipe_Call struct {
	*mock.Call
}

// WritePipe is a helper method to define mock.On call
func (_e *MockWorker_Expecter) WritePipe() *MockWorker_WritePipe_Call {
	return &MockWorker_WritePipe_Call{Call: _e.mock.On("WritePipe")}
}

func (_c *MockWorker_WritePipe_Call) Run(run func()) *MockWorker_WritePipe_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockWorker_WritePipe_Call) Return(_a0 io.WriteCloser, _a1 error) *MockWorker_WritePipe_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockWorker_WritePipe_Call) RunAndReturn(run func() (io.WriteCloser, error)) *MockWorker_WritePipe_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockWorker creates a new instance of MockWorker. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockWorker(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockWorker {
	mock := &MockWorker{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
