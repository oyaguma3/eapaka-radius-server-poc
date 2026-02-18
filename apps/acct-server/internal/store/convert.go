package store

import (
	"fmt"
	"reflect"
	"strconv"
)

// StructToMap はredisタグ付き構造体をmap[string]interface{}に変換する。
// redis:"-"タグおよびタグなしフィールドはスキップする。
func StructToMap(v any) map[string]any {
	result := make(map[string]any)
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("redis")
		if tag == "" || tag == "-" {
			continue
		}
		result[tag] = val.Field(i).Interface()
	}
	return result
}

// MapToStruct はmap[string]stringからredisタグ付き構造体にデシリアライズする。
func MapToStruct(m map[string]string, v any) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Pointer || val.IsNil() {
		return fmt.Errorf("MapToStruct: pointer required")
	}
	val = val.Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("redis")
		if tag == "" || tag == "-" {
			continue
		}
		strVal, ok := m[tag]
		if !ok {
			continue
		}
		if err := setFieldValue(val.Field(i), strVal); err != nil {
			return fmt.Errorf("field %s: %w", field.Name, err)
		}
	}
	return nil
}

// setFieldValue は文字列値を対象フィールドの型に変換して設定する。
func setFieldValue(field reflect.Value, strVal string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(strVal)
	case reflect.Int, reflect.Int64:
		n, err := strconv.ParseInt(strVal, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int value %q: %w", strVal, err)
		}
		field.SetInt(n)
	case reflect.Uint8:
		n, err := strconv.ParseUint(strVal, 10, 8)
		if err != nil {
			return fmt.Errorf("invalid uint8 value %q: %w", strVal, err)
		}
		field.SetUint(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(strVal)
		if err != nil {
			return fmt.Errorf("invalid bool value %q: %w", strVal, err)
		}
		field.SetBool(b)
	default:
		return fmt.Errorf("unsupported type: %s", field.Kind())
	}
	return nil
}
