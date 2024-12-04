package jcrypt

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type dstValue struct {
	Type        reflect.Type
	Value       reflect.Value
	StructField *reflect.StructField
}

func (dst dstValue) Elem() dstValue {
	return dstValue{dst.Type.Elem(), dst.Value.Elem(), dst.StructField}
}

func (dst dstValue) Index(i int) dstValue {
	v := dst.Value.Index(i)
	return dstValue{v.Type(), v, nil}
}

func (dst dstValue) Kind() reflect.Kind {
	return dst.Type.Kind()
}

func (dst dstValue) Assign(val interface{}) error {
	return dst.AssignRaw(reflect.ValueOf(val).Convert(dst.Type))
}

func (dst dstValue) AssignRaw(val reflect.Value) error {
	var recovered error
	err := func() error {
		defer func() {
			if r := recover(); r != nil {
				recovered = fmt.Errorf("%v", r)
			}
		}()

		if !dst.Value.CanSet() {
			return fmt.Errorf("cannot assign to %s", dst.Type.String())
		}

		dst.Value.Set(val)
		return nil
	}()

	if recovered != nil {
		return recovered
	}
	return err
}

type unmarshalHandler func(src interface{}, dst dstValue) (handled bool, err error)

func jsonUnmarshal(data []byte, dst interface{}, f unmarshalHandler) error {
	var raw interface{}
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return fmt.Errorf("malformed json: %s", err.Error())
	}

	t := reflect.TypeOf(dst)
	v := reflect.ValueOf(dst)
	return jsonUnmarshalValue(raw, dstValue{t, v, nil}, f)
}

func jsonUnmarshalValue(src interface{}, dst dstValue, f unmarshalHandler) error {
	switch dst.Kind() {
	case reflect.Ptr:
		if src == nil {
			return dst.AssignRaw(reflect.Zero(dst.Type))
		}

		if dst.Value.IsNil() {
			dst.AssignRaw(reflect.New(dst.Type.Elem()))
		}
		return jsonUnmarshalValue(src, dst.Elem(), f)

	case reflect.Map:
		return fmt.Errorf("maps not yet supported")
	case reflect.Struct:
		return jsonUnmarshalStruct(src, dst, f)

	case reflect.Array:
		return jsonUnmarshalArray(src, dst, f)

	case reflect.Slice:
		return jsonUnmarshalSlice(src, dst, f)

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
		return dst.Assign(src)

	default:
		return fmt.Errorf("data type %s not yet supported", dst.Kind())
	}
}

func jsonUnmarshalStruct(src interface{}, dst dstValue, f unmarshalHandler) error {
	dstFields := make(map[string]dstValue)

	fieldCount := dst.Type.NumField()
	for i := 0; i < fieldCount; i++ {
		field := dst.Type.Field(i)
		jsonTag := strings.Split(field.Tag.Get("json"), ",")
		fieldName := field.Name
		if len(jsonTag[0]) > 0 {
			if jsonTag[0] == "-" {
				continue
			} else {
				fieldName = jsonTag[0]
			}
		}

		//TODO omitempty and string options in jsonTag[1]
		dstFields[fieldName] = dstValue{field.Type, dst.Value.Field(i), &field}
	}

	srcFields, ok := src.(map[string]interface{})
	if !ok {
		return fmt.Errorf("cannot assign %T to %s", src, dst.Type.Name())
	}

	for srcName, srcValue := range srcFields {
		if dstField, ok := dstFields[srcName]; ok {
			if f != nil {
				handled, err := f(srcValue, dstField)
				if err != nil {
					return err
				}

				if handled {
					continue
				}
			}

			if err := jsonUnmarshalValue(srcValue, dstField, f); err != nil {
				return err
			}
		}
	}

	return nil
}

func jsonUnmarshalArray(src interface{}, dst dstValue, f unmarshalHandler) error {
	srcArray := reflect.ValueOf(src)

	len := srcArray.Len()
	if len > dst.Value.Len() {
		len = dst.Value.Len()
	}

	for i := 0; i < len; i++ {
		if err := jsonUnmarshalValue(srcArray.Index(i).Interface(), dst.Index(i), f); err != nil {
			return err
		}
	}

	return nil
}

func jsonUnmarshalSlice(src interface{}, dst dstValue, f unmarshalHandler) error {
	srcSlice := reflect.ValueOf(src)
	len := srcSlice.Len()

	if err := dst.AssignRaw(reflect.MakeSlice(dst.Type, len, len)); err != nil {
		return err
	}

	for i := 0; i < len; i++ {
		if err := jsonUnmarshalValue(srcSlice.Index(i).Interface(), dst.Index(i), f); err != nil {
			return err
		}
	}

	return nil
}
