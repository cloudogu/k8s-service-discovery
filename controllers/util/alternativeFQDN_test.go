package util

import (
	"reflect"
	"testing"
)

func TestParseAlternativeFQDNsFromConfigString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output []AlternativeFQDN
	}{
		{
			name:  "Single FQDN with empty certificate",
			input: "example.com",
			output: []AlternativeFQDN{
				{"example.com", ""},
			},
		},
		{
			name:  "Single FQDN with certificate",
			input: "example.com:certificate123",
			output: []AlternativeFQDN{
				{"example.com", "certificate123"},
			},
		},
		{
			name:  "Multiple FQDNs with certificates",
			input: "example.com:cert1,example.net:cert2,example.org:cert3",
			output: []AlternativeFQDN{
				{"example.com", "cert1"},
				{"example.net", "cert2"},
				{"example.org", "cert3"},
			},
		},
		{
			name:  "Multiple FQDNs with mixed certs",
			input: "example.com:cert1,example.net,example.org:cert3",
			output: []AlternativeFQDN{
				{"example.com", "cert1"},
				{"example.net", ""},
				{"example.org", "cert3"},
			},
		},
		{
			name:  "Multiple FQDNs with mixed certs with spaces",
			input: "example.com:cert1 , example.net, example.org : cert3",
			output: []AlternativeFQDN{
				{"example.com", "cert1"},
				{"example.net", ""},
				{"example.org", "cert3"},
			},
		},
		{
			name:   "Empty input string",
			input:  "",
			output: []AlternativeFQDN{},
		},
		{
			name:  "Input with extra commas",
			input: "example.com:cert1,,example.net:cert2,,",
			output: []AlternativeFQDN{
				{"example.com", "cert1"},
				{"example.net", "cert2"},
			},
		},
		{
			name:   "Input with empty FQDNs",
			input:  ":cert1,:cert2,:cert3",
			output: []AlternativeFQDN{},
		},
		{
			name:   "Only separators without values",
			input:  ",,:",
			output: []AlternativeFQDN{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ParseAlternativeFQDNsFromConfigString(test.input)
			if !reflect.DeepEqual(result, test.output) {
				t.Errorf("For input '%s', expected %#v, but got %#v", test.input, test.output, result)
			}
		})
	}
}
