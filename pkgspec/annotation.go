package pkgspec

import (
	"fmt"
	"path"
	"reflect"
)

// AnnotateFileMetadata sets the source file path on all embedded FileMetadata
// values found within v. It recursively walks v using reflection.
func AnnotateFileMetadata(file string, v any) {
	annotateFileMetadata(file, v)
}

// AnnotateFieldPointers sets the JsonPointer field on each Field in the tree
// based on its array index position, using RFC 6901 JSON Pointer format
// (e.g. /0/fields/1).
func AnnotateFieldPointers(fields []Field) {
	annotateFieldPointers(fields, "")
}

// PrefixFileMetadata prepends the given prefix to the file path of all
// embedded FileMetadata values found within v. It recursively walks v
// using reflection, joining prefix and the existing file path with
// [path.Join]. FileMetadata values with an empty file path are skipped.
func PrefixFileMetadata(prefix string, v any) {
	filePrefix{prefix: prefix}.walk(reflect.ValueOf(v))
}

type filePrefix struct {
	prefix string
}

func (p filePrefix) walk(val reflect.Value) {
	if val.CanAddr() && val.CanSet() {
		if m, ok := val.Addr().Interface().(*FileMetadata); ok {
			if m.file != "" {
				m.file = path.Join(p.prefix, m.file)
			}
			return
		}
	}

	switch val.Kind() {
	case reflect.Pointer:
		if !val.IsNil() {
			p.walk(val.Elem())
		}
	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			p.walk(val.Field(i))
		}
	case reflect.Slice:
		for i := 0; i < val.Len(); i++ {
			p.walk(val.Index(i))
		}
	case reflect.Map:
		itr := val.MapRange()
		for itr.Next() {
			p.walk(itr.Value())
		}
	}
}

func annotateFieldPointers(fields []Field, prefix string) {
	for i := range fields {
		fields[i].JsonPointer = fmt.Sprintf("%s/%d", prefix, i)
		if len(fields[i].Fields) > 0 {
			annotateFieldPointers(fields[i].Fields, fields[i].JsonPointer+"/fields")
		}
	}
}
