package jcrypt

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshalOrdinalTypes(t *testing.T) {
	testUnmarshal(t, "foo bar")
	testUnmarshal(t, 1337)
	testUnmarshal(t, 4.2)
	testUnmarshal(t, true)
	testUnmarshal(t, false)
}

func testUnmarshal(t *testing.T, expected interface{}) bool {
	return t.Run(fmt.Sprintf("TestUnmarshalOrdinalType %T", expected), func(t *testing.T) {
		data, _ := json.Marshal(&expected)

		dstPtr := reflect.New(reflect.TypeOf(expected)).Interface()
		err := jsonUnmarshal(data, dstPtr, nil)
		assert.NoError(t, err)
		assert.Equal(t, expected, reflect.ValueOf(dstPtr).Elem().Interface())
	})
}

func TestUnmarshalStruct(t *testing.T) {
	type Type struct {
		StrType   string  `json:"str"`
		IntType   int     `json:"integer"`
		FloatType float32 `json:"floating"`
		BoolType  bool
		Ignored   string `json:"-"`
	}

	expected := Type{"foo bar", 1337, 4.2, true, "should be empty"}
	data, err := json.Marshal(&expected)
	assert.NoError(t, err)

	var reconstructed Type
	assert.NoError(t, jsonUnmarshal(data, &reconstructed, nil))

	// is ignored -> expected to be empty
	expected.Ignored = ""
	assert.Equal(t, expected, reconstructed)
}

func TestUnmarshalArray(t *testing.T) {
	expected := [2]string{"foo", "bar"}
	data, err := json.Marshal(&expected)
	assert.NoError(t, err)

	reconstructed := [2]string{}
	assert.NoError(t, jsonUnmarshal(data, &reconstructed, nil))
	assert.Equal(t, expected, reconstructed)
}

func TestUnmarshalSlice(t *testing.T) {
	expected := []string{"foo", "bar"}
	data, err := json.Marshal(&expected)
	assert.NoError(t, err)

	reconstructed := []string{}
	assert.NoError(t, jsonUnmarshal(data, &reconstructed, nil))
	assert.Equal(t, expected, reconstructed)
}

func TestUnmarshalPointer(t *testing.T) {
	type Type struct {
		StrPtr *string `json:"str"`
	}
	strFoobar := "foobar"

	expected := Type{&strFoobar}
	data, err := json.Marshal(&expected)
	assert.NoError(t, err)

	reconstructed := Type{}
	assert.NoError(t, jsonUnmarshal(data, &reconstructed, nil))

	assert.Equal(t, expected, reconstructed)
}

func TestUnmarshalNilPointer(t *testing.T) {
	type Type struct {
		StrPtr   *string `json:"str"`
		OtherStr *string `json:"str2"`
	}
	strFoobar := "foobar"
	strFoo := "foo"
	strBar := "bar"

	expected := Type{nil, &strFoobar}
	data, err := json.Marshal(&expected)
	assert.NoError(t, err)

	reconstructed := Type{&strFoo, &strBar}
	assert.NoError(t, jsonUnmarshal(data, &reconstructed, nil))

	assert.Equal(t, expected, reconstructed)
}
