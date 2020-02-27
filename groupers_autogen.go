// This file was automatically generated.
// Any changes will be lost if this file is regenerated.
// Run "make generate" to regenerate from template.

package tada

import (
	"fmt"
	"time"
)

func convertSimplifiedFloat64Func(
	simplifiedFn func([]float64) float64) func(
	[]float64, []bool, []int) (float64, bool) {

	fn := func(vals []float64, isNull []bool, index []int) (float64, bool) {
		var atLeastOneValid bool
		inputVals := make([]float64, 0)
		for _, i := range index {
			if !isNull[i] {
				inputVals = append(inputVals, vals[i])
				atLeastOneValid = true
			}
		}
		if !atLeastOneValid {
			return empty{}.float64(), true
		}
		return simplifiedFn(inputVals), false
	}
	return fn
}

func convertSimplifiedFloat64FuncNested(
	simplifiedFn func([]float64) []float64) func(
	[]float64, []bool, []int) ([]float64, bool) {

	fn := func(vals []float64, isNull []bool, index []int) ([]float64, bool) {
		var atLeastOneValid bool
		inputVals := make([]float64, 0)
		for _, i := range index {
			if !isNull[i] {
				inputVals = append(inputVals, vals[i])
				atLeastOneValid = true
			}
		}
		if !atLeastOneValid {
			return []float64{}, true
		}
		return simplifiedFn(inputVals), false
	}
	return fn
}

func groupedFloat64Func(
	vals []float64,
	nulls []bool,
	name string,
	aligned bool,
	rowIndices [][]int,
	fn func(val []float64, isNull []bool, index []int) (float64, bool)) *valueContainer {
	// default: return length is equal to the number of groups
	retLength := len(rowIndices)
	if aligned {
		// if aligned: return length is overwritten to equal the length of original data
		retLength = len(vals)
	}
	retVals := make([]float64, retLength)
	retNulls := make([]bool, retLength)
	for i, rowIndex := range rowIndices {
		output, isNull := fn(vals, nulls, rowIndex)
		if !aligned {
			// default: write each output once and in sequential order into retVals
			retVals[i] = output
			retNulls[i] = isNull
		} else {
			// if aligned: write each output multiple times and out of order into retVals
			for _, index := range rowIndex {
				retVals[index] = output
				retNulls[index] = isNull
			}
		}
	}
	return &valueContainer{
		slice:  retVals,
		isNull: retNulls,
		name:   name,
	}
}

func groupedFloat64FuncNested(
	vals []float64,
	nulls []bool,
	name string,
	aligned bool,
	rowIndices [][]int,
	fn func(val []float64, isNull []bool, index []int) ([]float64, bool)) *valueContainer {
	// default: return length is equal to the number of groups
	retLength := len(rowIndices)
	if aligned {
		// if aligned: return length is overwritten to equal the length of original data
		retLength = len(vals)
	}
	retVals := make([][]float64, retLength)
	retNulls := make([]bool, retLength)
	for i, rowIndex := range rowIndices {
		output, isNull := fn(vals, nulls, rowIndex)
		if !aligned {
			// default: write each output once and in sequential order
			retVals[i] = output
			retNulls[i] = isNull
		} else {
			// if aligned: write each output multiple times and out of order
			for _, index := range rowIndex {
				retVals[index] = output
				retNulls[index] = isNull
			}
		}
	}
	return &valueContainer{
		slice:  retVals,
		isNull: retNulls,
		name:   name,
	}
}

func (g *GroupedSeries) float64Func(name string, fn func(val []float64, isNull []bool, index []int) (float64, bool)) *Series {
	var sharedData bool
	if g.aligned {
		name = fmt.Sprintf("%v_%v", g.series.values.name, name)
	}
	retVals := groupedFloat64Func(
		g.series.values.float64().slice, g.series.values.isNull, name, g.aligned, g.rowIndices, fn)
	// default: grouped labels
	retLabels := g.labels
	if g.aligned {
		// if aligned: all labels
		retLabels = g.series.labels
		sharedData = true
	}
	return &Series{
		values:     retVals,
		labels:     retLabels,
		sharedData: sharedData,
	}
}

func (g *GroupedSeries) float64FuncNested(name string, fn func(val []float64, isNull []bool, index []int) ([]float64, bool)) *Series {
	var sharedData bool
	if g.aligned {
		name = fmt.Sprintf("%v_%v", g.series.values.name, name)
	}
	retVals := groupedFloat64FuncNested(
		g.series.values.float64().slice, g.series.values.isNull, name, g.aligned, g.rowIndices, fn)
	// default: grouped labels
	retLabels := g.labels
	if g.aligned {
		// if aligned: all labels
		retLabels = g.series.labels
		sharedData = true
	}
	return &Series{
		values:     retVals,
		labels:     retLabels,
		sharedData: sharedData,
	}
}

