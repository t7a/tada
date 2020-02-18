package tada

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
)

func (s *Series) resetWithError(err error) {
	s.values = nil
	s.labels = nil
	s.err = err
}

func (df *DataFrame) resetWithError(err error) {
	df.values = nil
	df.labels = nil
	df.name = ""
	df.err = err
	df.colLevelNames = nil
}

func seriesWithError(err error) *Series {
	return &Series{
		err: err,
	}
}

func dataFrameWithError(err error) *DataFrame {
	return &DataFrame{
		err: err,
	}
}

func isSlice(input interface{}) bool {
	return reflect.TypeOf(input).Kind() == reflect.Slice
}

func makeValueContainerFromInterface(slice interface{}, name string) (*valueContainer, error) {
	if !isSlice(slice) {
		return nil, fmt.Errorf("unsupported kind (%v); must be slice", reflect.TypeOf(slice).Kind())
	}
	if reflect.ValueOf(slice).Len() == 0 {
		return nil, fmt.Errorf("empty slice: cannot be empty")
	}
	isNull := setNullsFromInterface(slice)
	if isNull == nil {
		return nil, fmt.Errorf("unable to calculate null values ([]%v not supported)", reflect.TypeOf(slice).Elem())
	}
	return &valueContainer{
		slice: slice, isNull: isNull, name: name,
	}, nil
}

func makeValueContainersFromInterfaces(slices []interface{}, prefixAsterisk bool) ([]*valueContainer, error) {
	var namePrefix string
	if prefixAsterisk {
		namePrefix = "*"
	}
	ret := make([]*valueContainer, len(slices))
	for i, slice := range slices {
		vc, err := makeValueContainerFromInterface(slice, namePrefix+fmt.Sprint(i))
		if err != nil {
			return nil, fmt.Errorf("error at position %d: %v", i, err)
		}
		ret[i] = vc
	}
	return ret, nil
}

// makeDefaultLabels returns a valueContainer with a
// sequential series of numbers (inclusive of min, exclusive of max), a companion isNull slice, and a name.
func makeDefaultLabels(min, max int) *valueContainer {
	labels := make([]int, max-min)
	isNull := make([]bool, len(labels))
	for i := range labels {
		labels[i] = min + i
		isNull[i] = false
	}
	return &valueContainer{
		slice:  labels,
		isNull: isNull,
		name:   "*0",
	}
}

// makeIntRange returns a sequential series of numbers (inclusive of min, exclusive of max)
func makeIntRange(min, max int) []int {
	ret := make([]int, max-min)
	for i := range ret {
		ret[i] = min + i
	}
	return ret
}

// search for a name only once. if it is found multiple times, return only the first
func findMatchingKeysBetweenTwoLabelContainers(labels1 []*valueContainer, labels2 []*valueContainer) ([]int, []int) {
	var leftKeys, rightKeys []int
	searched := make(map[string]bool)
	// add every level name to the map in order to skip duplicates
	for j := range labels1 {
		key := labels1[j].name
		// if level name already in map, skip
		if _, ok := searched[key]; ok {
			continue
			// if level name not already in map, add to map
		} else {
			searched[key] = true
		}
		for k := range labels2 {
			// compare to every name in labels2
			if key == labels2[k].name {
				leftKeys = append(leftKeys, j)
				rightKeys = append(rightKeys, k)
				break
			}
		}
	}
	return leftKeys, rightKeys
}

// if name is found in neither columns nor labels, return error
func findNameInColumnsOrLabels(name string, cols []*valueContainer, labels []*valueContainer) (index int, isCol bool, err error) {
	// first check column names
	if lvl, err := findColWithName(name, cols); err == nil {
		return lvl, true, nil
		// then check label level names
	} else if lvl, err := findColWithName(name, labels); err == nil {
		return lvl, false, nil
	} else {
		return 0, false, fmt.Errorf("name (%v) does not match any existing column", name)
	}
}

// findColWithName returns the position of the first level within `cols` with a name matching `name`, or an error if no level matches
func findColWithName(name string, cols []*valueContainer) (int, error) {
	for j := range cols {
		if strings.ToLower(cols[j].name) == strings.ToLower(name) {
			return j, nil
		}
	}
	return 0, fmt.Errorf("name (%v) does not match any existing column", name)
}

func withColumn(cols []*valueContainer, name string, input interface{}, requiredLen int) ([]*valueContainer, error) {
	switch reflect.TypeOf(input).Kind() {
	// `input` is string: rename label level
	case reflect.String:
		lvl, err := findColWithName(name, cols)
		if err != nil {
			return nil, fmt.Errorf("cannot rename column: %v", err)
		}
		cols[lvl].name = input.(string)
	case reflect.Slice:
		isNull := setNullsFromInterface(input)
		if isNull == nil {
			return nil, fmt.Errorf("unable to calculate null values ([]%v not supported)", reflect.TypeOf(input).Elem())
		}
		if l := reflect.ValueOf(input).Len(); l != requiredLen {
			return nil, fmt.Errorf(
				"cannot replace items in column %s: length of input does not match existing length (%d != %d)",
				name, l, requiredLen)
		}
		// `input` is supported slice
		lvl, err := findColWithName(name, cols)
		if err != nil {
			// `name` does not already exist: append new label level
			cols = append(cols, &valueContainer{slice: input, name: name, isNull: isNull})
		} else {
			// `name` already exists: overwrite existing label level
			cols[lvl].slice = input
			cols[lvl].isNull = isNull
		}
	case reflect.Ptr:
		v, ok := input.(*Series)
		if !ok {
			return nil, fmt.Errorf("unsupported input: *Series is only supported pointer")
		}

		if v.Len() != requiredLen {
			return nil, fmt.Errorf(
				"cannot replace items in column %s: length of input Series does not match existing length (%d != %d)",
				name, v.Len(), requiredLen)
		}
		// `name` does not already exist: append new level
		lvl, err := findColWithName(name, cols)
		if err != nil {
			cols = append(cols, v.values)
			cols[len(cols)-1].name = name
		} else {
			// `name` already exists: overwrite existing level
			cols[lvl] = v.values
			cols[lvl].name = name
		}

	default:
		return nil, fmt.Errorf("unsupported input kind: must be either slice, string, or Series")
	}
	return cols, nil
}

// -- MATRIX MANIPULATION

// expects every item in `slices` to be a slice, and for len(slices) to equal len(isNull) and len(names)
// if isNull is nil, sets null values from `slices`
func copyInterfaceIntoValueContainers(slices []interface{}, isNull [][]bool, names []string) []*valueContainer {
	ret := make([]*valueContainer, len(names))
	if isNull == nil {
		isNull = make([][]bool, len(slices))
		for k := range slices {
			isNull[k] = setNullsFromInterface(slices[k])
		}
	}
	for k := range slices {
		ret[k] = &valueContainer{
			slice:  slices[k],
			isNull: isNull[k],
			name:   names[k],
		}
	}
	return ret
}

