package pkgspec

import "fmt"

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

func annotateFieldPointers(fields []Field, prefix string) {
	for i := range fields {
		fields[i].JsonPointer = fmt.Sprintf("%s/%d", prefix, i)
		if len(fields[i].Fields) > 0 {
			annotateFieldPointers(fields[i].Fields, fields[i].JsonPointer+"/fields")
		}
	}
}
