// Copyright © 2009--2013 The Web.go Authors
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"io"
	"net/http"
	"reflect"
)

// internal handler type. handler of slightly differing signatures are accepted
// but transformed (wrapped) early on to match this one.
type handlerf func(ctx *Context, arg ...string) error

// internal handler whose parameters have been closed over
type closedhandlerf func(*Context) error

// functions according to reflect
type valuefun func([]reflect.Value) []reflect.Value

var nilerr error
var nilerrv reflect.Value = reflect.ValueOf(&nilerr).Elem()
var errtype reflect.Type = reflect.TypeOf(&nilerr).Elem()

// Small optimization: cache the context type instead of repeteadly calling reflect.Typeof
var contextType reflect.Type = reflect.TypeOf(Context{})

// Bind parameters to a handler
func closeHandler(h handlerf, arg ...string) closedhandlerf {
	return func(ctx *Context) error {
		return h(ctx, arg...)
	}
}

//should the context be passed to the handler?
func requiresContext(handlerType reflect.Type) bool {
	//if the method doesn't take arguments, no
	if handlerType.NumIn() == 0 {
		return false
	}

	//if the first argument is not a pointer, no
	a0 := handlerType.In(0)
	if a0.Kind() != reflect.Ptr {
		return false
	}
	//if the first argument is a context, yes
	if a0.Elem() == contextType {
		return true
	}

	return false
}

// waiting for go1.1
func callableValue(fv reflect.Value) valuefun {
	if fv.Type().Kind() != reflect.Func {
		panic("not a function value")
	}
	return func(args []reflect.Value) []reflect.Value {
		return fv.Call(args)
	}
}

// Wrap f in a function that disregards its first arg
func disregardFirstArg(f valuefun) valuefun {
	return func(args []reflect.Value) []reflect.Value {
		return f(args[1:])
	}
}

// Wrap f to return a nil error value in addition to current return values
func addNilErrorReturn(f valuefun) valuefun {
	return func(args []reflect.Value) []reflect.Value {
		ret := f(args)
		return append(ret, nilerrv)
	}
}

// Wrap f to write its string return value to the first arg (being an io.Writer)
// requires the original function signature to be:
//
// func (io.Writer, ...) (string, error)
//
// signature of wrapped function:
//
// func (io.Writer, ...) error
//
// if the error value of the original call is not nil that value is passed back
// verbatim and no further action is taken. If it is nil the wrapper writes the
// string to the writer and returns whatever error ocurred there, if any.
//
// Note that wherever it says string []byte is also okay.
func writeStringToFirstArg(f valuefun) valuefun {
	return func(args []reflect.Value) []reflect.Value {
		wv := args[0]
		w, ok := wv.Interface().(io.Writer)
		if !ok {
			panic("First argument must be an io.Writer")
		}
		ret := f(args)
		if len(ret) < 2 {
			panic("Two return values required for proper wrapping")
		}
		if i := ret[1].Interface(); i != nil {
			return ret[1:]
		}
		var ar []byte
		if i := ret[0].Interface(); i != nil {
			switch typed := i.(type) {
			case string:
				ar = []byte(typed)
				break
			case []byte:
				ar = typed
				break
			default:
				panic("First return value must be a byte array / string")
			}
		}
		_, err := w.Write(ar)
		if err != nil {
			return []reflect.Value{reflect.ValueOf(err)}
		}
		return []reflect.Value{nilerrv}
	}
}

func lastRetIsError(fv reflect.Value) bool {
	// type of fun
	t := fv.Type()
	if t.NumOut() == 0 {
		return false
	}
	// type of last return val
	t = t.Out(t.NumOut() - 1)
	return t.Implements(errtype)
}

func firstRetIsString(fv reflect.Value) bool {
	// type of fun
	t := fv.Type()
	if t.NumOut() == 0 {
		return false
	}
	// type of first return val
	t = t.Out(0)
	return t.AssignableTo(reflect.TypeOf("")) || t.AssignableTo(reflect.TypeOf([]byte{}))
}

// convert a value back to the original error interface. panics if value is not
// nil and also does not implement error.
func value2error(v reflect.Value) error {
	i := v.Interface()
	if i == nil {
		return nil
	}
	return i.(error)
}

// Beat the supplied handler into a uniform signature. panics if incompatible
// (may only happen when the wrapped fun is called)
func fixHandlerSignature(f interface{}) handlerf {
	// classic net/http.Hander implementors can easily be converted
	if httph, ok := f.(http.Handler); ok {
		return func(ctx *Context, args ...string) error {
			httph.ServeHTTP(ctx.ResponseWriter, ctx.Request)
			return nil
		}
	}
	fv := reflect.ValueOf(f)
	var callf valuefun = callableValue(fv)
	if !requiresContext(fv.Type()) {
		callf = disregardFirstArg(callf)
	}
	// now callf definitely accepts a *Context as its first arg
	if !lastRetIsError(fv) {
		callf = addNilErrorReturn(callf)
	}
	// now callf definitely returns an error as its last value
	if firstRetIsString(fv) {
		callf = writeStringToFirstArg(callf)
	}
	// now callf definitely does not return a string: just an error
	// wrap callf in a function with pretty signature
	return func(ctx *Context, args ...string) error {
		argvs := make([]reflect.Value, len(args)+1)
		argvs[0] = reflect.ValueOf(ctx)
		for i, arg := range args {
			argvs[i+1] = reflect.ValueOf(arg)
		}
		rets := callf(argvs)
		return value2error(rets[0])
	}
}
