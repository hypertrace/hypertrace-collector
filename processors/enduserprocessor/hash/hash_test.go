package hash

import (
	"testing"

	"gotest.tools/assert"
)

func TestResolveHashAlgorithm(t *testing.T) {
	const value = "abc123"

	tCases := map[string]struct {
		name               string
		expectedResolution bool
		expectedValue      string
	}{
		"SHA-1 algorithm": {
			name:               "SHA-1",
			expectedResolution: true,
			expectedValue:      "6367c48dd193d56ea7b0baad25b19455e529f5ee",
		},
		"SHAKE256 algorithm": {
			name:               "SHAKE256",
			expectedResolution: true,
			expectedValue:      "2ba0b47e3371abfccb29873c9a45f938316afc02c644ef9e98478893f1f5e3a739ff006fa85d8418949ee2d5ef43b64df7cc9fe8b7ca71ff1ec6c1ed1f6cf37e",
		},
		"Empty algorithm name": {
			name:               "",
			expectedResolution: false,
			expectedValue:      "6367c48dd193d56ea7b0baad25b19455e529f5ee",
		},
		"Unknown algorithm name": {
			name:               "SOMETHING",
			expectedResolution: false,
			expectedValue:      "6367c48dd193d56ea7b0baad25b19455e529f5ee",
		}}

	for name, tCase := range tCases {
		t.Run(name, func(t *testing.T) {
			algo, ok := ResolveHashAlgorithm(tCase.name)
			assert.Equal(t, tCase.expectedResolution, ok)
			assert.Equal(t, algo(value), tCase.expectedValue)
		})
	}
}
