package jcrypt

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarshalOrdinalTypes(t *testing.T) {
	testMarshal(t, "foo bar")
	testMarshal(t, 1337)
	testMarshal(t, 4.2)
	testMarshal(t, true)
	testMarshal(t, false)
}

func testMarshal(t *testing.T, obj interface{}) bool {
	return t.Run(fmt.Sprintf("TestMarshalOrdinalType %T", obj), func(t *testing.T) {
		expectedData, _ := json.Marshal(obj)

		data, err := jsonMarshal(obj, nil)
		assert.NoError(t, err)
		assert.Equal(t, expectedData, data)
	})
}

func TestMarshalStruct(t *testing.T) {
	type Type struct {
		StrType   string  `json:"str"`
		IntType   int     `json:"integer"`
		FloatType float32 `json:"floating"`
		BoolType  bool
		Ignored   string `json:"-"`
	}

	expected := Type{"foo bar", 1337, 4.2, true, "should be empty"}
	data, err := jsonMarshal(&expected, nil)
	assert.NoError(t, err)

	var reconstructed Type
	assert.NoError(t, json.Unmarshal(data, &reconstructed))

	// is ignored -> expected to be empty
	expected.Ignored = ""
	assert.Equal(t, expected, reconstructed)
}

func TestMarshalArray(t *testing.T) {
	expected := [2]string{"foo", "bar"}
	data, err := jsonMarshal(&expected, nil)
	assert.NoError(t, err)

	reconstructed := [2]string{}
	assert.NoError(t, json.Unmarshal(data, &reconstructed))

	assert.Equal(t, expected, reconstructed)
}

func TestMarshalSlice(t *testing.T) {
	expected := []string{"foo", "bar"}
	data, err := jsonMarshal(&expected, nil)
	assert.NoError(t, err)

	reconstructed := []string{}
	assert.NoError(t, json.Unmarshal(data, &reconstructed))

	assert.Equal(t, expected, reconstructed)
}

func TestMarshalPointer(t *testing.T) {
	type Type struct {
		StrPtr   *string `json:"str"`
		OtherStr *string `json:"str2"`
	}
	strFoobar := "foobar"
	strFoo := "foo"
	strBar := "bar"

	expected := Type{nil, &strFoobar}
	data, err := jsonMarshal(&expected, nil)
	assert.NoError(t, err)

	reconstructed := Type{&strFoo, &strBar}
	assert.NoError(t, json.Unmarshal(data, &reconstructed))

	assert.Equal(t, expected, reconstructed)
}
