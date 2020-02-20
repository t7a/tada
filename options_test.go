package tada

import (
	"testing"
)

func TestSetOptionLevelSeparator(t *testing.T) {
	type args struct {
		sep string
	}
	tests := []struct {
		name string
		args args
	}{
		{"pass", args{"||"}},
	}
	for _, tt := range tests {
		archive := optionLevelSeparator
		t.Run(tt.name, func(t *testing.T) {
			SetOptionLevelSeparator(tt.args.sep)
		})

		if got := optionLevelSeparator; got != tt.args.sep {
			t.Errorf("SetOptionLevelSeparator() -> %v, want %v", got, tt.args.sep)
		}
		optionLevelSeparator = archive
	}
}

func TestSetOptionMaxRows(t *testing.T) {
	type args struct {
		n int
	}
	tests := []struct {
		name string
		args args
	}{
		{"pass", args{5}},
	}
	for _, tt := range tests {
		archive := optionMaxRows
		t.Run(tt.name, func(t *testing.T) {
			SetOptionMaxRows(tt.args.n)
		})

		if got := optionMaxRows; got != tt.args.n {
			t.Errorf("SetOptionMaxRows() -> %v, want %v", got, tt.args.n)
		}
		optionMaxRows = archive
	}
}

func TestSetOptionAutoMerge(t *testing.T) {
	type args struct {
		set bool
	}
	tests := []struct {
		name string
		args args
	}{
		{"pass", args{false}},
	}
	for _, tt := range tests {
		archive := optionAutoMerge
		t.Run(tt.name, func(t *testing.T) {
			SetOptionAutoMerge(tt.args.set)
		})

		if got := optionAutoMerge; got != tt.args.set {
			t.Errorf("SetOptionAutoMerge() -> %v, want %v", got, tt.args.set)
		}
		optionAutoMerge = archive
	}
}