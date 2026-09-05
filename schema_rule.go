package linter

// SchemaRule checks a standalone YAML schema using the same public violation type.
type SchemaRule interface {
	Name() string
	VisitSchema(string, *SchemaDocument) []Violation
}
