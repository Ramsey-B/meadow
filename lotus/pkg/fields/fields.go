// Package fields defines the schema for source and target data structures.
//
// # Overview
//
// Fields define the shape of data that flows through a mapping. They specify:
//   - What fields exist in source/target data
//   - The data type of each field
//   - How to access the field (via Path)
//   - Nested structures (objects and arrays)
//
// # Field Types
//
// Scalar types: string, number, bool, date, any
// Complex types: object (nested fields), array (with Items)
//
// # Example: Simple Fields
//
//	Fields{
//	  {ID: "name", Path: "name", Type: ValueTypeString},
//	  {ID: "age", Path: "age", Type: ValueTypeNumber},
//	}
//
// # Example: Nested Object
//
//	Field{
//	  ID: "address", Path: "address", Type: ValueTypeObject,
//	  Fields: Fields{
//	    {ID: "city", Path: "city", Type: ValueTypeString},
//	    {ID: "zip", Path: "zip", Type: ValueTypeString},
//	  },
//	}
//
// # Example: Array with Items
//
//	Field{
//	  ID: "tags", Path: "tags", Type: ValueTypeArray,
//	  Items: &Field{ID: "tag", Type: ValueTypeString},
//	}
//
// # ID vs Path
//
// ID: Unique identifier used in links (e.g., "src_name", "tgt_name")
// Path: JSON path to access the value in data (e.g., "user.profile.name")
//
// This separation allows fields to be renamed/reorganized without breaking links.
package fields

import (
	"fmt"

	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
)

// Field defines a single field in source or target data.
//
// For objects: use Fields to define nested structure.
// For arrays: use Items to define the element type.
type Field struct {
	ID       string           `json:"id" validate:"required"`                                                    // Unique identifier for linking
	Name     string           `json:"name" validate:"required"`                                                  // Human-readable name
	Path     string           `json:"path" validate:"required"`                                                  // JSON path to access value
	Type     models.ValueType `json:"type" validate:"required,oneof=string int float bool array object date any"` // Data type
	Items    *Field           `json:"items" validate:"omitempty"`                                                // For arrays: element schema
	Fields   Fields           `json:"fields" validate:"omitempty"`                                               // For objects: nested fields
	Required bool             `json:"required" validate:"omitempty"`                                             // Whether field must exist
	Default  any              `json:"default" validate:"omitempty"`                                              // Default value if missing
	IsItem   bool             `json:"-"`                                                                         // Internal: true if this is an array item
}

func (f *Field) GetType() models.ActionValueType {
	if f.Items != nil {
		return models.ActionValueType{
			Type:  f.Type,
			Items: f.Items.GetType().Type,
		}
	}

	return models.ActionValueType{
		Type: f.Type,
	}
}

func (f *Field) Validate() error {
	if f.Items != nil {
		return f.Items.Validate()
	}

	if f.Type == models.ValueTypeArray {
		if f.Items == nil {
			return errors.NewMappingError("array type requires an items field").AddField(f.ID)
		}
		return f.Items.Validate()
	}

	if f.Items != nil {
		return errors.NewMappingError("items are not allowed on non-array types").AddField(f.ID)
	}

	if f.Type == models.ValueTypeObject {
		if len(f.Fields) == 0 {
			return errors.NewMappingError("object type requires a fields array").AddField(f.ID)
		}
		for _, field := range f.Fields {
			if err := field.Validate(); err != nil {
				return err
			}
		}
		return nil
	}

	if len(f.Fields) > 0 {
		return errors.NewMappingError("fields are not allowed on non-object types").AddField(f.ID)
	}

	return nil
}

type Fields []Field

func (f Fields) GetField(id string) (Field, error) {
	for _, field := range f {
		field, err := field.GetField(id)
		if err == nil {
			return field, nil
		}
	}

	return Field{}, errors.NewMappingError("field not found").AddField(id)
}

func (f Field) GetField(id string) (Field, error) {
	if f.ID == id {
		return f, nil
	}

	if f.Items != nil {
		if f.Items.ID == id {
			f.Items.IsItem = true
			return *f.Items, nil
		}

		// try recursively
		child, _ := f.Items.GetField(id)
		if child.ID != "" {
			// Any field found under Items is, by definition, within an array item context.
			child.IsItem = true
			return child, nil
		}
	}

	for _, field := range f.Fields {
		child, _ := field.GetField(id)
		if child.ID != "" {
			return child, nil
		}
	}

	return Field{}, errors.NewMappingError("field not found").AddField(id)
}

func (f Fields) GetFieldPath(id string) (string, error) {
	for _, field := range f {
		if field.ID == id {
			return field.Path, nil
		}

		childPath, err := field.Fields.GetFieldPath(id)
		if err != nil {
			return "", err
		}

		if childPath != "" {
			return fmt.Sprintf("%s.%s", field.Path, childPath), nil
		}
	}

	return "", errors.NewMappingError("field not found").AddField(id)
}

type PathToField struct {
	ParentID string
	FieldID  string
	IsItem   bool
	IsField  bool
}

type FieldPaths []PathToField

func (fields Fields) GetFieldPaths() FieldPaths {
	paths := FieldPaths{}

	for _, field := range fields {
		path := FieldPaths{
			{
				ParentID: "",
				FieldID:  field.ID,
				IsItem:   false,
				IsField:  false,
			},
		}

		if field.Items != nil {
			path = append(path, PathToField{
				ParentID: field.ID,
				FieldID:  field.Items.ID,
				IsItem:   true,
			})

			// If the array item is an object, include paths for its nested fields too.
			itemSubPaths := field.Items.Fields.GetFieldPaths()
			itemSubPaths = ectolinq.Map(itemSubPaths, func(p PathToField) PathToField {
				if p.ParentID == "" {
					p.ParentID = field.Items.ID
				}
				if !p.IsItem {
					p.IsField = true
				}
				return p
			})
			paths = append(paths, itemSubPaths...)
		}

		// check fields
		subPaths := field.Fields.GetFieldPaths()
		subPaths = ectolinq.Map(subPaths, func(p PathToField) PathToField {
			if p.ParentID == "" {
				p.ParentID = field.ID
			}
		// Any non-item child path represents traversing into an object field.
		// Mark it so the mapping engine can correctly build nested objects.
		if !p.IsItem {
			p.IsField = true
		}

			return p
		})

		paths = append(paths, path...)
		paths = append(paths, subPaths...)
	}

	return paths
}

func (paths FieldPaths) GetPathToField(fieldID string) (FieldPaths, error) {
	path := ectolinq.Find(paths, func(p PathToField) bool {
		return p.FieldID == fieldID
	})

	if ectolinq.IsEmpty(path) {
		return FieldPaths{}, errors.NewMappingErrorf("path to field '%s' not found", fieldID).AddField(fieldID)
	}

	// if the parent is the top, return the path
	if path.ParentID == "" {
		return FieldPaths{
			path,
		}, nil
	}

	parentPath, err := paths.GetPathToField(path.ParentID)
	if err != nil {
		return nil, err
	}

	return append(parentPath, path), nil
}

func (paths FieldPaths) Order() FieldPaths {
	lastPath := ectolinq.Find(paths, func(p PathToField) bool {
		return p.ParentID == ""
	})

	newPaths := FieldPaths{
		lastPath,
	}

	for {
		lastPath = ectolinq.Find(paths, func(p PathToField) bool {
			return p.ParentID == lastPath.FieldID
		})

		if ectolinq.IsEmpty(lastPath) {
			break
		}

		newPaths = append(newPaths, lastPath)
	}

	return newPaths
}