// convert strings to interface. if isNull is nil, sets null values from `slices`
func copyStringsIntoValueContainers(slices [][]string, isNull [][]bool, names []string) []*valueContainer {
	slicesInterface := make([]interface{}, len(slices))
	for k := range slices {
		slicesInterface[k] = slices[k]
	}
	return copyInterfaceIntoValueContainers(slicesInterface, isNull, names)
}

// columns as major dimension
func makeStringMatrix(numCols, numRows int) [][]string {
	ret := make([][]string, numCols)
	for k := 0; k < numCols; k++ {
		ret[k] = make([]string, numRows)
	}
	return ret
}

// columns as major dimension
func makeBoolMatrix(numCols, numRows int) [][]bool {
	ret := make([][]bool, numCols)
	for k := 0; k < numCols; k++ {
		ret[k] = make([]bool, numRows)
	}
	return ret
}

func copyGroupedLabels(labels []string, levelNames []string) []*valueContainer {
	labelLevels := makeStringMatrix(len(levelNames), len(labels))
	labelIsNull := makeBoolMatrix(len(levelNames), len(labels))

	for i := range labels {
		splitLabels := splitLabelIntoLevels(labels[i], true)
		for j := range splitLabels {
			labelLevels[j][i] = splitLabels[j]
			labelIsNull[j][i] = false
		}
	}
	return copyStringsIntoValueContainers(labelLevels, labelIsNull, levelNames)
}

func intersection(slices [][]int) []int {
	set := make(map[int]int)
	for _, slice := range slices {
		for _, i := range slice {
			if _, ok := set[i]; !ok {
				set[i] = 1
			} else {
				set[i]++
			}
		}
	}
	var ret []int
	orderedKeys := make([]int, 0)
	for k := range set {
		orderedKeys = append(orderedKeys, k)
	}
	sort.Ints(orderedKeys)
	for _, key := range orderedKeys {
		// this means that the value appeared in every slice
		if set[key] == len(slices) {
			ret = append(ret, key)
		}
	}
	return ret
}

func union(slices [][]int) []int {
	var ret []int
	set := make(map[int]bool)
	for _, slice := range slices {
		for i := range slice {
			if _, ok := set[slice[i]]; !ok {
				set[slice[i]] = true
				ret = append(ret, slice[i])
			}
		}
	}
	sort.Ints(ret)
	return ret
}

func difference(slice1 []int, slice2 []int) []int {
	var ret []int
	// mark as true if contained in slice1
	set := make(map[int]bool)
	for _, i := range slice1 {
		set[i] = true
	}
	// mark as false if contained in slice2 as well
	for _, i := range slice2 {
		if _, ok := set[i]; ok {
			set[i] = false
		}
	}
	orderedKeys := make([]int, 0)
	for k := range set {
		orderedKeys = append(orderedKeys, k)
	}
	sort.Ints(orderedKeys)
	for _, k := range orderedKeys {
		if set[k] {
			ret = append(ret, k)
		}
	}
	return ret
}

func (df *DataFrame) toCSVByRows(ignoreLabels bool) ([][]string, error) {
	if df.values == nil {
		return nil, fmt.Errorf("cannot export empty dataframe")
	}
	// make final container with rows as major dimension
	ret := make([][]string, df.numColLevels()+df.Len())
	for i := range ret {
		var newCols int
		if !ignoreLabels {
			newCols = df.numLevels() + df.numColumns()
		} else {
			newCols = df.numColumns()
		}
		ret[i] = make([]string, newCols)
	}
	if !ignoreLabels {
		for j := range df.labels {
			// write label headers, index at first header row
			ret[df.numColLevels()-1][j] = df.labels[j].name
			v := df.labels[j].str().slice
			// write label values, offset by header rows
			for i := range v {
				ret[i+df.numColLevels()][j] = v[i]
			}
		}
	}
	// if there are multiple column headers, those rows will be blank above the index header
	for k := range df.values {
		var offset int
		if !ignoreLabels {
			offset = df.numLevels()
		}
		// if number of col levels is only one, return the name as a single-item slice
		multiColHeaders := splitLabelIntoLevels(df.values[k].name, df.numColLevels() > 1)
		for l := 0; l < df.numColLevels(); l++ {
			// write multi column headers, offset by label levels
			ret[l][k+offset] = multiColHeaders[l]
		}
		v := df.values[k].str().slice
		// write label values, offset by header rows and label levels
		for i := range v {
			ret[i+df.numColLevels()][k+offset] = v[i]
		}
	}
	return ret, nil
}

// expects non-nil cfg
func readCSVByRows(csv [][]string, cfg *ReadConfig) *DataFrame {
	numCols := len(csv[0]) - cfg.NumLabelCols
	numRows := len(csv) - cfg.NumHeaderRows

	// prepare intermediary values containers
	vals := makeStringMatrix(numCols, numRows)
	valsIsNull := makeBoolMatrix(numCols, numRows)
	valsNames := makeStringMatrix(numCols, cfg.NumHeaderRows)

	// prepare intermediary label containers
	labels := makeStringMatrix(cfg.NumLabelCols, numRows)
	labelsIsNull := makeBoolMatrix(cfg.NumLabelCols, numRows)
	levelNames := makeStringMatrix(cfg.NumLabelCols, cfg.NumHeaderRows)

	// iterate over csv and transpose rows and columns
	for row := range csv {
		for column := range csv[row] {
			if row < cfg.NumHeaderRows {
				if column < cfg.NumLabelCols {
					// write header rows to labels, no offset
					levelNames[column][row] = csv[row][column]
				} else {
					// write header rows to cols, offset for label cols
					offsetFromLabelCols := column - cfg.NumLabelCols
					valsNames[offsetFromLabelCols][row] = csv[row][column]
				}
				continue
			}
			offsetFromHeaderRows := row - cfg.NumHeaderRows
			if column < cfg.NumLabelCols {
				// write values to labels, offset for header rows
				labels[column][offsetFromHeaderRows] = csv[row][column]
				labelsIsNull[column][offsetFromHeaderRows] = isNullString(csv[row][column])
			} else {
				offsetFromLabelCols := column - cfg.NumLabelCols
				// write values to cols, offset for label cols and header rows
				vals[offsetFromLabelCols][offsetFromHeaderRows] = csv[row][column]
				valsIsNull[offsetFromLabelCols][offsetFromHeaderRows] = isNullString(csv[row][column])
			}
		}
	}

	retNames := make([]string, len(valsNames))
	for k := range valsNames {
		retNames[k] = strings.Join(valsNames[k], optionLevelSeparator)
	}
	retLevelNames := make([]string, len(levelNames))
	for k := range levelNames {
		retLevelNames[k] = strings.Join(levelNames[k], optionLevelSeparator)
	}

	// transfer values and labels to final value containers
	retVals := copyStringsIntoValueContainers(vals, valsIsNull, retNames)
	retLabels := copyStringsIntoValueContainers(labels, labelsIsNull, retLevelNames)

	// create default labels if no labels
	retLabels = defaultLabelsIfEmpty(retLabels, numRows)

	// create default col level names
	var retColLevelNames []string
	retColLevelNames, retVals = defaultColsIfNoHeader(cfg.NumHeaderRows, retVals)

	return &DataFrame{
		values:        retVals,
		labels:        retLabels,
		colLevelNames: retColLevelNames,
	}
}

