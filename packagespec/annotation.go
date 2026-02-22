package packagespec

// AnnotateFileMetadata sets the source file path on all embedded FileMetadata
// values found within v. It recursively walks v using reflection.
func AnnotateFileMetadata(file string, v any) {
	annotateFileMetadata(file, v)
}
