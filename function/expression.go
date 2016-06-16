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
	"sync"
	"sync/atomic"
	"time"

	"github.com/square/metrics/api"
	"github.com/square/metrics/inspect"
	"github.com/square/metrics/metric_metadata"
	"github.com/square/metrics/query/predicate"
	"github.com/square/metrics/tasks"
	"github.com/square/metrics/timeseries"
)

// EvaluationContext is the central piece of logic, providing
// helper funcions & varaibles to evaluate a given piece of
// metrics query.
// * Contains a TimeseriesStorageAPI object, which can be used to fetch data
// from the backend systems.
// * Contains current timerange being queried for - this can be
// changed by say, application of time shift function.
type EvaluationContext struct {
	TimeseriesStorageAPI  timeseries.StorageAPI   // Backend to fetch data from
	MetricMetadataAPI     metadata.MetricAPI      // Api to obtain metadata from
	Timerange             api.Timerange           // Timerange to fetch data from
	SampleMethod          timeseries.SampleMethod // SampleMethod to use when up/downsampling to match the requested resolution
	Predicate             predicate.Predicate     // Predicate to apply to TagSets prior to fetching
	FetchLimit            FetchCounter            // A limit on the number of fetches which may be performed
	Timeout               *tasks.Timeout
	Registry              Registry
	Profiler              *inspect.Profiler // A profiler pointer
	EvaluationNotes       *EvaluationNotes  // Debug + numerical notes that can be added during evaluation
	UserSpecifiableConfig timeseries.UserSpecifiableConfig
}

// EvaluationNotes holds notes that were recorded during evaluation.
type EvaluationNotes struct {
	mutex sync.Mutex
	notes []string
}

// AddNote adds a new note to the collection in a threadsafe manner.
func (notes *EvaluationNotes) AddNote(note string) {
	// @@ leaking param: notes
	// @@ leaking param: note
	if notes == nil {
		return
	}
	notes.mutex.Lock()
	defer notes.mutex.Unlock()
	// @@ notes.mutex escapes to heap
	notes.notes = append(notes.notes, note)
	// @@ notes.mutex escapes to heap
}

// Notes returns the current collection of notes in a threadsafe manner.
func (notes *EvaluationNotes) Notes() []string {
	// @@ leaking param: notes
	if notes == nil {
		return nil
	}
	notes.mutex.Lock()
	defer notes.mutex.Unlock()
	// @@ notes.mutex escapes to heap
	return notes.notes
	// @@ notes.mutex escapes to heap
}

// WithTimerange duplicates the EvaluationContext but with a new timerange.
func (e EvaluationContext) WithTimerange(t api.Timerange) EvaluationContext {
	// @@ leaking param: e to result ~r1 level=0
	e.Timerange = t
	// @@ can inline EvaluationContext.WithTimerange
	return e
}

// FetchCounter is used to count the number of fetches remaining in a thread-safe manner.
type FetchCounter struct {
	count *int32
	limit int
}

// NewFetchCounter creates a FetchCounter with n as the limit.
func NewFetchCounter(n int) FetchCounter {
	n32 := int32(n)
	// @@ can inline NewFetchCounter
	return FetchCounter{
		// @@ moved to heap: n32
		count: &n32,
		limit: n,
		// @@ &n32 escapes to heap
	}
}

// Limit returns the max # of fetches allowed by this counter.
func (c FetchCounter) Limit() int {
	return c.limit
	// @@ can inline FetchCounter.Limit
}

// Current returns the current number of fetches remaining for the counter.
func (c FetchCounter) Current() int {
	// @@ leaking param: c
	return c.limit - int(atomic.LoadInt32(c.count))
}

// Consume decrements the internal counter and returns whether the result is at least 0.
// It does so in a threadsafe manner.
func (c FetchCounter) Consume(n int) error {
	// @@ leaking param: c
	remaining := atomic.AddInt32(c.count, -int32(n))
	if remaining < 0 {
		return fmt.Errorf("performing fetch of %d additional series brings the total to %d, which exceeds the specified limit %d", n, c.limit-int(remaining), c.limit)
	}
	// @@ n escapes to heap
	// @@ c.limit - int(remaining) escapes to heap
	// @@ c.limit escapes to heap
	return nil
}

// Expression is a piece of code, which can be evaluated in a given
// EvaluationContext. EvaluationContext must never be changed in an Evalute().
//
// The contract of Expressions is that leaf nodes must sample a resulting
// timeseries according to the resolution specified in its EvaluationContext's
// Timerange. Internal nodes may assume that results from evaluating child
// Expressions correspond to the timerange in the current EvaluationContext.
type Expression interface {
	// Evaluate the given expression.
	Evaluate(context EvaluationContext) (Value, error)
	Name() string
	QueryString() string
}