// expects non-nil cfg
func readCSVByCols(csv [][]string, cfg *ReadConfig) *DataFrame {
	numRows := len(csv[0]) - cfg.NumHeaderRows
	numCols := len(csv) - cfg.NumLabelCols

	// prepare intermediary values containers
	vals := make([][]string, numCols)
	valsIsNull := make([][]bool, numCols)
	valsNames := make([]string, numCols)

	// prepare intermediary label containers
	labels := make([][]string, cfg.NumLabelCols)
	labelsIsNull := make([][]bool, cfg.NumLabelCols)
	labelsNames := make([]string, cfg.NumLabelCols)

	// iterate over all cols to get header names
	for j := 0; j < cfg.NumLabelCols; j++ {
		// write label headers, no offset
		labelsNames[j] = strings.Join(csv[j][:cfg.NumHeaderRows], optionLevelSeparator)
	}
	for k := 0; k < numCols; k++ {
		// write col headers, offset for label cols
		offsetFromLabelCols := k + cfg.NumLabelCols
		valsNames[k] = strings.Join(csv[offsetFromLabelCols][:cfg.NumHeaderRows], optionLevelSeparator)
	}
	for column := range csv {
		if column < cfg.NumLabelCols {
			// write label values as slice, offset for header rows
			valsToWrite := csv[column][cfg.NumHeaderRows:]
			labels[column] = valsToWrite
			labelsIsNull[column] = setNullsFromInterface(valsToWrite)
		} else {
			// write column values as slice, offset for label cols and header rows
			offsetFromLabelCols := column - cfg.NumLabelCols
			valsToWrite := csv[column][cfg.NumHeaderRows:]
			vals[offsetFromLabelCols] = valsToWrite
			valsIsNull[offsetFromLabelCols] = setNullsFromInterface(valsToWrite)
		}
	}

	// transfer values and labels to final value containers
	retVals := copyStringsIntoValueContainers(vals, valsIsNull, valsNames)
	retLabels := copyStringsIntoValueContainers(labels, labelsIsNull, labelsNames)

	// create default labels if no labels
	retLabels = defaultLabelsIfEmpty(retLabels, numRows)

	// create default col level names
	var retColLevelNames []string
	retColLevelNames, retVals = defaultColsIfNoHeader(cfg.NumHeaderRows, retVals)

	return &DataFrame{
		values:        retVals,
		labels:        retLabels,
		colLevelNames: retColLevelNames,
	}

}
func defaultLabelsIfEmpty(labels []*valueContainer, numRows int) []*valueContainer {
	if len(labels) == 0 {
		defaultLabels := makeDefaultLabels(0, numRows)
		labels = append(labels, defaultLabels)
	}
	return labels
}
func defaultColsIfNoHeader(numHeaderRows int, columns []*valueContainer) ([]string, []*valueContainer) {
	if numHeaderRows <= 0 {
		// if no header rows, change the names of the columns to be 0, 1...
		for k := range columns {
			columns[k].name = fmt.Sprintf("%v", k)
		}
		return []string{"*0"}, columns
	}
	// if header rows, set the col level names to *0, *1...
	ret := make([]string, numHeaderRows)
	for l := range ret {
		ret[l] = fmt.Sprintf("*%d", l)
	}
	return ret, columns
}

func defaultConfigIfNil(config *ReadConfig) *ReadConfig {
	if config == nil {
		config = &ReadConfig{NumHeaderRows: 1}
	}
	return config
}

func readStruct(slice interface{}) ([]*valueContainer, error) {
	if !isSlice(slice) {
		return nil, fmt.Errorf("unsupported kind (%v); must be slice", reflect.TypeOf(slice).Kind())
	}
	if kind := reflect.TypeOf(slice).Elem().Kind(); kind != reflect.Struct {
		return nil, fmt.Errorf("unsupported kind (%v); must be slice of structs", reflect.TypeOf(slice).Elem().Kind())
	}
	v := reflect.ValueOf(slice)
	if v.Len() == 0 {
		return nil, fmt.Errorf("slice must contain at least one struct")
	}
	strct := v.Index(0)
	numCols := strct.NumField()
	retValues := make([][]string, numCols)
	retNames := make([]string, numCols)
	for k := 0; k < numCols; k++ {
		for i := 0; i < v.Len(); i++ {
			strct := v.Index(i)
			if i == 0 {
				retNames[k] = strct.Type().Field(k).Name
				retValues[k] = make([]string, v.Len())
			}
			retValues[k][i] = fmt.Sprint(strct.Field(k).Interface())
		}
	}
	// transfer to final container
	ret := make([]*valueContainer, numCols)
	for k := range ret {
		ret[k] = &valueContainer{
			slice:  retValues[k],
			isNull: setNullsFromInterface(retValues[k]),
			name:   retNames[k],
		}
	}
	return ret, nil
}

func inferType(input string) DType {
	if _, err := dateparse.ParseAny(input); err == nil {
		return DateTime
	}
	if _, err := strconv.ParseFloat(input, 64); err == nil {
		return Float
	}
	return String
}

func mockCSVFromDTypes(dtypes []map[DType]int, numMockRows int) [][]string {
	rand.Seed(randSeed)
	dominantDTypes := make([]DType, len(dtypes))
	var highestCount int
	var dominantDType DType
	// determine the dominant data type per column
	for k := range dtypes {
		for key, v := range dtypes[k] {
			if v > highestCount {
				dominantDType = key
			}
		}
		dominantDTypes[k] = dominantDType
	}
	ret := make([][]string, numMockRows)
	for i := range ret {
		ret[i] = make([]string, len(dtypes))
		for k := range ret[i] {
			ret[i][k] = mockString(dominantDTypes[k])
		}
	}
	return ret
}

func mockString(dtype DType) string {
	var options []string
	switch dtype {
	case Float:
		options = []string{"1", "2", "3"}
	case DateTime:
		options = []string{"12/1/2019", "1/1/2020", "2/1/2020"}
	case String:
		options = []string{"foo", "bar", "baz"}
	}
	nullPct := .2
	f := rand.Float64()
	if f < nullPct {
		return ""
	}
	randomIndex := rand.Intn(len(options))
	return options[randomIndex]
}

func (vc *valueContainer) valid() []int {
	index := make([]int, 0)
	for i, isNull := range vc.isNull {
		if !isNull {
			index = append(index, i)
		}
	}
	return index
}

func (vc *valueContainer) null() []int {
	index := make([]int, 0)
	for i, isNull := range vc.isNull {
		if isNull {
			index = append(index, i)
		}
	}
	return index
}

