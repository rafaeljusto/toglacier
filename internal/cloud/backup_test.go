package cloud_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/rafaeljusto/toglacier/internal/cloud"
)

func TestParseLocation(t *testing.T) {
	scenarios := []struct {
		description   string
		value         string
		expected      cloud.Location
		expectedError error
	}{
		{
			description: "it should convert an aws location correctly",
			value:       "  AWS  ",
			expected:    cloud.LocationAWS,
		},
		{
			description: "it should convert a gcs location correctly",
			value:       "  GCS  ",
			expected:    cloud.LocationGCS,
		},
		{
			description:   "it should detect an unknown location",
			value:         "unknown-location",
			expectedError: errors.New("unknown location “unknown-location”"),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			location, err := cloud.ParseLocation(scenario.value)
			if scenario.expected != location {
				t.Errorf("invalid location. expected location “%s” and got “%s”", scenario.expected, location)
			}
			if !reflect.DeepEqual(scenario.expectedError, err) {
				t.Errorf("errors don't match. expected “%v” and got “%v”", scenario.expectedError, err)
			}
		})
	}
}

func TestLocation_Defined(t *testing.T) {
	scenarios := []struct {
		description string
		location    cloud.Location
		expected    bool
	}{
		{
			description: "it should detect a defined location",
			location:    cloud.LocationAWS,
			expected:    true,
		},
		{
			description: "it should detect an undefined location",
			location:    cloud.Location("unknown"),
			expected:    false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.description, func(t *testing.T) {
			defined := scenario.location.Defined()
			if scenario.expected != defined {
				t.Errorf("expected %t with location “%s”", scenario.expected, scenario.location)
			}
		})
	}
}
