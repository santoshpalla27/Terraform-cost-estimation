// Package expression - Reference parsing and resolution
package expression

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ReferenceKind identifies the type of reference
type ReferenceKind int

const (
	RefUnknown ReferenceKind = iota
	RefVariable      // var.name
	RefLocal         // local.name
	RefResource      // resource_type.name or resource_type.name[index]
	RefData          // data.type.name
	RefModule        // module.name
	RefOutput        // output.name (within modules)
	RefSelf          // self.attr
	RefCount         // count.index
	RefEach          // each.key or each.value
	RefPath          // path.module, path.root, path.cwd
	RefTerraform     // terraform.workspace
)

// Reference represents a reference to another value in HCL
type Reference struct {
	Kind       ReferenceKind
	Subject    string   // The full subject (e.g., "aws_instance.web")
	Key        string   // Primary key (e.g., "web" for aws_instance.web)
	Type       string   // Resource type (e.g., "aws_instance")
	Attribute  string   // Attribute being accessed (e.g., "id")
	Index      *Index   // Optional index for count/for_each
	Remaining  []string // Remaining path segments
	RawString  string   // Original reference string
}

// Index represents a count or for_each index
type Index struct {
	IsNumeric bool
	NumValue  int
	StrValue  string
	IsUnknown bool
}

// NewNumericIndex creates a numeric index
func NewNumericIndex(n int) *Index {
	return &Index{IsNumeric: true, NumValue: n}
}

// NewStringIndex creates a string index
func NewStringIndex(s string) *Index {
	return &Index{IsNumeric: false, StrValue: s}
}

// UnknownIndex creates an unknown index
func UnknownIndex() *Index {
	return &Index{IsUnknown: true}
}

// String returns the index as a string
func (idx *Index) String() string {
	if idx == nil {
		return ""
	}
	if idx.IsUnknown {
		return "[?]"
	}
	if idx.IsNumeric {
		return fmt.Sprintf("[%d]", idx.NumValue)
	}
	return fmt.Sprintf("[%q]", idx.StrValue)
}

// ParseReference parses a reference string into structured form
func ParseReference(ref string) (*Reference, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("empty reference")
	}

	result := &Reference{RawString: ref}

	// Split by dots, but handle indices specially
	parts := splitReference(ref)
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid reference: %s", ref)
	}

	// Determine reference kind from first part
	switch parts[0] {
	case "var":
		result.Kind = RefVariable
		if len(parts) < 2 {
			return nil, fmt.Errorf("variable reference requires name: %s", ref)
		}
		result.Key = parts[1]
		result.Subject = "var." + result.Key
		if len(parts) > 2 {
			result.Remaining = parts[2:]
		}

	case "local":
		result.Kind = RefLocal
		if len(parts) < 2 {
			return nil, fmt.Errorf("local reference requires name: %s", ref)
		}
		result.Key = parts[1]
		result.Subject = "local." + result.Key
		if len(parts) > 2 {
			result.Remaining = parts[2:]
		}

	case "data":
		result.Kind = RefData
		if len(parts) < 3 {
			return nil, fmt.Errorf("data reference requires type and name: %s", ref)
		}
		result.Type = parts[1]
		result.Key, result.Index = parseKeyAndIndex(parts[2])
		result.Subject = fmt.Sprintf("data.%s.%s", result.Type, result.Key)
		if len(parts) > 3 {
			result.Remaining = parts[3:]
		}

	case "module":
		result.Kind = RefModule
		if len(parts) < 2 {
			return nil, fmt.Errorf("module reference requires name: %s", ref)
		}
		result.Key, result.Index = parseKeyAndIndex(parts[1])
		result.Subject = "module." + result.Key
		if len(parts) > 2 {
			result.Remaining = parts[2:]
		}

	case "self":
		result.Kind = RefSelf
		result.Subject = "self"
		if len(parts) > 1 {
			result.Attribute = parts[1]
			result.Remaining = parts[2:]
		}

	case "count":
		result.Kind = RefCount
		result.Subject = "count"
		if len(parts) > 1 && parts[1] == "index" {
			result.Attribute = "index"
		}

	case "each":
		result.Kind = RefEach
		result.Subject = "each"
		if len(parts) > 1 {
			result.Attribute = parts[1] // key or value
		}

	case "path":
		result.Kind = RefPath
		result.Subject = "path"
		if len(parts) > 1 {
			result.Attribute = parts[1] // module, root, cwd
		}

	case "terraform":
		result.Kind = RefTerraform
		result.Subject = "terraform"
		if len(parts) > 1 {
			result.Attribute = parts[1] // workspace
		}

	default:
		// Assume it's a resource reference: type.name
		result.Kind = RefResource
		result.Type = parts[0]
		if len(parts) < 2 {
			return nil, fmt.Errorf("resource reference requires name: %s", ref)
		}
		result.Key, result.Index = parseKeyAndIndex(parts[1])
		result.Subject = fmt.Sprintf("%s.%s", result.Type, result.Key)
		if len(parts) > 2 {
			result.Remaining = parts[2:]
		}
	}

	// Extract attribute from remaining if present
	if len(result.Remaining) > 0 {
		result.Attribute = result.Remaining[0]
		result.Remaining = result.Remaining[1:]
	}

	return result, nil
}

