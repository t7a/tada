package tada

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
)

// -- CONSTRUCTORS

// NewDataFrame stub
func NewDataFrame(slices []interface{}, labels ...interface{}) *DataFrame {
	// handle values
	var values []*valueContainer
	for i, slice := range slices {
		if !isSlice(slice) {
			return &DataFrame{err: fmt.Errorf(
				"NewDataFrame(): unsupported kind (%v) in `slices` (position %v); must be slice", reflect.TypeOf(slice), i)}
		}
		if reflect.ValueOf(slice).Len() == 0 {
			return &DataFrame{err: fmt.Errorf("NewDataFrame(): empty slice in slices (position %v): cannot be empty", i)}
		}
		isNull := setNullsFromInterface(slice)
		if isNull == nil {
			return &DataFrame{err: fmt.Errorf(
				"NewDataFrame(): unable to calculate null values ([]%v not supported)", reflect.TypeOf(slice).Elem())}
		}
		// handle special case of []Element: convert to []interface{}
		elements := handleElementsSlice(slice)
		if elements != nil {
			slice = elements
		}
		values = append(values, &valueContainer{slice: slice, isNull: isNull, name: fmt.Sprintf("%d", i)})
	}

	// handle labels
	retLabels := make([]*valueContainer, len(labels))
	if len(retLabels) == 0 {
		// default labels
		defaultLabels, isNull := makeDefaultLabels(0, reflect.ValueOf(slices[0]).Len())
		retLabels = append(retLabels, &valueContainer{slice: defaultLabels, isNull: isNull, name: "*0"})
	} else {
		for i := range retLabels {
			slice := labels[i]
			if !isSlice(slice) {
				return dataFrameWithError(fmt.Errorf("NewDataFrame(): unsupported label kind (%v) at level %d; must be slice", reflect.TypeOf(slice), i))
			}
			isNull := setNullsFromInterface(slice)
			if isNull == nil {
				return dataFrameWithError(fmt.Errorf(
					"NewDataFrame(): unable to calculate null values at level %d ([]%v not supported)", i, reflect.TypeOf(slice).Elem()))
			}
			// handle special case of []Element: convert to []interface{}
			elements := handleElementsSlice(slice)
			if elements != nil {
				slice = elements
			}
			retLabels[i] = &valueContainer{slice: slice, isNull: isNull, name: fmt.Sprintf("*%d", i)}
		}
	}

	return &DataFrame{values: values, labels: retLabels}
}

// Copy stub
func (df *DataFrame) Copy() *DataFrame {
	values := make([]*valueContainer, len(df.values))
	for j := range df.values {
		values[j] = df.values[j].copy()
	}

	labels := make([]*valueContainer, len(df.labels))
	for j := range df.labels {
		labels[j] = df.labels[j].copy()
	}

	return &DataFrame{
		values: values,
		labels: labels,
		err:    df.err,
		name:   df.name,
	}
}

// ReadCSV stub
func (df *DataFrame) ReadCSV(csv [][]string) *DataFrame {
	return nil
}

// ReadInterface stub
func (df *DataFrame) ReadInterface([][]interface{}) *DataFrame {
	return nil
}

// ReadStructs stub
func (df *DataFrame) ReadStructs(interface{}) *DataFrame {
	return nil
}

// -- GETTERS

// Len returns the number of rows in each column of the DataFrame.
func (df *DataFrame) Len() int {
	return reflect.ValueOf(df.values[0].slice).Len()
}

// Levels returns the number of columns of labels in the DataFrame.
func (df *DataFrame) Levels() int {
	return len(df.labels)
}

// InPlace returns a DataFrameMutator, which contains most of the same methods as DataFrame but never returns a new DataFrame.
// If you want to save memory and improve performance and do not need to preserve the original DataFrame, consider using InPlace().
func (df *DataFrame) InPlace() *DataFrameMutator {
	return &DataFrameMutator{dataframe: df}
}

// Subset returns only the rows specified at the index positions, in the order specified. Returns a new DataFrame.
func (df *DataFrame) Subset(index []int) *DataFrame {
	df = df.Copy()
	df.InPlace().Subset(index)
	return df
}

