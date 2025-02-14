package jwtauth

import (
	"encoding/json"
	"testing"

	"github.com/go-test/deep"
	"github.com/hashicorp/go-hclog"
)

func TestGetClaim(t *testing.T) {
	data := `{
		"a": 42,
		"b": "bar",
		"c": {
			"d": 95,
			"e": [
				"dog",
				"cat",
				"bird"
			],
			"f": {
				"g": "zebra"
			}
		}
	}`
	var claims map[string]interface{}
	if err := json.Unmarshal([]byte(data), &claims); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		claim string
		value interface{}
	}{
		{"a", float64(42)},
		{"/a", float64(42)},
		{"b", "bar"},
		{"/c/d", float64(95)},
		{"/c/e/1", "cat"},
		{"/c/f/g", "zebra"},
		{"nope", nil},
		{"/c/f/h", nil},
		{"", nil},
		{"\\", nil},
	}

	for _, test := range tests {
		v := getClaim(hclog.NewNullLogger(), claims, test.claim)

		if diff := deep.Equal(v, test.value); diff != nil {
			t.Fatal(diff)
		}
	}
}

func TestExtractMetadata(t *testing.T) {
	emptyMap := make(map[string]string)

	tests := []struct {
		testCase      string
		allClaims     map[string]interface{}
		claimMappings map[string]string
		expected      map[string]string
		errExpected   bool
	}{
		{"empty", nil, nil, emptyMap, false},
		{
			"full match",
			map[string]interface{}{
				"data1": "foo",
				"data2": "bar",
			},
			map[string]string{
				"data1": "val1",
				"data2": "val2",
			},
			map[string]string{
				"val1": "foo",
				"val2": "bar",
			},
			false,
		},
		{
			"partial match",
			map[string]interface{}{
				"data1": "foo",
				"data2": "bar",
			},
			map[string]string{
				"data1": "val1",
				"data3": "val2",
			},
			map[string]string{
				"val1": "foo",
			},
			false,
		},
		{
			"no match",
			map[string]interface{}{
				"data1": "foo",
				"data2": "bar",
			},
			map[string]string{
				"data8": "val1",
				"data9": "val2",
			},
			emptyMap,
			false,
		},
		{
			"nested data",
			map[string]interface{}{
				"data1": "foo",
				"data2": map[string]interface{}{
					"child": "bar",
				},
			},
			map[string]string{
				"data1":        "val1",
				"/data2/child": "val2",
			},
			map[string]string{
				"val1": "foo",
				"val2": "bar",
			},
			false,
		},
		{
			"error: non-string data",
			map[string]interface{}{
				"data1": 42,
			},
			map[string]string{
				"data1": "val1",
			},
			nil,
			true,
		},
	}

	for _, test := range tests {
		actual, err := extractMetadata(hclog.NewNullLogger(), test.allClaims, test.claimMappings)
		if (err != nil) != test.errExpected {
			t.Fatalf("case '%s': expected error: %t, actual: %v", test.testCase, test.errExpected, err)
		}
		if diff := deep.Equal(actual, test.expected); diff != nil {
			t.Fatalf("case '%s': expected results: %v", test.testCase, diff)
		}
	}
}

func TestValidateAudience(t *testing.T) {
	tests := []struct {
		boundAudiences []string
		audience       []string
		strict         bool
		errExpected    bool
	}{
		{[]string{"a"}, []string{"a"}, false, false},
		{[]string{"a"}, []string{"b"}, false, true},
		{[]string{"a"}, []string{""}, false, true},
		{[]string{}, []string{"a"}, false, false},
		{[]string{}, []string{"a"}, true, true},
		{[]string{"a", "b"}, []string{"a"}, false, false},
		{[]string{"a", "b"}, []string{"b"}, false, false},
		{[]string{"a", "b"}, []string{"a", "b", "c"}, false, false},
		{[]string{"a", "b"}, []string{"c", "d"}, false, true},
	}

	for _, test := range tests {
		err := validateAudience(test.boundAudiences, test.audience, test.strict)
		if test.errExpected != (err != nil) {
			t.Fatalf("unexpected error result: boundAudiences %v, audience %v, strict %t, err: %v",
				test.boundAudiences, test.audience, test.strict, err)
		}
	}
}

func TestValidateBoundClaims(t *testing.T) {
	tests := []struct {
		name        string
		boundClaims map[string]interface{}
		allClaims   map[string]interface{}
		errExpected bool
	}{
		{
			name: "valid",
			boundClaims: map[string]interface{}{
				"foo": "a",
				"bar": "b",
			},
			allClaims: map[string]interface{}{
				"foo": "a",
				"bar": "b",
			},
			errExpected: false,
		},
		{
			name: "valid - extra data",
			boundClaims: map[string]interface{}{
				"foo": "a",
				"bar": "b",
			},
			allClaims: map[string]interface{}{
				"foo":   "a",
				"bar":   "b",
				"color": "green",
			},
			errExpected: false,
		},
		{
			name: "mismatched value",
			boundClaims: map[string]interface{}{
				"foo": "a",
				"bar": "b",
			},
			allClaims: map[string]interface{}{
				"foo": "a",
				"bar": "wrong",
			},
			errExpected: true,
		},
		{
			name: "missing claim",
			boundClaims: map[string]interface{}{
				"foo": "a",
				"bar": "b",
			},
			allClaims: map[string]interface{}{
				"foo": "a",
			},
			errExpected: true,
		},
		{
			name: "valid - JSONPointer",
			boundClaims: map[string]interface{}{
				"foo":        "a",
				"/bar/baz/1": "y",
			},
			allClaims: map[string]interface{}{
				"foo": "a",
				"bar": map[string]interface{}{
					"baz": []string{"x", "y", "z"},
				},
			},
			errExpected: false,
		},
		{
			name: "invalid - JSONPointer value mismatch",
			boundClaims: map[string]interface{}{
				"foo":        "a",
				"/bar/baz/1": "q",
			},
			allClaims: map[string]interface{}{
				"foo": "a",
				"bar": map[string]interface{}{
					"baz": []string{"x", "y", "z"},
				},
			},
			errExpected: true,
		},
		{
			name: "invalid - JSONPointer not found",
			boundClaims: map[string]interface{}{
				"foo":           "a",
				"/bar/XXX/1243": "q",
			},
			allClaims: map[string]interface{}{
				"foo": "a",
				"bar": map[string]interface{}{
					"baz": []string{"x", "y", "z"},
				},
			},
			errExpected: true,
		},
	}
	for _, tt := range tests {
		if err := validateBoundClaims(hclog.NewNullLogger(), tt.boundClaims, tt.allClaims); (err != nil) != tt.errExpected {
			t.Errorf("validateBoundClaims(%s) error = %v, wantErr %v", tt.name, err, tt.errExpected)
		}
	}
}
