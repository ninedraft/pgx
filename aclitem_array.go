// Code generated by erb. DO NOT EDIT.

package pgtype

import (
	"database/sql/driver"
	"reflect"

	errors "golang.org/x/xerrors"
)

type ACLItemArray struct {
	Elements   []ACLItem
	Dimensions []ArrayDimension
	Status     Status
}

func (dst *ACLItemArray) Set(src interface{}) error {
	// untyped nil and typed nil interfaces are different
	if src == nil {
		*dst = ACLItemArray{Status: Null}
		return nil
	}

	if value, ok := src.(interface{ Get() interface{} }); ok {
		value2 := value.Get()
		if value2 != value {
			return dst.Set(value2)
		}
	}

	value := reflect.ValueOf(src)
	if !value.IsValid() || value.IsZero() {
		*dst = ACLItemArray{Status: Null}
		return nil
	}

	dimensions, elementsLength, ok := findDimensionsFromValue(reflect.ValueOf(src), nil, 0)
	if !ok {
		return errors.Errorf("cannot find dimensions of %v for ACLItemArray", src)
	}
	if elementsLength == 0 {
		*dst = ACLItemArray{Status: Present}
		return nil
	}
	if len(dimensions) == 0 {
		if originalSrc, ok := underlyingSliceType(src); ok {
			return dst.Set(originalSrc)
		}
		return errors.Errorf("cannot convert %v to ACLItemArray", src)
	}

	*dst = ACLItemArray{
		Elements:   make([]ACLItem, elementsLength),
		Dimensions: dimensions,
		Status:     Present,
	}
	elementCount, err := dst.setRecursive(reflect.ValueOf(src), 0, 0)
	if err != nil {
		// Maybe the target was one dimension too far, try again:
		if len(dst.Dimensions) > 1 {
			dst.Dimensions = dst.Dimensions[:len(dst.Dimensions)-1]
			elementsLength = 0
			for _, dim := range dst.Dimensions {
				if elementsLength == 0 {
					elementsLength = int(dim.Length)
				} else {
					elementsLength *= int(dim.Length)
				}
			}
			dst.Elements = make([]ACLItem, elementsLength)
			elementCount, err = dst.setRecursive(reflect.ValueOf(src), 0, 0)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	if elementCount != len(dst.Elements) {
		return errors.Errorf("cannot convert %v to ACLItemArray, expected %d dst.Elements, but got %d instead", src, len(dst.Elements), elementCount)
	}

	return nil
}

func (dst *ACLItemArray) setRecursive(value reflect.Value, index, dimension int) (int, error) {
	switch value.Kind() {
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		if len(dst.Dimensions) == dimension {
			break
		}

		if int32(value.Len()) != dst.Dimensions[dimension].Length {
			return 0, errors.Errorf("multidimensional arrays must have array expressions with matching dimensions")
		}
		for i := 0; i < value.Len(); i++ {
			var err error
			index, err = dst.setRecursive(value.Index(i), index, dimension+1)
			if err != nil {
				return 0, err
			}
		}

		return index, nil
	}
	if !value.CanInterface() {
		return 0, errors.Errorf("cannot convert all values to ACLItemArray")
	}
	if err := dst.Elements[index].Set(value.Interface()); err != nil {
		return 0, errors.Errorf("%v in ACLItemArray", err)
	}
	index++

	return index, nil
}

func (dst ACLItemArray) Get() interface{} {
	switch dst.Status {
	case Present:
		return dst
	case Null:
		return nil
	default:
		return dst.Status
	}
}

func (src *ACLItemArray) AssignTo(dst interface{}) error {
	switch src.Status {
	case Present:
		value := reflect.ValueOf(dst)
		if value.Kind() == reflect.Ptr {
			value = value.Elem()
		}
		if !value.CanSet() {
			if nextDst, retry := GetAssignToDstType(dst); retry {
				return src.AssignTo(nextDst)
			}
			return errors.Errorf("unable to assign to %T", dst)
		}

		elementCount, err := src.assignToRecursive(value, 0, 0)
		if err != nil {
			return err
		}
		if elementCount != len(src.Elements) {
			return errors.Errorf("cannot assign %v, needed to assign %d elements, but only assigned %d", dst, len(src.Elements), elementCount)
		}

		return nil
	case Null:
		return NullAssignTo(dst)
	}

	return errors.Errorf("cannot decode %#v into %T", src, dst)
}

func (src *ACLItemArray) assignToRecursive(value reflect.Value, index, dimension int) (int, error) {
	switch kind := value.Kind(); kind {
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		if len(src.Dimensions) == dimension {
			break
		}

		length := int(src.Dimensions[dimension].Length)
		if reflect.Array == kind {
			if value.Type().Len() != length {
				return 0, errors.Errorf("expected size %d array, but %s has size %d array", length, value.Type(), value.Type().Len())
			}
			value.Set(reflect.New(value.Type()).Elem())
		} else {
			value.Set(reflect.MakeSlice(value.Type(), length, length))
		}

		var err error
		for i := 0; i < length; i++ {
			index, err = src.assignToRecursive(value.Index(i), index, dimension+1)
			if err != nil {
				return 0, err
			}
		}

		return index, nil
	}
	if len(src.Dimensions) != dimension {
		return 0, errors.Errorf("incorrect dimensions, expected %d, found %d", len(src.Dimensions), dimension)
	}
	if !value.CanAddr() || !value.Addr().CanInterface() {
		return 0, errors.Errorf("cannot assign all values from ACLItemArray")
	}
	err := src.Elements[index].AssignTo(value.Addr().Interface())
	if err != nil {
		return 0, err
	}
	index++
	return index, nil
}

func (dst *ACLItemArray) DecodeText(ci *ConnInfo, src []byte) error {
	if src == nil {
		*dst = ACLItemArray{Status: Null}
		return nil
	}

	uta, err := ParseUntypedTextArray(string(src))
	if err != nil {
		return err
	}

	var elements []ACLItem

	if len(uta.Elements) > 0 {
		elements = make([]ACLItem, len(uta.Elements))

		for i, s := range uta.Elements {
			var elem ACLItem
			var elemSrc []byte
			if s != "NULL" {
				elemSrc = []byte(s)
			}
			err = elem.DecodeText(ci, elemSrc)
			if err != nil {
				return err
			}

			elements[i] = elem
		}
	}

	*dst = ACLItemArray{Elements: elements, Dimensions: uta.Dimensions, Status: Present}

	return nil
}

func (src ACLItemArray) EncodeText(ci *ConnInfo, buf []byte) ([]byte, error) {
	switch src.Status {
	case Null:
		return nil, nil
	case Undefined:
		return nil, errUndefined
	}

	if len(src.Dimensions) == 0 {
		return append(buf, '{', '}'), nil
	}

	buf = EncodeTextArrayDimensions(buf, src.Dimensions)

	// dimElemCounts is the multiples of elements that each array lies on. For
	// example, a single dimension array of length 4 would have a dimElemCounts of
	// [4]. A multi-dimensional array of lengths [3,5,2] would have a
	// dimElemCounts of [30,10,2]. This is used to simplify when to render a '{'
	// or '}'.
	dimElemCounts := make([]int, len(src.Dimensions))
	dimElemCounts[len(src.Dimensions)-1] = int(src.Dimensions[len(src.Dimensions)-1].Length)
	for i := len(src.Dimensions) - 2; i > -1; i-- {
		dimElemCounts[i] = int(src.Dimensions[i].Length) * dimElemCounts[i+1]
	}

	inElemBuf := make([]byte, 0, 32)
	for i, elem := range src.Elements {
		if i > 0 {
			buf = append(buf, ',')
		}

		for _, dec := range dimElemCounts {
			if i%dec == 0 {
				buf = append(buf, '{')
			}
		}

		elemBuf, err := elem.EncodeText(ci, inElemBuf)
		if err != nil {
			return nil, err
		}
		if elemBuf == nil {
			buf = append(buf, `NULL`...)
		} else {
			buf = append(buf, QuoteArrayElementIfNeeded(string(elemBuf))...)
		}

		for _, dec := range dimElemCounts {
			if (i+1)%dec == 0 {
				buf = append(buf, '}')
			}
		}
	}

	return buf, nil
}

// Scan implements the database/sql Scanner interface.
func (dst *ACLItemArray) Scan(src interface{}) error {
	if src == nil {
		return dst.DecodeText(nil, nil)
	}

	switch src := src.(type) {
	case string:
		return dst.DecodeText(nil, []byte(src))
	case []byte:
		srcCopy := make([]byte, len(src))
		copy(srcCopy, src)
		return dst.DecodeText(nil, srcCopy)
	}

	return errors.Errorf("cannot scan %T", src)
}

// Value implements the database/sql/driver Valuer interface.
func (src ACLItemArray) Value() (driver.Value, error) {
	buf, err := src.EncodeText(nil, nil)
	if err != nil {
		return nil, err
	}
	if buf == nil {
		return nil, nil
	}

	return string(buf), nil
}