// Subset returns only the rows specified at the index positions, in the order specified.
// Modifies the underlying DataFrame in place.
func (df *DataFrameMutator) Subset(index []int) {
	if reflect.DeepEqual(index, []int{-999}) {
		df.dataframe.resetWithError(errors.New(
			"Subset(): invalid filter (every filter must have at least one filter function; if ColName is supplied, it must be valid)"))
	}
	for k := range df.dataframe.values {
		err := df.dataframe.values[k].subsetRows(index)
		if err != nil {
			df.dataframe.resetWithError(fmt.Errorf("Subset(): %v", err))
			return
		}
	}
	for j := range df.dataframe.labels {
		df.dataframe.labels[j].subsetRows(index)
	}
	return
}

// SubsetLabels returns only the labels specified at the index positions, in the order specified.
// Returns a new DataFrame.
func (df *DataFrame) SubsetLabels(index []int) *DataFrame {
	df = df.Copy()
	df.InPlace().SubsetLabels(index)
	return df
}

// SubsetLabels returns only the labels specified at the index positions, in the order specified.
// Modifies the underlying DataFrame in place.
func (df *DataFrameMutator) SubsetLabels(index []int) {
	labels, err := subsetCols(df.dataframe.labels, index)
	if err != nil {
		df.dataframe.resetWithError(fmt.Errorf("SubsetLabels(): %v", err))
		return
	}
	df.dataframe.labels = labels
	return
}

// SubsetCols returns only the labels specified at the index positions, in the order specified.
// Returns a new DataFrame.
func (df *DataFrame) SubsetCols(index []int) *DataFrame {
	df = df.Copy()
	df.InPlace().SubsetCols(index)
	return df
}

// SubsetCols returns only the labels specified at the index positions, in the order specified.
// Modifies the underlying DataFrame in place.
func (df *DataFrameMutator) SubsetCols(index []int) {
	cols, err := subsetCols(df.dataframe.values, index)
	if err != nil {
		df.dataframe.resetWithError(fmt.Errorf("SubsetCols(): %v", err))
		return
	}
	df.dataframe.values = cols
	return
}

// Col finds the first column with matching `name` and returns as a Series.
func (df *DataFrame) Col(name string) *Series {
	index, err := findColWithName(name, df.values)
	if err != nil {
		return seriesWithError(fmt.Errorf("Col(): %v", err))
	}
	return &Series{
		values: df.values[index],
		labels: df.labels,
	}
}

// Cols returns all column with matching `names`.
func (df *DataFrame) Cols(names ...string) *DataFrame {
	vals := make([]*valueContainer, len(names))
	for i, name := range names {
		index, err := findColWithName(name, df.values)
		if err != nil {
			return dataFrameWithError(fmt.Errorf("Cols(): %v", err))
		}
		vals[i] = df.values[index]
	}
	return &DataFrame{
		values: vals,
		labels: df.labels,
		name:   df.name,
	}
}

// Head returns the first `n` rows of the Series. If `n` is greater than the length of the Series, returns the entire Series.
// In either case, returns a new Series.
func (df *DataFrame) Head(n int) *DataFrame {
	if df.Len() < n {
		n = df.Len()
	}
	retVals := make([]*valueContainer, len(df.values))
	for k := range df.values {
		retVals[k] = df.values[k].head(n)
	}
	retLabels := make([]*valueContainer, df.Levels())
	for j := range df.labels {
		retLabels[j] = df.labels[j].head(n)
	}
	return &DataFrame{values: retVals, labels: retLabels, name: df.name}
}

// Tail returns the last `n` rows of the Series. If `n` is greater than the length of the Series, returns the entire Series.
// In either case, returns a new Series.
func (df *DataFrame) Tail(n int) *DataFrame {
	if df.Len() < n {
		n = df.Len()
	}
	retVals := make([]*valueContainer, len(df.values))
	for k := range df.values {
		retVals[k] = df.values[k].tail(n)
	}
	retLabels := make([]*valueContainer, df.Levels())
	for j := range df.labels {
		retLabels[j] = df.labels[j].tail(n)
	}
	return &DataFrame{values: retVals, labels: retLabels, name: df.name}
}