func (g *GroupedDataFrame) float64Func(
	name string, cols []string, fn func(val []float64, isNull []bool, index []int) (float64, bool)) *DataFrame {
	if len(cols) == 0 {
		cols = make([]string, len(g.df.values))
		for k := range cols {
			cols[k] = g.df.values[k].name
		}
	}
	retVals := make([]*valueContainer, len(cols))
	for k := range retVals {
		retVals[k] = groupedFloat64Func(
			g.df.values[k].float64().slice, g.df.values[k].isNull, cols[k], false, g.rowIndices, fn)
	}
	return &DataFrame{
		values:        retVals,
		labels:        g.labels,
		colLevelNames: []string{"*0"},
		name:          name,
	}
}

func (g *GroupedDataFrame) float64FuncNested(
	name string, cols []string, fn func(val []float64, isNull []bool, index []int) ([]float64, bool)) *DataFrame {
	if len(cols) == 0 {
		cols = make([]string, len(g.df.values))
		for k := range cols {
			cols[k] = g.df.values[k].name
		}
	}
	retVals := make([]*valueContainer, len(cols))
	for k := range retVals {
		retVals[k] = groupedFloat64FuncNested(
			g.df.values[k].float64().slice, g.df.values[k].isNull, cols[k], false, g.rowIndices, fn)
	}
	return &DataFrame{
		values:        retVals,
		labels:        g.labels,
		colLevelNames: []string{"*0"},
		name:          name,
	}
}

func convertSimplifiedStringFunc(
	simplifiedFn func([]string) string) func(
	[]string, []bool, []int) (string, bool) {

	fn := func(vals []string, isNull []bool, index []int) (string, bool) {
		var atLeastOneValid bool
		inputVals := make([]string, 0)
		for _, i := range index {
			if !isNull[i] {
				inputVals = append(inputVals, vals[i])
				atLeastOneValid = true
			}
		}
		if !atLeastOneValid {
			return empty{}.string(), true
		}
		return simplifiedFn(inputVals), false
	}
	return fn
}

func convertSimplifiedStringFuncNested(
	simplifiedFn func([]string) []string) func(
	[]string, []bool, []int) ([]string, bool) {

	fn := func(vals []string, isNull []bool, index []int) ([]string, bool) {
		var atLeastOneValid bool
		inputVals := make([]string, 0)
		for _, i := range index {
			if !isNull[i] {
				inputVals = append(inputVals, vals[i])
				atLeastOneValid = true
			}
		}
		if !atLeastOneValid {
			return []string{}, true
		}
		return simplifiedFn(inputVals), false
	}
	return fn
}

func groupedStringFunc(
	vals []string,
	nulls []bool,
	name string,
	aligned bool,
	rowIndices [][]int,
	fn func(val []string, isNull []bool, index []int) (string, bool)) *valueContainer {
	// default: return length is equal to the number of groups
	retLength := len(rowIndices)
	if aligned {
		// if aligned: return length is overwritten to equal the length of original data
		retLength = len(vals)
	}
	retVals := make([]string, retLength)
	retNulls := make([]bool, retLength)
	for i, rowIndex := range rowIndices {
		output, isNull := fn(vals, nulls, rowIndex)
		if !aligned {
			// default: write each output once and in sequential order into retVals
			retVals[i] = output
			retNulls[i] = isNull
		} else {
			// if aligned: write each output multiple times and out of order into retVals
			for _, index := range rowIndex {
				retVals[index] = output
				retNulls[index] = isNull
			}
		}
	}
	return &valueContainer{
		slice:  retVals,
		isNull: retNulls,
		name:   name,
	}
}

func groupedStringFuncNested(
	vals []string,
	nulls []bool,
	name string,
	aligned bool,
	rowIndices [][]int,
	fn func(val []string, isNull []bool, index []int) ([]string, bool)) *valueContainer {
	// default: return length is equal to the number of groups
	retLength := len(rowIndices)
	if aligned {
		// if aligned: return length is overwritten to equal the length of original data
		retLength = len(vals)
	}
	retVals := make([][]string, retLength)
	retNulls := make([]bool, retLength)
	for i, rowIndex := range rowIndices {
		output, isNull := fn(vals, nulls, rowIndex)
		if !aligned {
			// default: write each output once and in sequential order
			retVals[i] = output
			retNulls[i] = isNull
		} else {
			// if aligned: write each output multiple times and out of order
			for _, index := range rowIndex {
				retVals[index] = output
				retNulls[index] = isNull
			}
		}
	}
	return &valueContainer{
		slice:  retVals,
		isNull: retNulls,
		name:   name,
	}
}