// splitReference splits a reference by dots, handling index brackets
func splitReference(ref string) []string {
	var parts []string
	var current strings.Builder
	inBracket := 0

	for _, ch := range ref {
		switch ch {
		case '[':
			inBracket++
			current.WriteRune(ch)
		case ']':
			inBracket--
			current.WriteRune(ch)
		case '.':
			if inBracket > 0 {
				current.WriteRune(ch)
			} else {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// indexPattern matches [0], [1], ["key"], etc.
var indexPattern = regexp.MustCompile(`^([^[]+)\[(.+)\]$`)

// parseKeyAndIndex extracts key and optional index from "name[index]"
func parseKeyAndIndex(s string) (string, *Index) {
	matches := indexPattern.FindStringSubmatch(s)
	if matches == nil {
		return s, nil
	}

	key := matches[1]
	indexStr := matches[2]

	// Try numeric index
	if n, err := strconv.Atoi(indexStr); err == nil {
		return key, NewNumericIndex(n)
	}

	// Try string index (quoted)
	if strings.HasPrefix(indexStr, "\"") && strings.HasSuffix(indexStr, "\"") {
		return key, NewStringIndex(indexStr[1 : len(indexStr)-1])
	}

	// Unknown/computed index
	return key, UnknownIndex()
}

// Address returns the full address including index
func (r *Reference) Address() string {
	addr := r.Subject
	if r.Index != nil {
		addr += r.Index.String()
	}
	if r.Attribute != "" {
		addr += "." + r.Attribute
	}
	for _, rem := range r.Remaining {
		addr += "." + rem
	}
	return addr
}

// ResourceAddress returns just the resource address without attribute
func (r *Reference) ResourceAddress() string {
	addr := r.Subject
	if r.Index != nil {
		addr += r.Index.String()
	}
	return addr
}

// String returns the reference as a string
func (r *Reference) String() string {
	return r.Address()
}

// ExtractReferences finds all references in a string (simple heuristic)
func ExtractReferences(s string) []*Reference {
	// Pattern for common reference formats
	patterns := []string{
		`var\.[a-zA-Z_][a-zA-Z0-9_]*`,
		`local\.[a-zA-Z_][a-zA-Z0-9_]*`,
		`data\.[a-zA-Z_][a-zA-Z0-9_]*\.[a-zA-Z_][a-zA-Z0-9_]*`,
		`module\.[a-zA-Z_][a-zA-Z0-9_]*`,
		`[a-zA-Z_][a-zA-Z0-9_]*\.[a-zA-Z_][a-zA-Z0-9_]*(\[[^\]]+\])?(\.[a-zA-Z_][a-zA-Z0-9_]*)*`,
	}

	var refs []*Reference
	seen := make(map[string]bool)

	for _, pat := range patterns {
		re := regexp.MustCompile(pat)
		matches := re.FindAllString(s, -1)
		for _, m := range matches {
			if seen[m] {
				continue
			}
			seen[m] = true

			ref, err := ParseReference(m)
			if err == nil {
				refs = append(refs, ref)
			}
		}
	}

	return refs
}