// Range returns the rows of the DataFrame starting at `first` and `ending` with last (inclusive).
// If either `first` or `last` is greater than the length of the DataFrame, a DataFrame error is returned.
// In all cases, returns a new DataFrame.
func (df *DataFrame) Range(first, last int) *DataFrame {
	if first >= df.Len() {
		return dataFrameWithError(fmt.Errorf("Range(): first index out of range (%d > %d)", first, df.Len()-1))
	} else if last >= df.Len() {
		return dataFrameWithError(fmt.Errorf("Range(): last index out of range (%d > %d)", last, df.Len()-1))
	}
	retVals := make([]*valueContainer, len(df.values))
	for k := range df.values {
		retVals[k] = df.values[k].rangeSlice(first, last)
	}
	retLabels := make([]*valueContainer, df.Levels())
	for j := range df.labels {
		retLabels[j] = df.labels[j].rangeSlice(first, last)
	}
	return &DataFrame{values: retVals, labels: retLabels, name: df.name}
}

// Valid returns all the rows with all non-null values.
// If `subset` is supplied, returns all the rows with all non-null values in the specified columns.
// Returns a new DataFrame.
func (df *DataFrame) Valid(subset ...string) *DataFrame {
	var index []int
	if len(subset) == 0 {
		index = makeIntRange(0, len(df.values))
	} else {
		for _, name := range subset {
			i, err := findColWithName(name, df.values)
			if err != nil {
				return dataFrameWithError(fmt.Errorf("Valid(): %v", err))
			}
			index = append(index, i)
		}

	}

	subIndexes := make([][]int, len(index))
	for k := range index {
		subIndexes[k] = df.values[k].valid()
	}
	allValid := intersection(subIndexes)
	return df.Subset(allValid)
}

// Null returns all the rows with any null values.
// If `subset` is supplied, returns all the rows with all non-null values in the specified columns.
// Returns a new DataFrame.
func (df *DataFrame) Null(subset ...string) *DataFrame {
	var index []int
	if len(subset) == 0 {
		index = makeIntRange(0, len(df.values))
	} else {
		for _, name := range subset {
			i, err := findColWithName(name, df.values)
			if err != nil {
				return dataFrameWithError(fmt.Errorf("Valid(): %v", err))
			}
			index = append(index, i)
		}
	}

	subIndexes := make([][]int, len(index))
	for k := range index {
		subIndexes[k] = df.values[k].null()
	}
	anyNull := union(subIndexes)
	return df.Subset(anyNull)
}

// FilterCols returns the column positions of all columns (excluding labels) that satisfy `lambda`.
// If a column contains multiple levels, its name is a single pipe-delimited string and may be split within the lambda function.
func (df *DataFrame) FilterCols(lambda func(string) bool) []int {
	var ret []int
	for k := range df.values {
		if lambda(df.values[k].name) {
			ret = append(ret, k)
		}
	}
	return ret
}

// -- SETTERS

// WithLabels resolves as follows:
//
// If a scalar string is supplied as `input` and a column of labels exists that matches `name`: rename the level to match `input`
//
// If a slice is supplied as `input` and a column of labels exists that matches `name`: replace the values at this level to match `input`
//
// If a slice is supplied as `input` and a column of labels does not exist that matches `name`: append a new level with a name matching `name` and values matching `input`
//
// Error conditions: supplying slice of unsupported type, supplying slice with a different length than the underlying DataFrame, or supplying scalar string and `name` that does not match an existing label level.
// In all cases, returns a new DataFrame.
func (df *DataFrame) WithLabels(name string, input interface{}) *DataFrame {
	df.Copy()
	df.InPlace().WithLabels(name, input)
	return df
}

// WithLabels resolves as follows:
//
// If a scalar string is supplied as `input` and a column of labels exists that matches `name`: rename the level to match `input`
//
// If a slice is supplied as `input` and a column of labels exists that matches `name`: replace the values at this level to match `input`
//
// If a slice is supplied as `input` and a column of labels does not exist that matches `name`: append a new level with a name matching `name` and values matching `input`
//
// Error conditions: supplying slice of unsupported type, supplying slice with a different length than the underlying DataFrame, or supplying scalar string and `name` that does not match an existing label level.
// In all cases, modifies the underlying DataFrame in place.
func (df *DataFrameMutator) WithLabels(name string, input interface{}) {
	labels, err := withColumn(df.dataframe.labels, name, input, df.dataframe.Len())
	if err != nil {
		df.dataframe.resetWithError(fmt.Errorf("WithLabels(): %v", err))
	}
	df.dataframe.labels = labels
}

