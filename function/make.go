// Copyright 2015 - 2016 Square Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package function

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/square/metrics/api"
)

var stringType = reflect.TypeOf("")
var scalarType = reflect.TypeOf(float64(0.0))
var scalarSetType = reflect.TypeOf(ScalarSet{})
var durationType = reflect.TypeOf(time.Duration(0))
var timeseriesType = reflect.TypeOf(api.SeriesList{})
var valueType = reflect.TypeOf((*Value)(nil)).Elem()
var expressionType = reflect.TypeOf((*Expression)(nil)).Elem()
var groupsType = reflect.TypeOf(Groups{})
var contextType = reflect.TypeOf(EvaluationContext{})
var timerangeType = reflect.TypeOf(api.Timerange{})

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// MakeFunction is a convenient way to use type-safe functions to
// construct MetricFunctions without manually checking parameters.
func MakeFunction(name string, function interface{}) MetricFunction {
	// @@ leaking param: function
	// @@ leaking param: name to result ~r2 level=0
	funcValue := reflect.ValueOf(function)
	if funcValue.Kind() != reflect.Func {
		panic("MakeFunction expects a function as input.")
		// @@ inlining call to reflect.Value.Kind
		// @@ inlining call to reflect.reflect.flag.reflect.kind
	}
	funcType := funcValue.Type()
	if funcType.IsVariadic() {
		panic("MakeFunction's argument cannot be variadic.")
	}
	if funcType.NumOut() == 0 {
		panic("MakeFunction's argument function must return a value.")
	}
	if funcType.NumOut() > 2 {
		panic("MakeFunction's argument function must return at most two values.")
	}
	if !funcType.Out(0).ConvertibleTo(valueType) && funcType.Out(0) != timeseriesType {
		panic("MakeFunction's argument function's first return type must be convertible to `function.Value`.")
	}
	if funcType.NumOut() == 2 && !funcType.Out(1).ConvertibleTo(errorType) {
		panic("MakeFunction's argument function's second return type must convertible be `error`.")
	}

	requiredArgumentCount := 0
	optionalArgumentCount := 0
	allowsGroupBy := false
	for i := 0; i < funcType.NumIn(); i++ {
		argType := funcType.In(i)
		switch argType {
		case contextType, timerangeType:
			// Asks for part of context.
		case groupsType:
			// asks for groups
			allowsGroupBy = true
		case stringType, scalarType, scalarSetType, durationType, timeseriesType, valueType, expressionType:
			// An ordinary argument.
			if optionalArgumentCount > 0 {
				panic("Non-optional arguments cannot occur after optional ones.")
			}
			requiredArgumentCount++
		case reflect.PtrTo(stringType), reflect.PtrTo(scalarType), reflect.PtrTo(scalarSetType), reflect.PtrTo(durationType), reflect.PtrTo(timeseriesType), reflect.PtrTo(valueType), reflect.PtrTo(expressionType):
			// An optional argument
			optionalArgumentCount++
		default:
			panic(fmt.Sprintf("MetricFunction function argument asks for unsupported type: cannot supply argument %d of type %+v.", i, argType))
		}
		// @@ i escapes to heap
		// @@ argType escapes to heap
	}
	// The function has been checked and inspected.
	// Now, generate the corresponding MetricFunction.

	return MetricFunction{
		FunctionName:  name,
		MinArguments:  requiredArgumentCount,
		MaxArguments:  requiredArgumentCount + optionalArgumentCount,
		AllowsGroupBy: allowsGroupBy,
		// Compute does a lot of reflection to get this to work.
		Compute: func(context EvaluationContext, arguments []Expression, groups Groups) (Value, error) {
			// @@ leaking param: context
			// @@ leaking param: groups

			// @@ moved to heap: context
			// @@ func literal escapes to heap
			// @@ func literal escapes to heap
			// nextArgument will extract the next argument from the expression list `arguments`.
			// if there are not more to return, it will return nil.
			expressionArgument := 0
			nextArgument := func() Expression {
				if expressionArgument >= len(arguments) {
					// @@ can inline MakeFunction.func1.1
					return nil
				}
				arg := arguments[expressionArgument]
				expressionArgument++
				return arg
			}

			// evalTo takes an expression and a reflect.Type and evaluates to the appropriate type.
			// If an Expression is requested, it just returns it.
			evalTo := func(expression Expression, resultType reflect.Type) (interface{}, error) {
				// @@ leaking param: expression
				// @@ leaking param: resultType
				switch resultType {
				// @@ func literal escapes to heap
				// @@ func literal escapes to heap
				// @@ func literal escapes to heap
				// @@ func literal escapes to heap
				case expressionType:
					return expression, nil
				case stringType:
					// @@ expression escapes to heap
					return EvaluateToString(expression, context)
				case scalarType:
					// @@ leaking closure reference context
					// @@ leaking closure reference context
					// @@ leaking closure reference context
					// @@ leaking closure reference context
					// @@ leaking closure reference context
					// @@ leaking closure reference context
					// @@ &context escapes to heap
					// @@ &context escapes to heap
					return EvaluateToScalar(expression, context)
				case scalarSetType:
					return EvaluateToScalarSet(expression, context)
				case durationType:
					return EvaluateToDuration(expression, context)
				case timeseriesType:
					return EvaluateToSeriesList(expression, context)
				case valueType:
					return expression.Evaluate(context)
				}
				panic(fmt.Sprintf("Unreachable :: Attempting to evaluate to unknown type %+v", resultType))
			}
			// @@ resultType escapes to heap

			// argumentFuncs holds functions to obtain the Value arguments.
			argumentFuncs := make([]func() (interface{}, error), funcType.NumIn())

			// @@ make([]func() (interface {}, error), funcType.NumIn()) escapes to heap
			// @@ leaking closure reference funcType
			// @@ leaking closure reference funcType
			// @@ leaking closure reference funcType
			// @@ leaking closure reference funcType
			// @@ make([]func() (interface {}, error), funcType.NumIn()) escapes to heap
			// @@ leaking closure reference funcType
			// provideValue takes any value, and returns a function that returns it.
			provideValue := func(x interface{}) func() (interface{}, error) {
				// @@ leaking param: x
				return func() (interface{}, error) {
					return x, nil
					// @@ can inline MakeFunction.func1.3.1
					// @@ func literal escapes to heap
					// @@ func literal escapes to heap
				}
			}

			// provideZeroValue takes a type, and returns a function that returns the zero-value for that type.
			provideZeroValue := func(t reflect.Type) func() (interface{}, error) {
				// @@ leaking param: t
				return provideValue(reflect.Zero(t).Interface())
			}

			// ptrTo takes a value and returns a pointer to that value.
			ptrTo := func(x interface{}) interface{} {
				// @@ leaking param: x
				ptr := reflect.New(reflect.TypeOf(x))
				// @@ func literal escapes to heap
				// @@ func literal escapes to heap
				ptr.Elem().Set(reflect.ValueOf(x))
				// @@ inlining call to reflect.TypeOf
				// @@ inlining call to reflect.toType
				// @@ reflect.t·2 escapes to heap
				return ptr.Interface()
			}

			for i := range argumentFuncs {
				argType := funcType.In(i)
				switch argType {
				case contextType:
					argumentFuncs[i] = provideValue(context)
				case timerangeType:
					// @@ context escapes to heap
					argumentFuncs[i] = provideValue(context.Timerange)
				case groupsType:
					// @@ context.Timerange escapes to heap
					argumentFuncs[i] = provideValue(groups)
				case stringType, scalarType, scalarSetType, durationType, timeseriesType, valueType, expressionType:
					// @@ groups escapes to heap
					arg := nextArgument()
					argumentFuncs[i] = func() (interface{}, error) {
						return evalTo(arg, argType)
						// @@ func literal escapes to heap
						// @@ func literal escapes to heap
					}
					// @@ leaking closure reference arg
					// @@ leaking closure reference argType
				case reflect.PtrTo(stringType), reflect.PtrTo(scalarType), reflect.PtrTo(scalarSetType), reflect.PtrTo(durationType), reflect.PtrTo(timeseriesType), reflect.PtrTo(valueType), reflect.PtrTo(expressionType):
					arg := nextArgument()
					if arg == nil {
						argumentFuncs[i] = provideZeroValue(argType)
					} else {
						argumentFuncs[i] = func() (interface{}, error) {
							resultI, err := evalTo(arg, argType.Elem())
							// @@ func literal escapes to heap
							// @@ func literal escapes to heap
							if err != nil {
								// @@ leaking closure reference argType
								// @@ leaking closure reference arg
								return nil, err
							}
							return ptrTo(resultI), nil
						}
					}
				default:
					panic(fmt.Sprintf("Unreachable :: Argument to MakeFunction requests invalid type %+v.", argType))
				}
				// @@ argType escapes to heap
			}

			// Now we evaluate the functions in parallel.

			waiter := sync.WaitGroup{}
			argValues := make([]reflect.Value, funcType.NumIn())
			// @@ moved to heap: waiter
			errors := make(chan error, funcType.NumIn())
			// @@ make([]reflect.Value, funcType.NumIn()) escapes to heap
			// @@ make([]reflect.Value, funcType.NumIn()) escapes to heap
			for i := range argValues {
				// @@ make(chan error, funcType.NumIn()) escapes to heap
				i := i
				waiter.Add(1)
				go func() {
					// @@ waiter escapes to heap
					defer waiter.Done()
					// @@ func literal escapes to heap
					// @@ func literal escapes to heap
					arg, err := argumentFuncs[i]()
					// @@ waiter escapes to heap
					// @@ leaking closure reference waiter
					// @@ &waiter escapes to heap
					if err != nil {
						errors <- err
						return
					}
					argValues[i] = reflect.ValueOf(arg)
				}()
			}
			waiter.Wait() // Wait for all the arguments to be evaluated.

			// @@ waiter escapes to heap
			if len(errors) != 0 {
				return nil, <-errors
			}

			output := funcValue.Call(argValues)

			// @@ leaking closure reference funcValue
			if len(output) == 2 && output[1].Interface() != nil {
				return nil, output[1].Interface().(error)
			}
			switch funcType.Out(0) {
			case stringType:
				return StringValue(output[0].Interface().(string)), nil
			case scalarType:
				// @@ StringValue(output[0].Interface().(string)) escapes to heap
				return ScalarValue(output[0].Interface().(float64)), nil
			case scalarSetType:
				// @@ ScalarValue(output[0].Interface().(float64)) escapes to heap
				return output[0].Interface().(ScalarSet), nil
			case durationType:
				// @@ output[0].Interface().(ScalarSet) escapes to heap
				return DurationValue{"", output[0].Interface().(time.Duration)}, nil
			case timeseriesType:
				// @@ composite literal escapes to heap
				return SeriesListValue(output[0].Interface().(api.SeriesList)), nil
			default:
				// @@ SeriesListValue(output[0].Interface().(api.SeriesList)) escapes to heap
				return output[0].Interface().(Value), nil
			}
		},
	}

}
