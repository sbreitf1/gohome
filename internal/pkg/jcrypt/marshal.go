package jcrypt

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type srcValue struct {
	Type        reflect.Type
	Value       reflect.Value
	StructField *reflect.StructField
}

func (src srcValue) Elem() srcValue {
	return srcValue{src.Type.Elem(), src.Value.Elem(), src.StructField}
}

func (src srcValue) Kind() reflect.Kind {
	return src.Type.Kind()
}

func (src srcValue) IsNil() bool {
	return src.Value.IsNil()
}

func (src srcValue) Len() int {
	return src.Value.Len()
}

func (src srcValue) Index(i int) srcValue {
	v := src.Value.Index(i)
	return srcValue{v.Type(), v, nil}
}

func (src srcValue) Interface() interface{} {
	return src.Value.Interface()
}

type marshalHandler func(src srcValue) (result interface{}, handled bool, err error)

func jsonMarshal(src interface{}, f marshalHandler) ([]byte, error) {
	t := reflect.TypeOf(src)
	v := reflect.ValueOf(src)
	dst, err := jsonMarshalValue(srcValue{t, v, nil}, f)
	if err != nil {
		return nil, err
	}

	return json.Marshal(dst)
}

func jsonMarshalValue(src srcValue, f marshalHandler) (interface{}, error) {
	switch src.Type.Kind() {
	case reflect.Ptr:
		if src.IsNil() {
			return nil, nil
		}
		return jsonMarshalValue(src.Elem(), f)

	case reflect.Map:
		return nil, fmt.Errorf("maps not yet supported")
	case reflect.Struct:
		return jsonMarshalStruct(src, f)

	case reflect.Array:
		fallthrough
	case reflect.Slice:
		return jsonMarshalArray(src, f)

	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		fallthrough
	case reflect.Int64:
		fallthrough
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		fallthrough
	case reflect.Bool:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		fallthrough
	case reflect.Uint64:
		fallthrough
	case reflect.String:
		return src.Value.Interface(), nil

	default:
		return nil, fmt.Errorf("data type %s not yet supported", src.Type.Kind())
	}
}

func jsonMarshalStruct(src srcValue, f marshalHandler) (interface{}, error) {
	result := make(map[string]interface{})

	fieldCount := src.Type.NumField()
	for i := 0; i < fieldCount; i++ {
		field := src.Type.Field(i)
		jsonTag := strings.Split(field.Tag.Get("json"), ",")
		fieldName := field.Name
		if len(jsonTag[0]) > 0 {
			if jsonTag[0] == "-" {
				continue
			} else {
				fieldName = jsonTag[0]
			}
		}

		fieldValue := srcValue{field.Type, src.Value.Field(i), &field}

		//TODO omitempty and string options in jsonTag[1]

		value, err := func() (interface{}, error) {
			if f != nil {
				val, handled, err := f(fieldValue)
				if err != nil {
					return nil, err
				}

				if handled {
					return val, nil
				}
			}
			return jsonMarshalValue(fieldValue, f)
		}()
		if err != nil {
			return nil, err
		}

		result[fieldName] = value
	}

	return result, nil
}

func jsonMarshalArray(src srcValue, f marshalHandler) (interface{}, error) {
	len := src.Len()
	result := make([]interface{}, len)
	for i := 0; i < len; i++ {
		data, err := jsonMarshalValue(src.Index(i), f)
		if err != nil {
			return nil, err
		}
		result[i] = data
	}
	return result, nil
}