// WithCol resolves as follows:
//
// If a scalar string is supplied as `input` and a column exists that matches `name`: rename the column to match `input`
//
// If a slice is supplied as `input` and a column exists that matches `name`: replace the values at this column to match `input`
//
// If a slice is supplied as `input` and a column does not exist that matches `name`: append a new column with a name matching `name` and values matching `input`
//
// Error conditions: supplying slice of unsupported type, supplying slice with a different length than the underlying DataFrame, or supplying scalar string and `name` that does not match an existing label level.
// In all cases, returns a new DataFrame.
func (df *DataFrame) WithCol(name string, input interface{}) *DataFrame {
	df.Copy()
	df.InPlace().WithCol(name, input)
	return df
}

// WithCol resolves as follows:
//
// If a scalar string is supplied as `input` and a column exists that matches `name`: rename the column to match `input`
//
// If a slice is supplied as `input` and a column exists that matches `name`: replace the values at this column to match `input`
//
// If a slice is supplied as `input` and a column does not exist that matches `name`: append a new column with a name matching `name` and values matching `input`
//
// Error conditions: supplying slice of unsupported type, supplying slice with a different length than the underlying DataFrame, or supplying scalar string and `name` that does not match an existing label level.
// In all cases, modifies the underlying DataFrame in place.
func (df *DataFrameMutator) WithCol(name string, input interface{}) {
	cols, err := withColumn(df.dataframe.values, name, input, df.dataframe.Len())
	if err != nil {
		df.dataframe.resetWithError(fmt.Errorf("WithCol(): %v", err))
	}
	df.dataframe.values = cols
}

// WithRow stub
func (df *DataFrame) WithRow(label string, values []interface{}) *DataFrame {
	return nil
}

// DropCol drops the first column matching `name`
// Returns a new DataFrame.
func (df *DataFrame) DropCol(name string) *DataFrame {
	df.Copy()
	df.InPlace().DropCol(name)
	return df
}

// DropCol drops the first column matching `name`
func (df *DataFrameMutator) DropCol(name string) {
	toExclude, err := findColWithName(name, df.dataframe.values)
	if err != nil {
		df.dataframe.resetWithError(fmt.Errorf("DropCol(): %v", err))
	}
	index := excludeFromIndex(len(df.dataframe.values), toExclude)
	df.SubsetCols(index)
	return
}

// Drop removes the row at the specified index.
// Returns a new DataFrame.
func (df *DataFrame) Drop(index int) *DataFrame {
	df.Copy()
	df.InPlace().Drop(index)
	return df
}

// Drop removes the row at the specified index.
// Modifies the underlying DataFrame in place.
func (df *DataFrameMutator) Drop(index int) {
	for k := range df.dataframe.values {
		err := df.dataframe.values[k].dropRow(index)
		if err != nil {
			df.dataframe.resetWithError(fmt.Errorf("Drop(): %v", err))
			return
		}
	}
	for j := range df.dataframe.labels {
		df.dataframe.labels[j].dropRow(index)
	}
	return
}

// SetLabels removes the row at the specified index.
// Returns a new DataFrame.
func (df *DataFrame) SetLabels(colNames ...string) *DataFrame {
	df.Copy()
	df.InPlace().SetLabels(colNames...)
	return df
}

// SetLabels appends the column(s) supplied as `colNames` as label levels and drops the column(s).
// The number of `colNames` supplied must be less than the number of columns in the Series.
// Modifies the underlying DataFrame in place.
func (df *DataFrameMutator) SetLabels(colNames ...string) {
	if len(colNames) >= len(df.dataframe.values) {
		df.dataframe.resetWithError(fmt.Errorf("SetLabels(): number of colNames must be less than number of columns (%d >= %d)",
			len(colNames), len(df.dataframe.values)))
	}
	for i := 0; i < len(colNames); i++ {
		index, err := findColWithName(colNames[i], df.dataframe.values)
		if err != nil {
			df.dataframe.resetWithError(fmt.Errorf("SetLabels(): %v", err))
		}
		df.dataframe.labels = append(df.dataframe.labels, df.dataframe.values[index])
		df.DropCol(colNames[i])
	}
	return
}

