// Package expression - Evaluation context
package expression

import (
	"fmt"
	"sync"
)

// Context provides values for expression evaluation
type Context struct {
	mu sync.RWMutex

	// Variables from tfvars, defaults, CLI
	variables map[string]Value

	// Locals computed from local blocks
	locals map[string]Value

	// Resources indexed by address
	resources map[string]Value

	// Data sources indexed by address
	dataSources map[string]Value

	// Modules indexed by key
	modules map[string]*Context

	// Parent context for nested modules
	parent *Context

	// Module path for this context
	modulePath string

	// Workspace name
	workspace string

	// Path values
	pathModule string
	pathRoot   string
	pathCwd    string

	// Count/for_each context (when evaluating inside a resource)
	countIndex *int
	eachKey    *string
	eachValue  Value

	// Self reference (when evaluating provisioners)
	self Value
}

// NewContext creates a new evaluation context
func NewContext() *Context {
	return &Context{
		variables:   make(map[string]Value),
		locals:      make(map[string]Value),
		resources:   make(map[string]Value),
		dataSources: make(map[string]Value),
		modules:     make(map[string]*Context),
		workspace:   "default",
	}
}

// NewChildContext creates a child context for a module
func (c *Context) NewChildContext(modulePath string) *Context {
	child := NewContext()
	child.parent = c
	child.modulePath = modulePath
	child.workspace = c.workspace
	child.pathRoot = c.pathRoot
	child.pathCwd = c.pathCwd
	return child
}

// SetVariable sets a variable value
func (c *Context) SetVariable(name string, value Value) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.variables[name] = value
}

// SetVariables sets multiple variables
func (c *Context) SetVariables(vars map[string]Value) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range vars {
		c.variables[k] = v
	}
}

// SetLocal sets a local value
func (c *Context) SetLocal(name string, value Value) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.locals[name] = value
}

// SetResource sets a resource's computed values
func (c *Context) SetResource(address string, value Value) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resources[address] = value
}

// SetDataSource sets a data source's values
func (c *Context) SetDataSource(address string, value Value) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dataSources[address] = value
}

// SetModule adds a child module context
func (c *Context) SetModule(key string, child *Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.modules[key] = child
}

// SetWorkspace sets the workspace name
func (c *Context) SetWorkspace(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workspace = name
}

// SetPaths sets the path.module, path.root, path.cwd values
func (c *Context) SetPaths(module, root, cwd string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pathModule = module
	c.pathRoot = root
	c.pathCwd = cwd
}

// SetCountIndex sets the count.index value for resource evaluation
func (c *Context) SetCountIndex(index int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.countIndex = &index
}

// SetEach sets the each.key and each.value for for_each evaluation
func (c *Context) SetEach(key string, value Value) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eachKey = &key
	c.eachValue = value
}

// SetSelf sets the self reference value
func (c *Context) SetSelf(value Value) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.self = value
}

// ClearIterators clears count/for_each context
func (c *Context) ClearIterators() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.countIndex = nil
	c.eachKey = nil
	c.eachValue = Null()
}

// Resolve resolves a reference to a value
func (c *Context) Resolve(ref *Reference) (Value, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch ref.Kind {
	case RefVariable:
		return c.resolveVariable(ref)
	case RefLocal:
		return c.resolveLocal(ref)
	case RefResource:
		return c.resolveResource(ref)
	case RefData:
		return c.resolveDataSource(ref)
	case RefModule:
		return c.resolveModule(ref)
	case RefSelf:
		return c.resolveSelf(ref)
	case RefCount:
		return c.resolveCount(ref)
	case RefEach:
		return c.resolveEach(ref)
	case RefPath:
		return c.resolvePath(ref)
	case RefTerraform:
		return c.resolveTerraform(ref)
	default:
		return Unknown(), fmt.Errorf("unknown reference kind: %v", ref.Kind)
	}
}

func (c *Context) resolveVariable(ref *Reference) (Value, error) {
	val, ok := c.variables[ref.Key]
	if !ok {
		return Unknown(), fmt.Errorf("undefined variable: %s", ref.Key)
	}
	return c.traverseValue(val, ref)
}

func (c *Context) resolveLocal(ref *Reference) (Value, error) {
	val, ok := c.locals[ref.Key]
	if !ok {
		return Unknown(), fmt.Errorf("undefined local: %s", ref.Key)
	}
	return c.traverseValue(val, ref)
}