// subsetRows modifies vc in place to contain ony the rows specified by index.
// If any position is out of range, returns an error
func (vc *valueContainer) subsetRows(index []int) error {
	v := reflect.ValueOf(vc.slice)
	l := v.Len()
	retIsNull := make([]bool, len(index))
	retVals := reflect.MakeSlice(v.Type(), len(index), len(index))
	// []int{1, 5}
	// indexPosition: [0, 1], indexValue: [1,5]
	for indexPosition, indexValue := range index {
		if indexValue >= l {
			return fmt.Errorf("index out of range (%d > %d)", indexValue, l-1)
		}
		retIsNull[indexPosition] = vc.isNull[indexValue]
		ptr := retVals.Index(indexPosition)
		ptr.Set(v.Index(indexValue))
	}

	vc.slice = retVals.Interface()
	vc.isNull = retIsNull
	return nil
}

// subsetCols returns a new set of valueContainers containing only the columns specified by index.
// If any position is out of range, returns error
func subsetCols(cols []*valueContainer, index []int) ([]*valueContainer, error) {
	retLabels := make([]*valueContainer, len(index))
	for indexPosition, indexValue := range index {
		if indexValue >= len(cols) {
			return nil, fmt.Errorf("index out of range (%d > %d)", indexValue, len(cols)-1)
		}
		retLabels[indexPosition] = cols[indexValue]
	}
	return retLabels, nil
}

// head returns the first number of rows specified by `n`
func (vc *valueContainer) head(n int) *valueContainer {
	v := reflect.ValueOf(vc.slice)
	var retIsNull []bool
	retVals := v.Slice(0, n)
	retIsNull = vc.isNull[:n]

	return &valueContainer{
		slice:  retVals.Interface(),
		isNull: retIsNull,
		name:   vc.name,
	}
}

// tail returns the last number of rows specified by `n`
func (vc *valueContainer) tail(n int) *valueContainer {
	v := reflect.ValueOf(vc.slice)
	var retIsNull []bool
	retVals := v.Slice(len(vc.isNull)-n, len(vc.isNull))
	retIsNull = vc.isNull[len(vc.isNull)-n : len(vc.isNull)]

	return &valueContainer{
		slice:  retVals.Interface(),
		isNull: retIsNull,
		name:   vc.name,
	}
}

// rangeSlice returns the rows starting with first and ending with last (exclusive)
func (vc *valueContainer) rangeSlice(first, last int) *valueContainer {
	v := reflect.ValueOf(vc.slice)
	var retIsNull []bool
	retVals := v.Slice(first, last)
	retIsNull = vc.isNull[first:last]

	return &valueContainer{
		slice:  retVals.Interface(),
		isNull: retIsNull,
		name:   vc.name,
	}
}

func (vc *valueContainer) shift(n int) *valueContainer {
	v := reflect.ValueOf(vc.slice)
	vals := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
	isNull := make([]bool, v.Len())
	for i := 0; i < v.Len(); i++ {
		position := i - n
		if position < 0 || position >= v.Len() {
			vals.Index(i).Set(reflect.Zero(reflect.TypeOf(vc.slice).Elem()))
			isNull[i] = true
		} else {
			vals.Index(i).Set(v.Index(position))
			isNull[i] = vc.isNull[position]
		}
	}
	return &valueContainer{
		slice:  vals.Interface(),
		isNull: isNull,
		name:   vc.name,
	}
}

// convert to string as lowest common denominator
func (vc *valueContainer) append(other *valueContainer) *valueContainer {
	retSlice := append(vc.str().slice, other.str().slice...)
	retIsNull := append(vc.isNull, other.isNull...)
	return &valueContainer{
		slice:  retSlice,
		isNull: retIsNull,
		name:   vc.name,
	}
}

func (vc *valueContainer) iterRow(index int) Element {
	return Element{
		Val:    reflect.ValueOf(vc.slice).Index(index).Interface(),
		IsNull: vc.isNull[index]}
}

