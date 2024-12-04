package jcrypt

import (
	"io/ioutil"
	"os"
)

// UnmarshalFromFile reads a file and unmarshals it.
func UnmarshalFromFile(file string, v interface{}, options *Options) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	return Unmarshal(data, v, options)
}

// MarshalToFile marshals an object and writes the data to a file using os.ModePerm.
func MarshalToFile(file string, v interface{}, options *Options) error {
	data, err := Marshal(v, options)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, data, os.ModePerm)
}
