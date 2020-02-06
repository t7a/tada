package tada

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/d4l3k/messagediff"
)

func TestNewDataFrame(t *testing.T) {
	type args struct {
		slices []interface{}
		labels []interface{}
	}
	tests := []struct {
		name string
		args args
		want *DataFrame
	}{
		{"normal", args{
			[]interface{}{[]float64{1, 2}, []string{"foo", "bar"}},
			[]interface{}{[]int{0, 1}}},
			&DataFrame{
				values: []*valueContainer{
					{slice: []float64{1, 2}, isNull: []bool{false, false}, name: "0"},
					{slice: []string{"foo", "bar"}, isNull: []bool{false, false}, name: "1"}},
				labels: []*valueContainer{{slice: []int{0, 1}, isNull: []bool{false, false}, name: "*0"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewDataFrame(tt.args.slices, tt.args.labels...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewDataFrame() = %v, want %v", got, tt.want)
			}
		})
	}
}

// for DF to series conversion
// {"unsupported dataframe with multiple columns", args{
// 	slice: DataFrame{
// 		values: []*valueContainer{{slice: []float64{1}, isNull: []bool{false}}, {slice: []float64{2}, isNull: []bool{false}}},
// 		labels: []*valueContainer{{slice: []int{0}, isNull: []bool{false}}}}},
// 	&Series{err: errors.New("unsupported input type (DataFrame with multiple columns); must be slice or DataFrame with single column")}},
// {"dataframe with single column", args{
// 	slice: DataFrame{
// 		values: []*valueContainer{{slice: []float64{1}, isNull: []bool{false}}},
// 		labels: []*valueContainer{{slice: []int{0}, isNull: []bool{false}}}},
// },
// 	&Series{values: &valueContainer{slice: []float64{1}, isNull: []bool{false}},
// 		labels: []*valueContainer{{slice: []int{0}, isNull: []bool{false}}}}},

func TestDataFrame_Copy(t *testing.T) {
	type fields struct {
		labels []*valueContainer
		values []*valueContainer
		name   string
		err    error
	}
	tests := []struct {
		name   string
		fields fields
		want   *DataFrame
	}{
		{"normal", fields{
			values: []*valueContainer{
				{slice: []float64{1, 2}, isNull: []bool{false, false}, name: "0"},
				{slice: []string{"foo", "bar"}, isNull: []bool{false, false}, name: "1"}},
			labels: []*valueContainer{{slice: []int{0, 1}, isNull: []bool{false, false}, name: "*0"}},
			name:   "baz"},
			&DataFrame{values: []*valueContainer{
				{slice: []float64{1, 2}, isNull: []bool{false, false}, name: "0"},
				{slice: []string{"foo", "bar"}, isNull: []bool{false, false}, name: "1"}},
				labels: []*valueContainer{{slice: []int{0, 1}, isNull: []bool{false, false}, name: "*0"}},
				name:   "baz"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df := &DataFrame{
				labels: tt.fields.labels,
				values: tt.fields.values,
				name:   tt.fields.name,
				err:    tt.fields.err,
			}
			got := df.Copy()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataFrame.Copy() = %v, want %v", got, tt.want)
			}
			got.values[0].isNull[0] = true
			if reflect.DeepEqual(got, df) {
				t.Errorf("DataFrame.Copy() = retained reference to original values")
			}
			got.err = errors.New("foo")
			if reflect.DeepEqual(got, df) {
				t.Errorf("DataFrame.Copy() retained reference to original error")
			}
		})
	}
}

func TestDataFrame_Subset(t *testing.T) {
	type fields struct {
		labels []*valueContainer
		values []*valueContainer
		name   string
		err    error
	}
	type args struct {
		index []int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *DataFrame
	}{
		{"normal", fields{
			values: []*valueContainer{
				{slice: []float64{1, 2}, isNull: []bool{false, false}, name: "0"},
				{slice: []string{"foo", "bar"}, isNull: []bool{false, false}, name: "1"}},
			labels: []*valueContainer{{slice: []int{0, 1}, isNull: []bool{false, false}, name: "*0"}},
			name:   "baz"},
			args{[]int{0}},
			&DataFrame{values: []*valueContainer{
				{slice: []float64{1}, isNull: []bool{false}, name: "0"},
				{slice: []string{"foo"}, isNull: []bool{false}, name: "1"}},
				labels: []*valueContainer{{slice: []int{0}, isNull: []bool{false}, name: "*0"}},
				name:   "baz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df := &DataFrame{
				labels: tt.fields.labels,
				values: tt.fields.values,
				name:   tt.fields.name,
				err:    tt.fields.err,
			}
			if got := df.Subset(tt.args.index); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataFrame.Subset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataFrame_SubsetLabels(t *testing.T) {
	type fields struct {
		labels []*valueContainer
		values []*valueContainer
		name   string
		err    error
	}
	type args struct {
		index []int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *DataFrame
	}{
		{"normal", fields{
			values: []*valueContainer{
				{slice: []float64{1}, isNull: []bool{false}, name: "0"},
				{slice: []string{"foo"}, isNull: []bool{false}, name: "1"}},
			labels: []*valueContainer{
				{slice: []int{0}, isNull: []bool{false}, name: "*0"},
				{slice: []int{10}, isNull: []bool{false}, name: "*10"},
			},
			name: "baz"},
			args{[]int{1}},
			&DataFrame{values: []*valueContainer{
				{slice: []float64{1}, isNull: []bool{false}, name: "0"},
				{slice: []string{"foo"}, isNull: []bool{false}, name: "1"}},
				labels: []*valueContainer{{slice: []int{10}, isNull: []bool{false}, name: "*10"}},
				name:   "baz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df := &DataFrame{
				labels: tt.fields.labels,
				values: tt.fields.values,
				name:   tt.fields.name,
				err:    tt.fields.err,
			}
			if got := df.SubsetLabels(tt.args.index); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataFrame.SubsetLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataFrame_SubsetCols(t *testing.T) {
	type fields struct {
		labels []*valueContainer
		values []*valueContainer
		name   string
		err    error
	}
	type args struct {
		index []int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *DataFrame
	}{
		{"normal", fields{
			values: []*valueContainer{
				{slice: []float64{1}, isNull: []bool{false}, name: "0"},
				{slice: []string{"foo"}, isNull: []bool{false}, name: "1"}},
			labels: []*valueContainer{
				{slice: []int{0}, isNull: []bool{false}, name: "*0"},
				{slice: []int{10}, isNull: []bool{false}, name: "*10"},
			},
			name: "baz"},
			args{[]int{1}},
			&DataFrame{values: []*valueContainer{
				{slice: []string{"foo"}, isNull: []bool{false}, name: "1"}},
				labels: []*valueContainer{
					{slice: []int{0}, isNull: []bool{false}, name: "*0"},
					{slice: []int{10}, isNull: []bool{false}, name: "*10"}},
				name: "baz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df := &DataFrame{
				labels: tt.fields.labels,
				values: tt.fields.values,
				name:   tt.fields.name,
				err:    tt.fields.err,
			}
			if got := df.SubsetCols(tt.args.index); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataFrame.SubsetCols() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataFrame_Head(t *testing.T) {
	type fields struct {
		labels []*valueContainer
		values []*valueContainer
		name   string
		err    error
	}
	type args struct {
		n int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *DataFrame
	}{
		{"normal", fields{
			values: []*valueContainer{
				{slice: []string{"foo", "bar", "baz"}, isNull: []bool{false, false, false}, name: "0"}},
			labels: []*valueContainer{
				{slice: []int{0, 1, 2}, isNull: []bool{false, false, false}, name: "*0"},
			},
			name: "baz"},
			args{2},
			&DataFrame{values: []*valueContainer{
				{slice: []string{"foo", "bar"}, isNull: []bool{false, false}, name: "0"}},
				labels: []*valueContainer{{slice: []int{0, 1}, isNull: []bool{false, false}, name: "*0"}},
				name:   "baz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df := &DataFrame{
				labels: tt.fields.labels,
				values: tt.fields.values,
				name:   tt.fields.name,
				err:    tt.fields.err,
			}
			if got := df.Head(tt.args.n); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataFrame.Head() = %v, want %v", got, tt.want)
				t.Errorf(messagediff.PrettyDiff(got, tt.want))
			}
		})
	}
}
func TestDataFrame_Tail(t *testing.T) {
	type fields struct {
		labels []*valueContainer
		values []*valueContainer
		name   string
		err    error
	}
	type args struct {
		n int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *DataFrame
	}{
		{"normal", fields{
			values: []*valueContainer{
				{slice: []string{"foo", "bar", "baz"}, isNull: []bool{false, false, false}, name: "0"}},
			labels: []*valueContainer{
				{slice: []int{0, 1, 2}, isNull: []bool{false, false, false}, name: "*0"},
			},
			name: "baz"},
			args{2},
			&DataFrame{values: []*valueContainer{
				{slice: []string{"bar", "baz"}, isNull: []bool{false, false}, name: "0"}},
				labels: []*valueContainer{{slice: []int{1, 2}, isNull: []bool{false, false}, name: "*0"}},
				name:   "baz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df := &DataFrame{
				labels: tt.fields.labels,
				values: tt.fields.values,
				name:   tt.fields.name,
				err:    tt.fields.err,
			}
			if got := df.Tail(tt.args.n); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataFrame.Tail() = %v, want %v", got.labels[0], tt.want.labels[0])
			}
		})
	}
}
func TestDataFrame_Range(t *testing.T) {
	type fields struct {
		labels []*valueContainer
		values []*valueContainer
		name   string
		err    error
	}
	type args struct {
		first int
		last  int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *DataFrame
	}{
		{"normal", fields{
			values: []*valueContainer{
				{slice: []string{"foo", "bar", "baz"}, isNull: []bool{false, false, false}, name: "0"}},
			labels: []*valueContainer{
				{slice: []int{0, 1, 2}, isNull: []bool{false, false, false}, name: "*0"},
			},
			name: "baz"},
			args{1, 2},
			&DataFrame{values: []*valueContainer{
				{slice: []string{"bar", "baz"}, isNull: []bool{false, false}, name: "0"}},
				labels: []*valueContainer{{slice: []int{1, 2}, isNull: []bool{false, false}, name: "*0"}},
				name:   "baz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df := &DataFrame{
				labels: tt.fields.labels,
				values: tt.fields.values,
				name:   tt.fields.name,
				err:    tt.fields.err,
			}
			if got := df.Range(tt.args.first, tt.args.last); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataFrame.Range() = %v, want %v", got.labels[0], tt.want.labels[0])
			}
		})
	}
}

func TestDataFrame_FilterCols(t *testing.T) {
	type fields struct {
		labels []*valueContainer
		values []*valueContainer
		name   string
		err    error
	}
	type args struct {
		lambda func(string) bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []int
	}{
		{"single level", fields{
			values: []*valueContainer{
				{slice: []float64{1}, isNull: []bool{false}, name: "foo"},
				{slice: []float64{1}, isNull: []bool{false}, name: "bar"},
				{slice: []float64{1}, isNull: []bool{false}, name: "baz"}},
			labels: []*valueContainer{
				{slice: []int{0}, isNull: []bool{false}, name: "*0"},
				{slice: []int{10}, isNull: []bool{false}, name: "*10"}}},
			args{func(s string) bool {
				if strings.Contains(s, "ba") {
					return true
				}
				return false
			}},
			[]int{1, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df := &DataFrame{
				labels: tt.fields.labels,
				values: tt.fields.values,
				name:   tt.fields.name,
				err:    tt.fields.err,
			}
			if got := df.FilterCols(tt.args.lambda); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DataFrame.FilterCols() = %v, want %v", got, tt.want)
			}
		})
	}
}