// EvaluateToScalar is a helper function that takes an Expression and makes it a scalar.
func EvaluateToScalar(e Expression, context EvaluationContext) (float64, error) {
	// @@ leaking param: context
	// @@ leaking param: e
	scalarValue, err := e.Evaluate(context)
	if err != nil {
		return 0, err
	}
	value, convErr := scalarValue.ToScalar()
	if convErr != nil {
		return 0, convErr.WithContext(e.QueryString())
	}
	// @@ inlining call to (*ConversionFailure).WithContext
	// @@ convErr.WithContext(e.QueryString()) escapes to heap
	return value, nil
}

// EvaluateToScalarSet is a helper function that takes an Expression and makes it a scalar set.
func EvaluateToScalarSet(e Expression, context EvaluationContext) (ScalarSet, error) {
	// @@ leaking param: context
	// @@ leaking param: e
	scalarValue, err := e.Evaluate(context)
	if err != nil {
		return nil, err
	}
	value, convErr := scalarValue.ToScalarSet()
	if convErr != nil {
		return nil, convErr.WithContext(e.QueryString())
	}
	// @@ inlining call to (*ConversionFailure).WithContext
	// @@ convErr.WithContext(e.QueryString()) escapes to heap
	return value, nil
}

// EvaluateToDuration is a helper function that takes an Expression and makes it a duration.
func EvaluateToDuration(e Expression, context EvaluationContext) (time.Duration, error) {
	// @@ leaking param: context
	// @@ leaking param: e
	durationValue, err := e.Evaluate(context)
	if err != nil {
		return 0, err
	}
	value, convErr := durationValue.ToDuration()
	if convErr != nil {
		return 0, convErr.WithContext(e.QueryString())
	}
	// @@ inlining call to (*ConversionFailure).WithContext
	// @@ convErr.WithContext(e.QueryString()) escapes to heap
	return value, nil
}

// EvaluateToDuration is a helper function that takes an Expression and makes it a series list.
func EvaluateToSeriesList(e Expression, context EvaluationContext) (api.SeriesList, error) {
	// @@ leaking param: context
	// @@ leaking param: e
	seriesValue, err := e.Evaluate(context)
	if err != nil {
		return api.SeriesList{}, err
	}
	value, convErr := seriesValue.ToSeriesList(context.Timerange)
	if convErr != nil {
		return api.SeriesList{}, convErr.WithContext(e.QueryString())
	}
	// @@ inlining call to (*ConversionFailure).WithContext
	// @@ convErr.WithContext(e.QueryString()) escapes to heap
	return value, nil
}

// EvaluateToDuration is a helper function that takes an Expression and makes it a string.
func EvaluateToString(e Expression, context EvaluationContext) (string, error) {
	// @@ leaking param: context
	// @@ leaking param: e
	stringValue, err := e.Evaluate(context)
	if err != nil {
		return "", err
	}
	value, convErr := stringValue.ToString()
	if convErr != nil {
		return "", convErr.WithContext(e.QueryString())
	}
	// @@ inlining call to (*ConversionFailure).WithContext
	// @@ convErr.WithContext(e.QueryString()) escapes to heap
	return value, nil
}

// EvaluateMany evaluates a list of expressions using a single EvaluationContext.
// If any evaluation errors, EvaluateMany will propagate that error. The resulting values
// will be in the order corresponding to the provided expressions.
func EvaluateMany(context EvaluationContext, expressions []Expression) ([]Value, error) {
	// @@ leaking param: context
	// @@ leaking param content: expressions
	// @@ leaking param content: expressions
	// @@ leaking param content: expressions
	type result struct {
		// @@ moved to heap: context
		index int
		err   error
		value Value
	}
	length := len(expressions)
	if length == 0 {
		return []Value{}, nil
	}
	// @@ []Value literal escapes to heap
	if length == 1 {
		result, err := expressions[0].Evaluate(context)
		if err != nil {
			return nil, err
		}
		return []Value{result}, nil
	}
	// @@ []Value literal escapes to heap
	// concurrent evaluations
	results := make(chan result, length)
	for i, expr := range expressions {
		// @@ make(chan result, length) escapes to heap
		go func(i int, expr Expression) {
			// @@ leaking param: expr
			value, err := expr.Evaluate(context)
			// @@ func literal escapes to heap
			// @@ func literal escapes to heap
			results <- result{i, err, value}
			// @@ leaking closure reference context
			// @@ &context escapes to heap
		}(i, expr)
	}
	array := make([]Value, length)
	for i := 0; i < length; i++ {
		// @@ make([]Value, length) escapes to heap
		// @@ make([]Value, length) escapes to heap
		result := <-results
		if result.err != nil {
			return nil, result.err
		} else {
			array[result.index] = result.value
		}
	}
	return array, nil

}
