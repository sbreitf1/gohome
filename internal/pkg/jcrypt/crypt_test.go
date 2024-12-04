package jcrypt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testOptions = &Options{GetKeyHandler: StaticKey([]byte("test-secret"))}
)

func TestMarshalString(t *testing.T) {
	type Type struct {
		Value string `json:"data" jcrypt:"aes"`
	}

	expected := Type{"foo bar"}
	data, err := Marshal(&expected, testOptions)
	assert.NoError(t, err)

	var reconstructed Type
	assert.NoError(t, Unmarshal(data, &reconstructed, testOptions))

	assert.Equal(t, expected, reconstructed)
}

func TestMarshalInt(t *testing.T) {
	type Type struct {
		Value int `json:"data" jcrypt:"aes"`
	}

	expected := Type{1337}
	data, err := Marshal(&expected, testOptions)
	assert.NoError(t, err)

	var reconstructed Type
	assert.NoError(t, Unmarshal(data, &reconstructed, testOptions))

	assert.Equal(t, expected, reconstructed)
}

func TestUnmarshalRawString(t *testing.T) {
	type Type struct {
		Value string `json:"data" jcrypt:"aes"`
	}

	input := `{"data":"secret"}`
	expected := Type{"secret"}
	var d Type
	assert.NoError(t, Unmarshal([]byte(input), &d, nil))
	assert.Equal(t, expected, d)
}