func (c *Context) resolveResource(ref *Reference) (Value, error) {
	// Build address with optional index
	addr := ref.Subject
	if ref.Index != nil {
		addr = ref.ResourceAddress()
	}

	val, ok := c.resources[addr]
	if !ok {
		// Resource not yet computed - return unknown
		return Unknown(), nil
	}
	return c.traverseValue(val, ref)
}

func (c *Context) resolveDataSource(ref *Reference) (Value, error) {
	addr := ref.ResourceAddress()
	val, ok := c.dataSources[addr]
	if !ok {
		// Data source not yet computed - return unknown
		return Unknown(), nil
	}
	return c.traverseValue(val, ref)
}

func (c *Context) resolveModule(ref *Reference) (Value, error) {
	child, ok := c.modules[ref.Key]
	if !ok {
		return Unknown(), fmt.Errorf("undefined module: %s", ref.Key)
	}

	// Module references access outputs
	if ref.Attribute != "" {
		// Look for output in child context
		// Outputs would be stored in a special location
		outputVal, ok := child.locals["__output_"+ref.Attribute]
		if !ok {
			return Unknown(), nil
		}
		return outputVal, nil
	}

	// Return the whole module context as an object
	return Unknown(), nil
}

func (c *Context) resolveSelf(ref *Reference) (Value, error) {
	if c.self.IsNull() {
		return Unknown(), fmt.Errorf("self is not available in this context")
	}
	return c.traverseValue(c.self, ref)
}

func (c *Context) resolveCount(ref *Reference) (Value, error) {
	if ref.Attribute != "index" {
		return Unknown(), fmt.Errorf("count only has 'index' attribute")
	}
	if c.countIndex == nil {
		return Unknown(), fmt.Errorf("count.index not available in this context")
	}
	return NumberFromInt(int64(*c.countIndex)), nil
}

func (c *Context) resolveEach(ref *Reference) (Value, error) {
	switch ref.Attribute {
	case "key":
		if c.eachKey == nil {
			return Unknown(), fmt.Errorf("each.key not available in this context")
		}
		return String(*c.eachKey), nil
	case "value":
		if c.eachKey == nil {
			return Unknown(), fmt.Errorf("each.value not available in this context")
		}
		return c.eachValue, nil
	default:
		return Unknown(), fmt.Errorf("each only has 'key' and 'value' attributes")
	}
}

func (c *Context) resolvePath(ref *Reference) (Value, error) {
	switch ref.Attribute {
	case "module":
		return String(c.pathModule), nil
	case "root":
		return String(c.pathRoot), nil
	case "cwd":
		return String(c.pathCwd), nil
	default:
		return Unknown(), fmt.Errorf("path.%s is not a valid path reference", ref.Attribute)
	}
}

func (c *Context) resolveTerraform(ref *Reference) (Value, error) {
	switch ref.Attribute {
	case "workspace":
		return String(c.workspace), nil
	default:
		return Unknown(), fmt.Errorf("terraform.%s is not a valid terraform reference", ref.Attribute)
	}
}

// traverseValue follows attribute access path through a value
func (c *Context) traverseValue(val Value, ref *Reference) (Value, error) {
	// If we have an attribute to access, traverse
	if ref.Attribute != "" {
		attr, err := val.GetAttr(ref.Attribute)
		if err != nil {
			return Unknown(), nil // Unknown attributes
		}
		val = attr
	}

	// Traverse remaining path
	for _, seg := range ref.Remaining {
		attr, err := val.GetAttr(seg)
		if err != nil {
			return Unknown(), nil
		}
		val = attr
	}

	return val, nil
}

// Clone creates a copy of the context
func (c *Context) Clone() *Context {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clone := NewContext()
	clone.parent = c.parent
	clone.modulePath = c.modulePath
	clone.workspace = c.workspace
	clone.pathModule = c.pathModule
	clone.pathRoot = c.pathRoot
	clone.pathCwd = c.pathCwd

	for k, v := range c.variables {
		clone.variables[k] = v
	}
	for k, v := range c.locals {
		clone.locals[k] = v
	}
	for k, v := range c.resources {
		clone.resources[k] = v
	}
	for k, v := range c.dataSources {
		clone.dataSources[k] = v
	}
	for k, v := range c.modules {
		clone.modules[k] = v
	}

	if c.countIndex != nil {
		idx := *c.countIndex
		clone.countIndex = &idx
	}
	if c.eachKey != nil {
		key := *c.eachKey
		clone.eachKey = &key
		clone.eachValue = c.eachValue
	}
	clone.self = c.self

	return clone
}