func (g *GroupedSeries) stringFunc(name string, fn func(val []string, isNull []bool, index []int) (string, bool)) *Series {
	var sharedData bool
	if g.aligned {
		name = fmt.Sprintf("%v_%v", g.series.values.name, name)
	}
	retVals := groupedStringFunc(
		g.series.values.string().slice, g.series.values.isNull, name, g.aligned, g.rowIndices, fn)
	// default: grouped labels
	retLabels := g.labels
	if g.aligned {
		// if aligned: all labels
		retLabels = g.series.labels
		sharedData = true
	}
	return &Series{
		values:     retVals,
		labels:     retLabels,
		sharedData: sharedData,
	}
}

func (g *GroupedSeries) stringFuncNested(name string, fn func(val []string, isNull []bool, index []int) ([]string, bool)) *Series {
	var sharedData bool
	if g.aligned {
		name = fmt.Sprintf("%v_%v", g.series.values.name, name)
	}
	retVals := groupedStringFuncNested(
		g.series.values.string().slice, g.series.values.isNull, name, g.aligned, g.rowIndices, fn)
	// default: grouped labels
	retLabels := g.labels
	if g.aligned {
		// if aligned: all labels
		retLabels = g.series.labels
		sharedData = true
	}
	return &Series{
		values:     retVals,
		labels:     retLabels,
		sharedData: sharedData,
	}
}

func (g *GroupedDataFrame) stringFunc(
	name string, cols []string, fn func(val []string, isNull []bool, index []int) (string, bool)) *DataFrame {
	if len(cols) == 0 {
		cols = make([]string, len(g.df.values))
		for k := range cols {
			cols[k] = g.df.values[k].name
		}
	}
	retVals := make([]*valueContainer, len(cols))
	for k := range retVals {
		retVals[k] = groupedStringFunc(
			g.df.values[k].string().slice, g.df.values[k].isNull, cols[k], false, g.rowIndices, fn)
	}
	return &DataFrame{
		values:        retVals,
		labels:        g.labels,
		colLevelNames: []string{"*0"},
		name:          name,
	}
}

func (g *GroupedDataFrame) stringFuncNested(
	name string, cols []string, fn func(val []string, isNull []bool, index []int) ([]string, bool)) *DataFrame {
	if len(cols) == 0 {
		cols = make([]string, len(g.df.values))
		for k := range cols {
			cols[k] = g.df.values[k].name
		}
	}
	retVals := make([]*valueContainer, len(cols))
	for k := range retVals {
		retVals[k] = groupedStringFuncNested(
			g.df.values[k].string().slice, g.df.values[k].isNull, cols[k], false, g.rowIndices, fn)
	}
	return &DataFrame{
		values:        retVals,
		labels:        g.labels,
		colLevelNames: []string{"*0"},
		name:          name,
	}
}

func convertSimplifiedDateTimeFunc(
	simplifiedFn func([]time.Time) time.Time) func(
	[]time.Time, []bool, []int) (time.Time, bool) {

	fn := func(vals []time.Time, isNull []bool, index []int) (time.Time, bool) {
		var atLeastOneValid bool
		inputVals := make([]time.Time, 0)
		for _, i := range index {
			if !isNull[i] {
				inputVals = append(inputVals, vals[i])
				atLeastOneValid = true
			}
		}
		if !atLeastOneValid {
			return empty{}.dateTime(), true
		}
		return simplifiedFn(inputVals), false
	}
	return fn
}

func convertSimplifiedDateTimeFuncNested(
	simplifiedFn func([]time.Time) []time.Time) func(
	[]time.Time, []bool, []int) ([]time.Time, bool) {

	fn := func(vals []time.Time, isNull []bool, index []int) ([]time.Time, bool) {
		var atLeastOneValid bool
		inputVals := make([]time.Time, 0)
		for _, i := range index {
			if !isNull[i] {
				inputVals = append(inputVals, vals[i])
				atLeastOneValid = true
			}
		}
		if !atLeastOneValid {
			return []time.Time{}, true
		}
		return simplifiedFn(inputVals), false
	}
	return fn
}

