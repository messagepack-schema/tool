package internal

import (
	"path/filepath"

	"github.com/mitchellh/hashstructure/v2"
	"tomasweigenast.com/schema_interpreter/internal"
)

const builderVersion = 1

// Builder provides a method to build a list of .nex files
type Builder struct {
	rootFolder    string
	currentParser *internal.Parser
	analyzer      *internal.Analyzer

	contexts []*internal.ResolvedContext
	typesId  map[*internal.TypeStmt]string
}

// NewBuilder creates a new Builder
func NewBuilder() *Builder {
	return &Builder{}
}

// Build is the main entry point for parsing nexema projects. A folder is given and
// it first scan for a nexema.yaml, and then start scanning .nex files.
func (b *Builder) Build(inputFolder string) error {

	b.rootFolder = inputFolder

	// the first step is to search for the nexema.yaml in the root folder. Do not search in subfolders. inputFolder
	// should be the root package.
	err := b.scanProject()
	if err != nil {
		return err
	}

	return nil
}

// scanProject searches and parses a nexema.yaml file in the current folder.
// If the file cannot be found, an error is returned.
func (b *Builder) scanProject() error {
	return nil
}

// buildDefinition takes the analyzed source and builds a NexemaDefinition for the entire project.
func (b *Builder) buildDefinition() *internal.NexemaDefinition {
	def := &internal.NexemaDefinition{
		Version:  builderVersion,
		Hashcode: 0,
		Files:    make([]internal.NexemaFile, 0),
	}

	files := map[string]*internal.NexemaFile{}

	for _, ctx := range b.contexts {
		ast := ctx.Owner
		fpath := filepath.Join(ast.File.Pkg, ast.File.Name)
		nexemaFile, ok := files[fpath]
		if !ok {
			nexemaFile = &internal.NexemaFile{
				Name:  fpath,
				Types: make([]internal.NexemaTypeDefinition, 0),
			}
			files[fpath] = nexemaFile
		}

		for _, typeStmt := range *ast.Types {
			typeId := b.typesId[typeStmt]
			typeDefinition := internal.NexemaTypeDefinition{
				Id:            typeId,
				Name:          typeStmt.Name.Lit,
				Modifier:      typeStmt.Modifier.String(),
				Documentation: make([]string, 0),
				Fields:        make([]internal.NexemaTypeFieldDefinition, 0),
			}

			if typeStmt.Documentation != nil {
				for _, stmt := range *typeStmt.Documentation {
					typeDefinition.Documentation = append(typeDefinition.Documentation, stmt.Text)
				}
			}

			if typeStmt.Fields != nil {
				for _, stmt := range *typeStmt.Fields {
					field := internal.NexemaTypeFieldDefinition{
						Index:    (stmt.Index.(*internal.PrimitiveValueStmt)).RawValue.(int64),
						Name:     stmt.Name.Lit,
						Metadata: make(map[string]any),
					}

					if typeStmt.Modifier != internal.Token_Enum {
						primitive := internal.GetPrimitive(stmt.ValueType.Ident.Lit)
						switch primitive {
						case internal.Primitive_Illegal, internal.Primitive_Type:
							// Get type id
							var alias *string
							if stmt.ValueType.Ident.Alias != "" {
								alias = &stmt.ValueType.Ident.Alias
							}

							t, _ := ctx.LookupType(stmt.ValueType.Ident.Lit, alias)
							id := b.typesId[t]

							valueType := internal.NexemaTypeValueType{
								Base: internal.BaseNexemaValueType{
									Kind:     "NexemaTypeValueType",
									Nullable: stmt.ValueType.Nullable,
								},
								TypeId:      id,
								ImportAlias: alias,
							}
							field.Type = valueType

						default:
							valueType := internal.NexemaPrimitiveValueType{
								Base: internal.BaseNexemaValueType{
									Kind:     "NexemaPrimitiveValueType",
									Nullable: stmt.ValueType.Nullable,
								},
								Primitive:     primitive.String(),
								TypeArguments: make([]internal.NexemaValueType, 0),
							}

							if stmt.ValueType.TypeArguments != nil {
								for _, typeArg := range *stmt.ValueType.TypeArguments {
									valueType.TypeArguments = append(valueType.TypeArguments, internal.NexemaPrimitiveValueType{
										Base: internal.BaseNexemaValueType{
											Kind:     "NexemaPrimitiveValueType",
											Nullable: typeArg.Nullable,
										},
										Primitive:     internal.GetPrimitive(typeArg.Ident.Lit).String(),
										TypeArguments: make([]internal.NexemaValueType, 0),
									})
								}
							}

							field.Type = valueType
						}

						if stmt.DefaultValue != nil {
							field.DefaultValue = stmt.DefaultValue.Value()
						}
					}

					if stmt.Metadata != nil {
						for _, entry := range *stmt.Metadata {
							key := (entry.Key.(*internal.PrimitiveValueStmt)).RawValue.(string)
							value := (entry.Value.(*internal.PrimitiveValueStmt)).RawValue
							field.Metadata[key] = value
						}
					}

					typeDefinition.Fields = append(typeDefinition.Fields, field)
				}
			}

			nexemaFile.Types = append(nexemaFile.Types, typeDefinition)
		}
	}

	for _, file := range files {
		def.Files = append(def.Files, *file)
	}

	// calculate hashcode
	hash, err := hashstructure.Hash(def.Files, hashstructure.FormatV2, nil)
	if err != nil {
		panic(err)
	}

	def.Hashcode = hash

	return def
}
