//go:build js && wasm

package core

import "syscall/js"

// VNodeType represents the type of a virtual DOM node
type VNodeType int

const (
	VNodeElement   VNodeType = iota // <div>, <p>, etc.
	VNodeText                       // plain text
	VNodeComponent                  // custom component function
	VNodeEmpty                      // empty placeholder
)

// VNode is a virtual DOM node — the core data structure of the UI tree.
// It is a lightweight description of what the DOM should look like.
// The reconciler compares old and new VNode trees to produce minimal DOM mutations.
type VNode struct {
	Type      VNodeType
	Tag       string            // HTML tag name (e.g. "div", "span", "h1")
	Props     map[string]string // HTML attributes + event handlers
	Style     map[string]string // inline styles
	Text      string            // text content (for VNodeText)
	Children  []*VNode          // child nodes
	Key       string            // unique key for reconciliation optimization
	Component Component         // component constructor (for VNodeComponent)
	CompProps Props             // props passed to component

	// Internal: linked DOM element after mount
	domElement js.Value
	// Internal: component instance for VNodeComponent
	compInstance *componentInstance
}

// H creates a virtual DOM element.
// Usage: H("div", Attr{"class": "hero"}, H("h1", nil, "Hello"))
func H(tag string, props map[string]string, children ...interface{}) *VNode {
	v := &VNode{
		Type:     VNodeElement,
		Tag:      tag,
		Props:    props,
		Style:    make(map[string]string),
		Children: make([]*VNode, 0),
	}
	if props == nil {
		v.Props = make(map[string]string)
	}

	for _, child := range children {
		if child == nil {
			continue
		}
		switch c := child.(type) {
		case *VNode:
			v.Children = append(v.Children, c)
		case string:
			v.Children = append(v.Children, &VNode{Type: VNodeText, Text: c})
		case []*VNode:
			v.Children = append(v.Children, c...)
		case js.Value:
			// Raw DOM element — wrap as text
			text := js.Global().Get("document").Call("createElement", "div")
			text.Call("appendChild", c)
			v.Children = append(v.Children, &VNode{Type: VNodeText, Text: text.Get("outerHTML").String()})
		}
	}
	return v
}

// Text creates a text VNode
func Text(content string) *VNode {
	return &VNode{Type: VNodeText, Text: content}
}

// ComponentVNode creates a component VNode
func ComponentVNode(comp Component, props Props, children ...*VNode) *VNode {
	return &VNode{
		Type:      VNodeComponent,
		Component: comp,
		CompProps: props,
		Children:  children,
	}
}

// Fragment creates a fragment (returns children directly, no wrapper)
func Fragment(children ...*VNode) []*VNode {
	return children
}

// Attr is a convenience type for building props maps
type Attr map[string]string

// MergeProps merges multiple attr maps
func MergeProps(attrs ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, a := range attrs {
		for k, v := range a {
			result[k] = v
		}
	}
	return result
}

// VNodeType returns "element", "text", or "component" (debug)
func (v *VNode) VNodeKind() string {
	switch v.Type {
	case VNodeElement:
		return "element"
	case VNodeText:
		return "text"
	case VNodeComponent:
		return "component"
	default:
		return "empty"
	}
}

// Clone creates a shallow snapshot of a VNode for reconciliation comparison.
// Props and Style maps are referenced (not duplicated) since reconciliation
// diffs them by key; only the children slice is copied and child VNodes cloned.
func (v *VNode) Clone() *VNode {
	clone := &VNode{
		Type:      v.Type,
		Tag:       v.Tag,
		Props:     v.Props,
		Style:     v.Style,
		Text:      v.Text,
		Key:       v.Key,
		Component: v.Component,
		CompProps: v.CompProps,
	}
	if len(v.Children) > 0 {
		clone.Children = make([]*VNode, len(v.Children))
		for i, child := range v.Children {
			clone.Children[i] = child.Clone()
		}
	}
	return clone
}