// ResetLabels appends the label level(s) at the supplied index levels as columns and drops the levels.
// If no index levels are supplied, all label levels are appended as columns and dropped as levels, and replaced by a default label column.
// Returns a new DataFrame.
func (df *DataFrame) ResetLabels(index ...int) *DataFrame {
	df.Copy()
	df.InPlace().ResetLabels(index...)
	return df
}

// ResetLabels appends the label level(s) at the supplied index levels as columns and drops the levels.
// If no index levels are supplied, all label levels are appended as columns and dropped as levels, and replaced by a default label column.
// Modifies the underlying DataFrame in place.
func (df *DataFrameMutator) ResetLabels(index ...int) {
	if len(index) == 0 {
		index = makeIntRange(0, df.dataframe.Levels())
	}
	for _, i := range index {
		if i >= df.dataframe.Levels() {
			df.dataframe.resetWithError(fmt.Errorf("ResetLabels(): index out of range (%d > %d)", i, df.dataframe.Levels()-1))
		}
		newVal := df.dataframe.labels[i]
		newVal.name = regexp.MustCompile(`^\*`).ReplaceAllString(newVal.name, "")
		df.dataframe.values = append(df.dataframe.values, newVal)
		df.dataframe.labels, _ = subsetCols(df.dataframe.labels, excludeFromIndex(df.dataframe.Levels(), i))
	}
	if df.dataframe.Levels() == 0 {
		labels, isNull := makeDefaultLabels(0, df.dataframe.Len())
		df.dataframe.labels[0] = &valueContainer{slice: labels, isNull: isNull, name: "*0"}
	}
	return
}

// SetName stub
// in place
func (df *DataFrame) SetName() *DataFrame {
	return nil
}

// SetCols stub
// in place
func (df *DataFrame) SetCols() *DataFrame {
	return nil
}

// reshape

// Transpose stub
func (df *DataFrame) Transpose() *DataFrame {
	return nil
}

// PromoteCol stub
func (df *DataFrame) PromoteCol(name string) *DataFrame {
	return nil
}

// LabelToCol stub
func (df *DataFrame) LabelToCol(label string) *DataFrame {
	return nil
}

// ColToLabel stub
func (df *DataFrame) ColToLabel(name string) *DataFrame {
	return nil
}

// filter

// FilterFloat stub
func (df *DataFrame) FilterFloat(func(val float64) bool) *DataFrame {
	return nil
}

// apply

// ApplyFloat stub
func (df *DataFrame) ApplyFloat(func(val float64) float64) *DataFrame {
	return nil
}

// combine

// Merge stub
func (df *DataFrame) Merge(other *DataFrame) *DataFrame {
	return nil
}

// Lookup stub
func (df *DataFrame) Lookup(other *DataFrame, how string, leftOn string, rightOn string, dimension Dimension) *DataFrame {
	return nil
}

// Add stub
func (df *DataFrame) Add(other *DataFrame) *DataFrame {
	return nil
}

// Subtract stub
func (df *DataFrame) Subtract(other *DataFrame) *DataFrame {
	return nil
}

// Multiply stub
func (df *DataFrame) Multiply(other *DataFrame) *DataFrame {
	return nil
}

// Divide stub
func (df *DataFrame) Divide(other *DataFrame) *DataFrame {
	return nil
}

// sort

// Sort stub
func (df *DataFrame) Sort(...Sorter) *DataFrame {
	return nil
}

// grouping

// GroupBy stub
// includes label levels and columns
func (df *DataFrame) GroupBy(names ...string) *GroupedDataFrame {
	return nil
}

// PivotTable stub
func (df *DataFrame) PivotTable(labels, columns, values, aggFn string) *DataFrame {
	return nil
}

// iterator

// IterRows stub
func (df *DataFrame) IterRows() []map[string]Element {
	return nil
}

// IterCols stub
func (df *DataFrame) IterCols() []map[string]Element {
	return nil
}

// math

// Sum stub
func (df *DataFrame) Sum() *Series {
	return nil
}

// Mean stub
func (df *DataFrame) Mean() *Series {
	return nil
}

// Median stub
func (df *DataFrame) Median() *Series {
	return nil
}

// Std stub
func (df *DataFrame) Std() *Series {
	return nil
}