func groupedDateTimeFunc(
	vals []time.Time,
	nulls []bool,
	name string,
	aligned bool,
	rowIndices [][]int,
	fn func(val []time.Time, isNull []bool, index []int) (time.Time, bool)) *valueContainer {
	// default: return length is equal to the number of groups
	retLength := len(rowIndices)
	if aligned {
		// if aligned: return length is overwritten to equal the length of original data
		retLength = len(vals)
	}
	retVals := make([]time.Time, retLength)
	retNulls := make([]bool, retLength)
	for i, rowIndex := range rowIndices {
		output, isNull := fn(vals, nulls, rowIndex)
		if !aligned {
			// default: write each output once and in sequential order into retVals
			retVals[i] = output
			retNulls[i] = isNull
		} else {
			// if aligned: write each output multiple times and out of order into retVals
			for _, index := range rowIndex {
				retVals[index] = output
				retNulls[index] = isNull
			}
		}
	}
	return &valueContainer{
		slice:  retVals,
		isNull: retNulls,
		name:   name,
	}
}

func groupedDateTimeFuncNested(
	vals []time.Time,
	nulls []bool,
	name string,
	aligned bool,
	rowIndices [][]int,
	fn func(val []time.Time, isNull []bool, index []int) ([]time.Time, bool)) *valueContainer {
	// default: return length is equal to the number of groups
	retLength := len(rowIndices)
	if aligned {
		// if aligned: return length is overwritten to equal the length of original data
		retLength = len(vals)
	}
	retVals := make([][]time.Time, retLength)
	retNulls := make([]bool, retLength)
	for i, rowIndex := range rowIndices {
		output, isNull := fn(vals, nulls, rowIndex)
		if !aligned {
			// default: write each output once and in sequential order
			retVals[i] = output
			retNulls[i] = isNull
		} else {
			// if aligned: write each output multiple times and out of order
			for _, index := range rowIndex {
				retVals[index] = output
				retNulls[index] = isNull
			}
		}
	}
	return &valueContainer{
		slice:  retVals,
		isNull: retNulls,
		name:   name,
	}
}

func (g *GroupedSeries) dateTimeFunc(name string, fn func(val []time.Time, isNull []bool, index []int) (time.Time, bool)) *Series {
	var sharedData bool
	if g.aligned {
		name = fmt.Sprintf("%v_%v", g.series.values.name, name)
	}
	retVals := groupedDateTimeFunc(
		g.series.values.dateTime().slice, g.series.values.isNull, name, g.aligned, g.rowIndices, fn)
	// default: grouped labels
	retLabels := g.labels
	if g.aligned {
		// if aligned: all labels
		retLabels = g.series.labels
		sharedData = true
	}
	return &Series{
		values:     retVals,
		labels:     retLabels,
		sharedData: sharedData,
	}
}

func (g *GroupedSeries) dateTimeFuncNested(name string, fn func(val []time.Time, isNull []bool, index []int) ([]time.Time, bool)) *Series {
	var sharedData bool
	if g.aligned {
		name = fmt.Sprintf("%v_%v", g.series.values.name, name)
	}
	retVals := groupedDateTimeFuncNested(
		g.series.values.dateTime().slice, g.series.values.isNull, name, g.aligned, g.rowIndices, fn)
	// default: grouped labels
	retLabels := g.labels
	if g.aligned {
		// if aligned: all labels
		retLabels = g.series.labels
		sharedData = true
	}
	return &Series{
		values:     retVals,
		labels:     retLabels,
		sharedData: sharedData,
	}
}

func (g *GroupedDataFrame) dateTimeFunc(
	name string, cols []string, fn func(val []time.Time, isNull []bool, index []int) (time.Time, bool)) *DataFrame {
	if len(cols) == 0 {
		cols = make([]string, len(g.df.values))
		for k := range cols {
			cols[k] = g.df.values[k].name
		}
	}
	retVals := make([]*valueContainer, len(cols))
	for k := range retVals {
		retVals[k] = groupedDateTimeFunc(
			g.df.values[k].dateTime().slice, g.df.values[k].isNull, cols[k], false, g.rowIndices, fn)
	}
	return &DataFrame{
		values:        retVals,
		labels:        g.labels,
		colLevelNames: []string{"*0"},
		name:          name,
	}
}

func (g *GroupedDataFrame) dateTimeFuncNested(
	name string, cols []string, fn func(val []time.Time, isNull []bool, index []int) ([]time.Time, bool)) *DataFrame {
	if len(cols) == 0 {
		cols = make([]string, len(g.df.values))
		for k := range cols {
			cols[k] = g.df.values[k].name
		}
	}
	retVals := make([]*valueContainer, len(cols))
	for k := range retVals {
		retVals[k] = groupedDateTimeFuncNested(
			g.df.values[k].dateTime().slice, g.df.values[k].isNull, cols[k], false, g.rowIndices, fn)
	}
	return &DataFrame{
		values:        retVals,
		labels:        g.labels,
		colLevelNames: []string{"*0"},
		name:          name,
	}
}