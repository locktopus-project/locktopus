package lockqueue

import (
	"reflect"
	"testing"
)

func Test_truncateAfterPrefixes(t *testing.T) {
	type args struct {
		input []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		// TODO: Add test cases.
		{
			name: "empty input",
			args: args{
				input: []string{},
			},
			want: []string{},
		},
		{
			name: "single input",
			args: args{
				input: []string{"a"},
			},
			want: []string{"a"},
		},
		{
			name: "multiple input without prefixes",
			args: args{
				input: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			},
			want: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
		},
		{
			name: "multiple inputs reordered, with duplicates",
			args: args{
				input: []string{"h", "j", "b", "c", "a", "d", "e", "f", "h", "i", "g"},
			},
			want: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
		},
		{
			name: "multiple input with prefixes - 1",
			args: args{
				input: []string{"a", "ab", "abc"},
			},
			want: []string{"a"},
		},
		{
			name: "multiple input with prefixes - 2",
			args: args{
				input: []string{"a", "b", "c", "cd", "cde", "d", "e", "f"},
			},
			want: []string{"a", "b", "c", "d", "e", "f"},
		},
		{
			name: "multiple input with multiple prefix groups",
			args: args{
				input: []string{"a", "b", "c", "cd", "cde", "d", "dz", "dy", "e", "f", "fz", "fzy", "x"},
			},
			want: []string{"a", "b", "c", "d", "e", "f", "x"},
		},
		{
			name: "multiple input with prefixes, reordered, with duplicates",
			args: args{
				input: []string{"f", "e", "d", "cde", "cd", "c", "b", "cde", "a", "e"},
			},
			want: []string{"a", "b", "c", "d", "e", "f"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateAfterPrefixes(tt.args.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("removeSelfLockers() = %v, want %v", got, tt.want)
			}
		})
	}
}