func (vc *valueContainer) gt(comparison float64) []int {
	index, _ := vc.filter(FilterFn{F64: func(v float64) bool {
		if v > comparison {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) lt(comparison float64) []int {
	index, _ := vc.filter(FilterFn{F64: func(v float64) bool {
		if v < comparison {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) gte(comparison float64) []int {
	index, _ := vc.filter(FilterFn{F64: func(v float64) bool {
		if v >= comparison {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) lte(comparison float64) []int {
	index, _ := vc.filter(FilterFn{F64: func(v float64) bool {
		if v <= comparison {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) floateq(comparison float64) []int {
	index, _ := vc.filter(FilterFn{F64: func(v float64) bool {
		if v == comparison {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) floatneq(comparison float64) []int {
	index, _ := vc.filter(FilterFn{F64: func(v float64) bool {
		if v != comparison {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) eq(comparison string) []int {
	index, _ := vc.filter(FilterFn{String: func(v string) bool {
		if v == comparison {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) neq(comparison string) []int {
	index, _ := vc.filter(FilterFn{String: func(v string) bool {
		if v != comparison {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) contains(substr string) []int {
	index, _ := vc.filter(FilterFn{String: func(v string) bool {
		if strings.Contains(v, substr) {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) before(comparison time.Time) []int {
	index, _ := vc.filter(FilterFn{DateTime: func(v time.Time) bool {
		if v.Before(comparison) {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) after(comparison time.Time) []int {
	index, _ := vc.filter(FilterFn{DateTime: func(v time.Time) bool {
		if v.After(comparison) {
			return true
		}
		return false
	}})
	return index
}

func (vc *valueContainer) filter(filter FilterFn) ([]int, error) {
	var index []int
	if filter.F64 != nil {
		slice := vc.float().slice
		for i := range slice {
			if filter.F64(slice[i]) && !vc.isNull[i] {
				index = append(index, i)
			}
		}
	} else if filter.String != nil {
		slice := vc.str().slice
		for i := range slice {
			if filter.String(slice[i]) && !vc.isNull[i] {
				index = append(index, i)
			}
		}
	} else if filter.DateTime != nil {
		slice := vc.dateTime().slice
		for i := range slice {
			if filter.DateTime(slice[i]) && !vc.isNull[i] {
				index = append(index, i)
			}
		}
	} else {
		return nil, fmt.Errorf("no filter function provided")
	}
	return index, nil
}

func (vc *valueContainer) applyFormat(apply ApplyFormatFn) interface{} {
	var ret interface{}
	if apply.F64 != nil {
		slice := vc.float().slice
		retSlice := make([]string, len(slice))
		for i := range slice {
			retSlice[i] = apply.F64(slice[i])
		}
		ret = retSlice
	} else if apply.DateTime != nil {
		slice := vc.dateTime().slice
		retSlice := make([]string, len(slice))
		for i := range slice {
			retSlice[i] = apply.DateTime(slice[i])
		}
		ret = retSlice
	}
	return ret
}

func (vc *valueContainer) apply(apply ApplyFn) interface{} {
	var ret interface{}
	if apply.F64 != nil {
		slice := vc.float().slice
		retSlice := make([]float64, len(slice))
		for i := range slice {
			retSlice[i] = apply.F64(slice[i])
		}
		ret = retSlice
	} else if apply.String != nil {
		slice := vc.str().slice
		retSlice := make([]string, len(slice))
		for i := range slice {
			retSlice[i] = apply.String(slice[i])
		}
		ret = retSlice
	} else if apply.DateTime != nil {
		slice := vc.dateTime().slice
		retSlice := make([]time.Time, len(slice))
		for i := range slice {
			retSlice[i] = apply.DateTime(slice[i])
		}
		ret = retSlice
	}
	return ret
}

// expects slices to be same-lengthed; if either is true at position x, ret is true at position x
func isEitherNull(isNull1, isNull2 []bool) []bool {
	ret := make([]bool, len(isNull1))
	for i := 0; i < len(isNull1); i++ {
		ret[i] = isNull1[i] || isNull2[i]
	}
	return ret
}

func (vc *valueContainer) sort(dtype DType, descending bool, index []int) []int {
	var srt sort.Interface
	nulls := make([]int, 0)
	notNulls := make([]int, 0)
	var sortedIsNull []bool
	var sortedIndex []int
	switch dtype {
	case Float:
		d := vc.float()
		d.index = index
		srt = d
		if descending {
			srt = sort.Reverse(srt)
		}
		sort.Stable(srt)
		sortedIsNull = d.isNull
		sortedIndex = d.index

	case String:
		d := vc.str()
		d.index = index
		srt = d
		if descending {
			srt = sort.Reverse(srt)
		}
		sort.Stable(srt)
		sortedIsNull = d.isNull
		sortedIndex = d.index

	case DateTime:
		d := vc.dateTime()
		d.index = index
		srt = d
		if descending {
			srt = sort.Reverse(srt)
		}
		sort.Stable(srt)
		sortedIsNull = d.isNull
		sortedIndex = d.index
	}
	// move all null values to the bottom
	for i := range sortedIsNull {
		if sortedIsNull[i] {
			nulls = append(nulls, sortedIndex[i])
		} else {
			notNulls = append(notNulls, sortedIndex[i])
		}
	}
	return append(notNulls, nulls...)
}

// convertColNamesToIndexPositions converts a slice of label or column names to index positions.
// If any name is not in the set of columns, returns an error
func convertColNamesToIndexPositions(names []string, columns []*valueContainer) ([]int, error) {
	ret := make([]int, len(names))
	for i, name := range names {
		lvl, err := findColWithName(name, columns)
		if err != nil {
			return nil, err
		}
		ret[i] = lvl
	}
	return ret, nil
}

// concatenateLabelsToStrings reduces all label levels referenced in the index to a single slice of concatenated strings
func concatenateLabelsToStrings(labels []*valueContainer, index []int) []string {
	sep := "|"
	labelStrings := make([][]string, len(index))
	// coerce every label level referenced in the index to a separate string slice
	for j := range index {
		labelStrings[j] = labels[j].str().slice
	}
	ret := make([]string, len(labelStrings[0]))
	// for each row, combine labels into one concatenated string
	for i := 0; i < len(labelStrings[0]); i++ {
		labelComponents := make([]string, len(index))
		for j := range index {
			labelComponents[j] = labelStrings[j][i]
		}
		concatenatedString := strings.Join(labelComponents, sep)
		ret[i] = concatenatedString
	}
	// return a single slice of strings
	return ret
}

// labelsToMap reduces all label levels referenced in the index to two maps and a slice of strings:
// 1) map[string][]int: the key is a single concatenated string of the labels
// and the value is an integer slice of all the positions where the key appears in the labels, preserving the order in which they appear,
// 2) map[string]int: same key as above, but the value is the integer of the first position where the key appears in the labels.
// 3) []string: the map keys in the order in which they appear in the Series
func labelsToMap(labels []*valueContainer, index []int) (
	allIndex map[string][]int, firstIndex map[string]int,
	orderedKeys []string, originalIndexToUniqueIndex map[int]int) {
	sep := "|"
	// coerce all label levels referenced in the index to string
	labelStrings := make([][]string, len(index))
	for j := range index {
		labelStrings[j] = labels[index[j]].str().slice
	}
	allIndex = make(map[string][]int)
	firstIndex = make(map[string]int)
	orderedKeys = make([]string, 0)
	originalIndexToUniqueIndex = make(map[int]int, len(labelStrings[0]))
	orderedKeyIndex := make(map[string]int)
	// for each row, combine labels into a single string
	l := len(labelStrings[0])
	for i := 0; i < l; i++ {
		labelComponents := make([]string, len(labelStrings))
		for j := range labelStrings {
			labelComponents[j] = labelStrings[j][i]
		}
		key := strings.Join(labelComponents, sep)
		if _, ok := allIndex[key]; !ok {
			allIndex[key] = []int{i}
			firstIndex[key] = i
			orderedKeyIndex[key] = len(orderedKeys)
			orderedKeys = append(orderedKeys, key)
		} else {
			allIndex[key] = append(allIndex[key], i)
		}
		originalIndexToUniqueIndex[i] = orderedKeyIndex[key]
	}
	return
}

// if labels1[i] is in labels2, ret[i] = labels2[labels1[i]], else ret[i] = -1
func matchLabelPositions(labels1 []string, labels2 map[string]int) []int {
	ret := make([]int, len(labels1))
	for i, key := range labels1 {
		if val, ok := labels2[key]; ok {
			ret[i] = val
		} else {
			ret[i] = -1
		}
	}
	return ret
}

func (s *Series) combineMath(other *Series, ignoreMissing bool, fn func(v1 float64, v2 float64) float64) *Series {
	retFloat := make([]float64, s.Len())
	retIsNull := make([]bool, s.Len())
	lookupVals := s.Lookup(other)
	lookupFloat := lookupVals.values.float().slice
	lookupNulls := lookupVals.values.isNull
	originalFloat := s.values.float().slice
	originalNulls := s.values.isNull
	for i := range originalFloat {
		// handle null lookup
		if lookupNulls[i] {
			if ignoreMissing {
				retFloat[i] = originalFloat[i]
				retIsNull[i] = originalNulls[i]
				continue
			}
			retFloat[i] = 0
			retIsNull[i] = true
			continue
		}
		// actual combination logic
		combinedFloat := fn(originalFloat[i], lookupFloat[i])
		// handle division by 0
		if math.IsNaN(combinedFloat) || math.IsInf(combinedFloat, 0) {
			combinedFloat = 0
			retIsNull[i] = true
			continue
		}
		retFloat[i] = combinedFloat
		retIsNull[i] = originalNulls[i]
	}
	return &Series{values: &valueContainer{slice: retFloat, isNull: retIsNull}, labels: s.labels}
}

func lookup(how string,
	values1 *valueContainer, labels1 []*valueContainer, leftOn []int,
	values2 *valueContainer, labels2 []*valueContainer, rightOn []int) (*Series, error) {
	switch how {
	case "left":
		return lookupWithAnchor(values1.name, labels1, leftOn, values2, labels2, rightOn), nil
	case "right":
		return lookupWithAnchor(values2.name, labels2, rightOn, values1, labels1, leftOn), nil
	case "inner":
		s := lookupWithAnchor(values1.name, labels1, leftOn, values2, labels2, rightOn)
		s = s.DropNull()
		return s, nil
	default:
		return nil, fmt.Errorf("`how`: must be `left`, `right`, or `inner`")
	}
}

func lookupDataFrame(how string,
	name string, colLevelNames []string,
	values1 []*valueContainer, labels1 []*valueContainer, leftOn []int,
	values2 []*valueContainer, labels2 []*valueContainer, rightOn []int,
	excludeLeft []string, excludeRight []string) (*DataFrame, error) {
	mergedLabelsCols1 := append(labels1, values1...)
	mergedLabelsCols2 := append(labels2, values2...)
	switch how {
	case "left":
		return lookupDataFrameWithAnchor(name, colLevelNames,
			mergedLabelsCols1, labels1, leftOn,
			values2, mergedLabelsCols2, rightOn, excludeRight), nil
	case "right":
		return lookupDataFrameWithAnchor(name, colLevelNames,
			mergedLabelsCols2, labels2, rightOn,
			values1, mergedLabelsCols1, leftOn, excludeLeft), nil
	case "inner":
		df := lookupDataFrameWithAnchor(name, colLevelNames,
			mergedLabelsCols1, labels1, leftOn,
			values2, mergedLabelsCols2, rightOn, excludeRight)
		df = df.DropNull()
		return df, nil
	default:
		return nil, fmt.Errorf("`how`: must be `left`, `right`, or `inner`")
	}
}

// cuts labels by leftOn and rightOn, anchors to labels in labels1, finds matches in labels2
// looks up values in values2, converts to Series and preserves the name from values1
func lookupWithAnchor(
	name string, labels1 []*valueContainer, leftOn []int,
	values2 *valueContainer, labels2 []*valueContainer, rightOn []int) *Series {
	toLookup := concatenateLabelsToStrings(labels1, leftOn)
	_, lookupSource, _, _ := labelsToMap(labels2, rightOn)
	matches := matchLabelPositions(toLookup, lookupSource)
	v := reflect.ValueOf(values2.slice)
	isNull := make([]bool, len(matches))
	// return type is set to same type as within lookupSource
	vals := reflect.MakeSlice(v.Type(), len(matches), len(matches))
	for i, matchedIndex := range matches {
		// positive match: copy value from values2
		if matchedIndex != -1 {
			vals.Index(i).Set(v.Index(matchedIndex))
			isNull[i] = values2.isNull[i]
			// no match: set to zero value
		} else {
			vals.Index(i).Set(reflect.Zero(reflect.TypeOf(values2.slice).Elem()))
			isNull[i] = true
		}
	}
	return &Series{
		values: &valueContainer{slice: vals.Interface(), isNull: isNull, name: name},
		labels: labels1,
	}
}

// cuts labels by leftOn and rightOn, anchors to labels in labels1, finds matches in labels2
// looks up values in every column in values2 (excluding colNames matching `except`),
// preserves the column names from values2, converts to dataframe with `name`
func lookupDataFrameWithAnchor(
	name string, colLevelNames []string,
	mergedLabelsCols1 []*valueContainer, labels1 []*valueContainer, leftOn []int,
	values2 []*valueContainer, mergedLabelsCols2 []*valueContainer, rightOn []int, exclude []string) *DataFrame {
	toLookup := concatenateLabelsToStrings(mergedLabelsCols1, leftOn)
	_, lookupSource, _, _ := labelsToMap(mergedLabelsCols2, rightOn)
	matches := matchLabelPositions(toLookup, lookupSource)
	// slice of slices
	var retVals []*valueContainer
	for k := range values2 {
		var skip bool
		for _, exclusion := range exclude {
			// skip any column whose name is also used in the lookup
			if values2[k].name == exclusion {
				skip = true
			}
		}
		if skip {
			continue
		}
		v := reflect.ValueOf(values2[k].slice)
		isNull := make([]bool, len(matches))
		// return type is set to same type as within lookupSource
		vals := reflect.MakeSlice(v.Type(), len(matches), len(matches))
		for i, matchedIndex := range matches {
			// positive match: copy value from values2
			if matchedIndex != -1 {
				vals.Index(i).Set(v.Index(matchedIndex))
				isNull[i] = values2[k].isNull[matchedIndex]
				// no match
			} else {
				vals.Index(i).Set(reflect.Zero(reflect.TypeOf(values2[k].slice).Elem()))
				isNull[i] = true
			}
		}
		retVals = append(retVals, &valueContainer{slice: vals.Interface(), isNull: isNull, name: values2[k].name})
	}
	return &DataFrame{
		values:        retVals,
		labels:        labels1,
		name:          name,
		colLevelNames: colLevelNames,
	}
}

func (vc *valueContainer) dropRow(index int) error {
	v := reflect.ValueOf(vc.slice)
	l := v.Len()
	if index >= l {
		return fmt.Errorf("index out of range (%d > %d)", index, l-1)
	}
	retIsNull := append(vc.isNull[:index], vc.isNull[index+1:]...)
	retVals := reflect.MakeSlice(v.Type(), 0, 0)
	retVals = reflect.AppendSlice(v.Slice(0, index), v.Slice(index+1, l))

	vc.slice = retVals.Interface()
	vc.isNull = retIsNull
	return nil
}

func excludeFromIndex(indexLength int, item int) []int {
	var ret []int
	for i := 0; i < indexLength; i++ {
		if i != item {
			ret = append(ret, i)
		}
	}
	return ret
}

func (vc *valueContainer) copy() *valueContainer {
	v := reflect.ValueOf(vc.slice)
	vals := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
	for i := 0; i < v.Len(); i++ {
		ptr := vals.Index(i)
		ptr.Set(v.Index(i))
	}
	isNull := make([]bool, len(vc.isNull))
	copy(isNull, vc.isNull)
	return &valueContainer{
		slice:  vals.Interface(),
		isNull: isNull,
		name:   vc.name,
	}
}

func setNullsFromInterface(input interface{}) []bool {
	var ret []bool
	if reflect.TypeOf(input).Kind() != reflect.Slice {
		return nil
	}
	switch input.(type) {
	case []float64:
		vals := input.([]float64)
		ret = make([]bool, len(vals))
		for i := range ret {
			if math.IsNaN(vals[i]) {
				ret[i] = true
			} else {
				ret[i] = false
			}
		}

	case []string:
		vals := input.([]string)
		ret = make([]bool, len(vals))
		for i := range ret {
			if isNullString(vals[i]) {
				ret[i] = true
			} else {
				ret[i] = false
			}
		}
	case []time.Time:
		vals := input.([]time.Time)
		ret = make([]bool, len(vals))
		for i := range ret {
			if (time.Time{}) == vals[i] {
				ret[i] = true
			} else {
				ret[i] = false
			}
		}

	case []interface{}:
		vals := input.([]interface{})
		ret = make([]bool, len(vals))
		for i := range ret {
			if isNullInterface(vals[i]) {
				ret[i] = true
			} else {
				ret[i] = false
			}
		}
		// no null value possible
	case []bool, []uint, []uint8, []uint16, []uint32, []uint64, []int, []int8, []int16, []int32, []int64, []float32:
		l := reflect.ValueOf(input).Len()
		ret = make([]bool, l)
		for i := range ret {
			ret[i] = false
		}
	case []Element:
		vals := input.([]Element)
		ret = make([]bool, len(vals))
		for i := range ret {
			ret[i] = vals[i].IsNull
		}
	default:
		return nil
	}
	return ret
}

func isNullInterface(i interface{}) bool {
	switch i.(type) {
	case float64:
		f := i.(float64)
		if math.IsNaN(f) {
			return true
		}
	case string:
		s := i.(string)
		if isNullString(s) {
			return true
		}
	case time.Time:
		t := i.(time.Time)
		if (time.Time{}) == t {
			return true
		}
	}
	return false
}

func isNullString(s string) bool {
	nullStrings := []string{"NaN", "n/a", "N/A", "", "nil"}
	for _, ns := range nullStrings {
		if strings.TrimSpace(s) == ns {
			return true
		}
	}
	return false
}

// math

// sum sums the non-null values at the index positions in `vals`. If all values are null, the final result is null.
// Compatible with Grouped calculations as well as Series
func sum(vals []float64, isNull []bool, index []int) (float64, bool) {
	var sum float64
	var atLeastOneValid bool
	for _, i := range index {
		if !isNull[i] {
			sum += vals[i]
			atLeastOneValid = true
		}
	}
	if !atLeastOneValid {
		return 0, true
	}
	return sum, false
}

// mean calculates the mean of the non-null values at the index positions in `vals`.
// If all values are null, the final result is null.
// Compatible with Grouped calculations as well as Series
func mean(vals []float64, isNull []bool, index []int) (float64, bool) {
	var sum float64
	var counter float64
	var atLeastOneValid bool
	for _, i := range index {
		if !isNull[i] {
			sum += vals[i]
			counter++
			atLeastOneValid = true
		}
	}
	if !atLeastOneValid {
		return 0, true
	}
	return sum / counter, false
}

// median calculates the median of the non-null values at the index positions in `vals`.
// If all values are null, the final result is null.
// Compatible with Grouped calculations as well as Series
func median(vals []float64, isNull []bool, index []int) (float64, bool) {
	data := make([]float64, 0)
	for _, i := range index {
		if !isNull[i] {
			data = append(data, vals[i])
		}
	}
	sort.Float64s(data)
	if len(data) == 0 {
		return math.NaN(), true
	}
	// rounds down if there are even number of elements
	mNumber := len(data) / 2

	// odd number of elements
	if len(data)%2 != 0 {
		return data[mNumber], false
	}
	// even number of elements
	return (data[mNumber-1] + data[mNumber]) / 2, false
}

// std calculates the standard deviation of the non-null values at the index positions in `vals`.
// If all values are null, the final result is null.
// Compatible with Grouped calculations as well as Series
func std(vals []float64, isNull []bool, index []int) (float64, bool) {
	mean, _ := mean(vals, isNull, index)
	var variance, counter float64
	var atLeastOneValid bool
	for _, i := range index {
		if !isNull[i] {
			variance += math.Pow((vals[i] - mean), 2)
			counter++
			atLeastOneValid = true
		}
	}
	if !atLeastOneValid {
		return 0, true
	}
	return math.Pow(variance/counter, 0.5), false
}

// count counts the non-null values at the `index` positions in `vals`.
// Compatible with Grouped calculations as well as Series
func count(vals []float64, isNull []bool, index []int) (float64, bool) {
	var counter int
	var atLeastOneValid bool
	for _, i := range index {
		if !isNull[i] {
			counter++
			atLeastOneValid = true
		}
	}
	if !atLeastOneValid {
		return 0, true
	}
	return float64(counter), false
}

// min returns the min of the non-null values at the `index` positions in `vals`.
// Compatible with Grouped calculations as well as Series
func min(vals []float64, isNull []bool, index []int) (float64, bool) {
	min := math.Inf(0)
	var atLeastOneValid bool
	for _, i := range index {
		if !isNull[i] {
			if vals[i] < min {
				min = vals[i]
			}
			atLeastOneValid = true
		}
	}
	if !atLeastOneValid {
		return 0, true
	}
	return min, false
}

// max returns the max of the non-null values at the `index` positions in `vals`.
// Compatible with Grouped calculations as well as Series
func max(vals []float64, isNull []bool, index []int) (float64, bool) {
	max := math.Inf(-1)
	var atLeastOneValid bool
	for _, i := range index {
		if !isNull[i] {
			if vals[i] > max {
				max = vals[i]
			}
			atLeastOneValid = true
		}
	}
	if !atLeastOneValid {
		return 0, true
	}
	return max, false
}

// earliest returns the earliest of the non-null values at the `index` positions in `vals`.
// Compatible with Grouped calculations as well as Series
func earliest(vals []time.Time, isNull []bool, index []int) (time.Time, bool) {
	min := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	var atLeastOneValid bool
	for _, i := range index {
		if !isNull[i] {
			if vals[i].Before(min) {
				min = vals[i]
			}
			atLeastOneValid = true
		}
	}
	if !atLeastOneValid {
		return time.Time{}, true
	}
	return min, false
}

// latest returns the latest of the non-null values at the `index` positions in `vals`.
// Compatible with Grouped calculations as well as Series
func latest(vals []time.Time, isNull []bool, index []int) (time.Time, bool) {
	max := time.Time{}
	var atLeastOneValid bool
	for _, i := range index {
		if !isNull[i] {
			if vals[i].After(max) {
				max = vals[i]
			}
			atLeastOneValid = true
		}
	}
	if !atLeastOneValid {
		return time.Time{}, true
	}
	return max, false
}

// returns the first non-null value as a string
func first(vals []string, isNull []bool, index []int) (string, bool) {
	for _, i := range index {
		if !isNull[i] {
			return vals[i], false
		}
	}
	return "", true
}

// returns the last non-null value as a string
func last(vals []string, isNull []bool, index []int) (string, bool) {
	for i := len(index) - 1; i >= 0; i-- {
		if !isNull[index[i]] {
			return vals[index[i]], false
		}
	}
	return "", true
}

// cumsum is an aligned function, meaning it aligns with the original rows
func cumsum(vals []float64, isNull []bool, index []int) []float64 {
	ret := make([]float64, len(index))
	var cumsum float64
	for incrementor, i := range index {
		if !isNull[i] {
			cumsum += vals[i]
		}
		ret[incrementor] = cumsum
	}
	return ret
}

func (lambda ApplyFn) validate() error {
	if lambda.F64 == nil {
		if lambda.String == nil {
			if lambda.DateTime == nil {
				return fmt.Errorf("no apply function provided")
			}
		}
	}
	return nil
}

func (lambda ApplyFormatFn) validate() error {
	if lambda.F64 == nil {
		if lambda.DateTime == nil {
			return fmt.Errorf("no apply function provided")
		}
	}
	return nil
}

// left-exclusive, right-inclusive by default
// expects vals and isNull to be same length
func cut(vals []float64, isNull []bool,
	bins []float64, leftInclusive bool, rightExclusive bool,
	includeLess, includeMore bool, labels []string) ([]string, error) {
	if len(bins) == 0 {
		return nil, fmt.Errorf("must supply at least one bin edge")
	}
	originalBinCount := len(bins)
	var useDefaultLabels bool
	// create default labels
	if len(labels) == 0 {
		useDefaultLabels = true
		labels = make([]string, len(bins)-1)
		// do not iterate over the last edge to avoid range error
		for i := 0; i < len(bins)-1; i++ {
			labels[i] = fmt.Sprintf("%v-%v", bins[i], bins[i+1])
		}
	}
	if includeLess {
		if useDefaultLabels {
			str := "<=%v"
			if leftInclusive {
				str = "<%v"
			}
			labels = append([]string{fmt.Sprintf(str, bins[0])}, labels...)
		}
		bins = append([]float64{math.Inf(-1)}, bins...)
	}
	if includeMore {
		if useDefaultLabels {
			str := ">%v"
			if rightExclusive {
				str = ">=%v"
			}
			labels = append(labels, fmt.Sprintf(str, bins[len(bins)-1]))
		}
		bins = append(bins, math.Inf(1))
	}
	// validate correct number of bins
	var andLessEdge, andMoreEdge int
	if includeLess {
		andLessEdge = 1
	}
	if includeMore {
		andMoreEdge = 1
	}
	if len(bins)-1 != len(labels) {
		return nil, fmt.Errorf("number of bin edges (+ includeLess + includeMore), "+
			"must be one more than number of supplied labels: (%d + %d + %d) != (%d + 1)",
			originalBinCount, andLessEdge, andMoreEdge, len(labels))
	}
	ret := make([]string, len(vals))
	for i, val := range vals {
		if isNull[i] {
			continue
		}
		// do not iterate over the last edge to avoid range error
		for binEdge := 0; binEdge < len(bins)-1; binEdge++ {
			// check if value is within bin
			if leftInclusive && rightExclusive {
				if val >= bins[binEdge] && val < bins[binEdge+1] {
					ret[i] = labels[binEdge]
					break
				}
			} else if !leftInclusive && !rightExclusive {
				// default: leftExclusive and rightInclusive
				if val > bins[binEdge] && val <= bins[binEdge+1] {
					ret[i] = labels[binEdge]
					break
				}
			} else {
				// avoid this scenario
				return nil, fmt.Errorf("internal error: bad cut() conditions")
			}
		}
	}

	return ret, nil
}

func (vc *valueContainer) cut(bins []float64, includeLess, includeMore bool, labels []string) ([]string, error) {
	leftInclusive := false
	rightExclusive := false
	return cut(vc.float().slice, vc.isNull, bins, leftInclusive, rightExclusive, includeLess, includeMore, labels)
}

func (vc *floatValueContainer) rank() []float64 {
	ret := make([]float64, len(vc.slice))
	sort.Stable(vc)
	var offset float64
	// iterate over sorted values and write results back to []float aligned with original
	// the incrementor here is the naive ranking, but must be adjsuted for nulls and duplicates
	for i := range vc.slice {
		originalPosition := vc.index[i]
		// ranking is 1-based, index is 0-based
		rank := float64(i) + 1

		// handle null
		if vc.isNull[i] {
			ret[originalPosition] = -999
			offset--
			continue
		}
		// no duplicates prior to first value
		if i == 0 {
			ret[originalPosition] = rank
			continue
		}
		// handle duplicates
		if vc.slice[i] == vc.slice[i-1] {
			priorOriginal := vc.index[i-1]
			// if duplicate, look up original position immediately prior and increment the offset
			ret[originalPosition] = ret[priorOriginal]
			offset--
		} else {
			// reduce the rank by the offset amount
			ret[originalPosition] = rank + offset
		}
	}
	return ret
}

// null is returned as -999
func rank(vals []float64, isNull []bool, index []int) []float64 {

	// copy all existing values at index positions
	newVals := make([]float64, len(index))
	newIsNull := make([]bool, len(index))
	for i := range index {
		newVals[i] = vals[i]
		newIsNull[i] = isNull[i]
	}
	// sort floats
	floats := &floatValueContainer{slice: newVals, index: makeIntRange(0, len(index)), isNull: newIsNull}
	return floats.rank()
}

func (vc *floatValueContainer) percentile() []float64 {
	ret := make([]float64, len(vc.slice))
	var validCount int
	for i := range vc.isNull {
		if !vc.isNull[i] {
			validCount++
		}
	}
	rank := vc.rank()

	var counter int
	for i := range rank {
		originalPosition := vc.index[i]
		percentile := float64(counter) / float64(validCount)
		// handle null
		if vc.isNull[i] {
			ret[originalPosition] = -999
			continue
		}
		// no duplicates in first row
		if i == 0 {
			ret[originalPosition] = percentile
			counter++
			continue
		}
		// handle repeat ranks (ie duplicate rows)
		if vc.slice[i] == vc.slice[i-1] {
			priorOriginal := vc.index[i-1]
			ret[originalPosition] = ret[priorOriginal]
		} else {
			ret[originalPosition] = percentile
		}
		counter++
	}
	return ret
}

// exclusive definition: what % of all values are below this value
// -999 for null values
func percentile(vals []float64, isNull []bool, index []int) []float64 {
	// copy all existing values at index positions
	newVals := make([]float64, len(index))
	newIsNull := make([]bool, len(index))
	for i := range index {
		newVals[i] = vals[i]
		newIsNull[i] = isNull[i]
	}
	floats := &floatValueContainer{slice: newVals, index: makeIntRange(0, len(index)), isNull: newIsNull}

	return floats.percentile()
}

// percentile cut
func (vc *valueContainer) pcut(bins []float64, labels []string) ([]string, error) {
	for i, edge := range bins {
		if edge < 0 || edge > 1 {
			return nil, fmt.Errorf("all bin edges must be between 0 and 1 (%v at edge %d", edge, i)
		}
	}
	pctile := percentile(vc.float().slice, vc.isNull, makeIntRange(0, len(vc.isNull)))
	leftInclusive := true
	rightExclusive := true
	return cut(pctile, vc.isNull, bins, leftInclusive, rightExclusive, false, false, labels)
}
