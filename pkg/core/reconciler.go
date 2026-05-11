//go:build js && wasm

package core

import (
	"strings"
	"syscall/js"
)

// PatchOp describes a single DOM mutation to apply
type PatchOp struct {
	Type     int
	parent   *VNode
	node     *VNode
	index    int
	text     string
	propKey  string
	propVal  string
	styleKey string
	styleVal string
	// For keyed diff: reference node for insert/move
	refNode *VNode
}

const (
	patchReplace     = 0
	patchSetText     = 1
	patchSetProp     = 2
	patchRemoveProp  = 3
	patchInsert      = 4
	patchRemove      = 5
	patchSetStyle    = 6
	patchNop         = 7
	patchMove        = 8
	patchInsertKeyed = 9
	patchUpdateKeyed = 10
)

// Reconciler compares old and new VNode trees and applies minimal DOM mutations.
// Uses React's reconciliation algorithm:
//  1. Same type at same position → patch props/text, recurse children
//  2. Different type → replace entire subtree
//  3. Keys used to optimize list reordering (reuse instead of destroy+create)
type Reconciler struct {
	patchQueue []PatchOp
}

// NewReconciler creates a new reconciler
func NewReconciler() *Reconciler {
	return &Reconciler{}
}

// Render mounts or updates a VNode tree into a DOM root element.
// First call: full mount. Subsequent calls: diff + patch.
// If hydrateRoot is not null, the reconciler will attempt to reuse existing
// DOM nodes (from SSR) instead of creating new ones.
func (r *Reconciler) Render(root js.Value, oldTree, newTree *VNode) {
	r.patchQueue = r.patchQueue[:0]
	r.diff(oldTree, newTree)

	// Apply patches in order
	for _, op := range r.patchQueue {
		r.apply(op, root)
	}

	// If this was a full mount (no oldTree), append to root
	if oldTree == nil && newTree != nil {
		// Check if we're hydrating (SSR → CSR)
		if root.Type() == js.TypeObject && !root.IsNull() && root.Get("children").Get("length").Int() > 0 {
			r.hydrate(root, newTree)
		} else {
			r.mountToDOM(newTree)
			if root.Type() == js.TypeObject && !root.IsNull() {
				for _, child := range newTree.Children {
					root.Call("appendChild", child.domElement)
				}
			}
		}
	}
}

// hydrate walks the existing DOM and matches it to the VNode tree,
// reusing elements where types match (React-style hydration).
func (r *Reconciler) hydrate(root js.Value, tree *VNode) {
	if tree == nil {
		return
	}

	switch tree.Type {
	case VNodeText:
		// Find text node in root's children
		childNodes := root.Get("childNodes")
		for i := 0; i < childNodes.Get("length").Int(); i++ {
			node := childNodes.Call("item", i)
			if node.Get("nodeType").Int() == 3 { // TEXT_NODE
				if strings.TrimSpace(node.Get("textContent").String()) == strings.TrimSpace(tree.Text) {
					tree.domElement = node
					return
				}
			}
		}
		// No match found — create new
		tree.domElement = CreateTextNode(tree.Text)

	case VNodeElement:
		// Find matching element in root's children
		childNodes := root.Get("children")
		for i := 0; i < childNodes.Get("length").Int(); i++ {
			node := childNodes.Call("item", i)
			if !node.IsNull() && node.Get("tagName").String() == strings.ToUpper(tree.Tag) {
				// Tag matches — hydrate this element
				tree.domElement = node
				// Hydrate children recursively
				for j, child := range tree.Children {
					r.hydrateAtIndex(node, child, j)
				}
				return
			}
		}
		// No match — full mount
		r.mountToDOM(tree)

	case VNodeComponent:
		// Components mount fresh (they produce their own VNode)
		v := tree.Component.Render()
		if v != nil {
			r.hydrate(root, v)
		}
	}
}

// hydrateAtIndex hydrates a child VNode at a specific index within a parent element.
func (r *Reconciler) hydrateAtIndex(parent js.Value, tree *VNode, index int) {
	if tree == nil {
		return
	}

	childNodes := parent.Get("childNodes")
	if index >= childNodes.Get("length").Int() {
		r.mountToDOM(tree)
		parent.Call("appendChild", tree.domElement)
		return
	}

	node := childNodes.Call("item", index)
	switch tree.Type {
	case VNodeText:
		if node.Get("nodeType").Int() == 3 {
			tree.domElement = node
		} else {
			r.mountToDOM(tree)
			parent.Call("insertBefore", tree.domElement, node)
		}

	case VNodeElement:
		if node.Get("nodeType").Int() == 1 && node.Get("tagName").String() == strings.ToUpper(tree.Tag) {
			tree.domElement = node
			// Hydrate children
			for j, child := range tree.Children {
				r.hydrateAtIndex(node, child, j)
			}
		} else {
			r.mountToDOM(tree)
			parent.Call("insertBefore", tree.domElement, node)
		}
	}
}

// diff computes the minimal set of patches between old and new VNode trees
func (r *Reconciler) diff(oldNode, newNode *VNode) {
	// Case 1: both nil
	if oldNode == nil && newNode == nil {
		return
	}

	// Case 2: old is nil, new exists → full mount
	if oldNode == nil && newNode != nil {
		r.mountToDOM(newNode)
		return
	}

	// Case 3: old exists, new is nil → full unmount
	if oldNode != nil && newNode == nil {
		r.unmountFromDOM(oldNode)
		return
	}

	// Case 4: different type or different tag → replace entirely
	if oldNode.Type != newNode.Type ||
		(oldNode.Type == VNodeElement && oldNode.Tag != newNode.Tag) ||
		oldNode.Type == VNodeComponent {
		r.unmountFromDOM(oldNode)
		r.mountToDOM(newNode)
		r.patchQueue = append(r.patchQueue, PatchOp{
			Type:   patchReplace,
			node:   newNode,
			parent: oldNode,
		})
		return
	}

	// Case 5: text nodes → update text content
	if newNode.Type == VNodeText {
		if oldNode.Text != newNode.Text {
			r.patchQueue = append(r.patchQueue, PatchOp{
				Type: patchSetText,
				node: oldNode,
				text: newNode.Text,
			})
		}
		// Share DOM element reference
		newNode.domElement = oldNode.domElement
		return
	}

	// Case 6: same element type, same tag → diff props + children
	if newNode.Type == VNodeElement {
		// Share DOM element
		newNode.domElement = oldNode.domElement

		// React-style bail-out: if key, props, and styles are identical,
		// skip prop/style diffing entirely. Children are still reconciled
		// because they may contain subtree changes (text, nested props, etc.).
		propsChanged := oldNode.Key != newNode.Key ||
			!shallowEqualStringMap(oldNode.Props, newNode.Props) ||
			!shallowEqualStringMap(oldNode.Style, newNode.Style)

		if propsChanged {
			r.diffProps(oldNode, newNode)
			r.diffStyles(oldNode, newNode)
		}

		// Always recurse into children — per-node bail-out is at each child level
		r.diffChildrenKeyed(oldNode, newNode)
		return
	}
}

// shallowEqualStringMap compares two string→string maps with shallow equals.
func shallowEqualStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// diffProps computes property-level patches
func (r *Reconciler) diffProps(oldNode, newNode *VNode) {
	// Remove props that no longer exist
	for key := range oldNode.Props {
		if key == "key" {
			continue
		}
		if _, exists := newNode.Props[key]; !exists {
			r.patchQueue = append(r.patchQueue, PatchOp{
				Type:    patchRemoveProp,
				node:    newNode,
				propKey: key,
			})
		}
	}

	// Add or update props
	for key, val := range newNode.Props {
		if key == "key" {
			continue
		}
		if oldVal, exists := oldNode.Props[key]; !exists || oldVal != val {
			r.patchQueue = append(r.patchQueue, PatchOp{
				Type:    patchSetProp,
				node:    newNode,
				propKey: key,
				propVal: val,
			})
		}
	}
}

// diffStyles computes style-level patches
func (r *Reconciler) diffStyles(oldNode, newNode *VNode) {
	for key := range oldNode.Style {
		if _, exists := newNode.Style[key]; !exists {
			r.patchQueue = append(r.patchQueue, PatchOp{
				Type:     patchRemoveProp,
				node:     newNode,
				propKey:  "style",
				styleKey: key,
			})
		}
	}
	for key, val := range newNode.Style {
		if oldVal, exists := oldNode.Style[key]; !exists || oldVal != val {
			r.patchQueue = append(r.patchQueue, PatchOp{
				Type:     patchSetStyle,
				node:     newNode,
				styleKey: key,
				styleVal: val,
			})
		}
	}
}

// diffChildrenKeyed reconciles child lists using React-style keyed diffing.
// Algorithm: O(n) two-pass approach with key maps for efficient matching.
func (r *Reconciler) diffChildrenKeyed(oldParent, newParent *VNode) {
	oldChildren := oldParent.Children
	newChildren := newParent.Children
	oldLen := len(oldChildren)
	newLen := len(newChildren)

	if oldLen == 0 && newLen == 0 {
		return
	}

	// Phase 1: Build key maps for old children
	oldKeyMap := make(map[string]int) // key → index
	oldFree := make([]bool, oldLen)   // which old children are not keyed
	for i, child := range oldChildren {
		if child != nil && child.Key != "" {
			oldKeyMap[child.Key] = i
		} else {
			oldFree[i] = true
		}
	}

	// Phase 2: Traverse new children, find matches
	// matchedOld[i] = true if oldChildren[i] was matched (to be kept or moved)
	matchedOld := make([]bool, oldLen)
	// newToOld[i] = j if newChildren[i] matches oldChildren[j]
	newToOld := make([]int, newLen)
	for i := range newToOld {
		newToOld[i] = -1
	}

	lastPlacedIndex := 0

	for newIdx := 0; newIdx < newLen; newIdx++ {
		newChild := newChildren[newIdx]
		if newChild == nil {
			continue
		}

		if newChild.Key != "" {
			// Keyed child: find matching old child by key
			if oldIdx, ok := oldKeyMap[newChild.Key]; ok {
				newToOld[newIdx] = oldIdx
				matchedOld[oldIdx] = true

				// Check if this is a move (index < lastPlacedIndex)
				if oldIdx < lastPlacedIndex {
					// Move needed
					r.patchQueue = append(r.patchQueue, PatchOp{
						Type:    patchMove,
						node:    newChild,
						parent:  oldParent,
						index:   newIdx,
						refNode: oldChildren[oldIdx],
					})
				} else {
					lastPlacedIndex = oldIdx
				}

				// Diff the subtree
				oldChild := oldChildren[oldIdx]
				newChild.domElement = oldChild.domElement
				r.diff(oldChild, newChild)
			}
		} else {
			// Unkeyed child: match by position with first unmatched unkeyed old child
			for oldIdx := 0; oldIdx < oldLen; oldIdx++ {
				if oldFree[oldIdx] && !matchedOld[oldIdx] {
					newToOld[newIdx] = oldIdx
					matchedOld[oldIdx] = true
					oldChild := oldChildren[oldIdx]
					newChild.domElement = oldChild.domElement
					r.diff(oldChild, newChild)
					break
				}
			}
		}
	}

	// Phase 3: Remove old children that were not matched.
	// Use the child's actual DOM node reference, not its index,
	// because prior moves in the patch queue shift DOM positions.
	for oldIdx := 0; oldIdx < oldLen; oldIdx++ {
		if !matchedOld[oldIdx] {
			oldChild := oldChildren[oldIdx]
			r.patchQueue = append(r.patchQueue, PatchOp{
				Type:   patchRemove,
				node:   oldChild,
				parent: oldParent,
			})
			r.unmountFromDOM(oldChild)
		}
	}

	// Phase 4: Insert new children that didn't match any old child
	for newIdx := 0; newIdx < newLen; newIdx++ {
		if newToOld[newIdx] == -1 {
			newChild := newChildren[newIdx]
			r.mountToDOM(newChild)
			r.patchQueue = append(r.patchQueue, PatchOp{
				Type:   patchInsert,
				node:   newChild,
				parent: newParent,
				index:  newIdx,
			})
		}
	}

	// Update parent reference
	newParent.Children = newChildren
}

// mountToDOM creates real DOM elements from a VNode tree (recursive)
func (r *Reconciler) mountToDOM(v *VNode) {
	if v == nil {
		return
	}

	switch v.Type {
	case VNodeText:
		v.domElement = CreateTextNode(v.Text)

	case VNodeElement:
		v.domElement = CreateElement(v.Tag)
		for key, val := range v.Props {
			if key == "key" {
				continue
			}
			if key == "className" {
				v.domElement.Call("setAttribute", "class", val)
			} else if key == "htmlFor" {
				v.domElement.Call("setAttribute", "for", val)
			} else if len(key) > 2 && key[0] == 'o' && key[1] == 'n' {
				// onClick, onChange, etc. → event delegation handled at mount
				continue
			} else {
				v.domElement.Call("setAttribute", key, val)
			}
		}
		// Apply inline styles
		for key, val := range v.Style {
			v.domElement.Get("style").Set(key, val)
		}
		// Mount children
		for _, child := range v.Children {
			r.mountToDOM(child)
			v.domElement.Call("appendChild", child.domElement)
		}

	case VNodeComponent:
		v.compInstance = mountComponent(v.Component, v.CompProps, v.Children)
		v.domElement = v.compInstance.vnode.domElement
	}
}

// unmountFromDOM cleans up a VNode tree
func (r *Reconciler) unmountFromDOM(v *VNode) {
	if v == nil {
		return
	}
	if v.Type == VNodeComponent && v.compInstance != nil {
		if v.compInstance.onUnmount != nil {
			v.compInstance.onUnmount()
		}
	}
	for _, child := range v.Children {
		r.unmountFromDOM(child)
	}
}

// apply executes a patch operation on the real DOM
func (r *Reconciler) apply(op PatchOp, root js.Value) {
	if op.node == nil || op.node.domElement.IsNull() || op.node.domElement.IsUndefined() {
		// For insert operations, node might not have domElement yet but we mounted it
		if op.Type != patchInsert {
			return
		}
	}

	switch op.Type {
	case patchSetText:
		op.node.domElement.Set("textContent", op.text)

	case patchSetProp:
		if op.propKey == "className" {
			op.node.domElement.Call("setAttribute", "class", op.propVal)
		} else if op.propKey == "htmlFor" {
			op.node.domElement.Call("setAttribute", "for", op.propVal)
		} else {
			op.node.domElement.Call("setAttribute", op.propKey, op.propVal)
		}

	case patchRemoveProp:
		if op.styleKey != "" {
			op.node.domElement.Get("style").Set(op.styleKey, "")
		} else {
			op.node.domElement.Call("removeAttribute", op.propKey)
		}

	case patchSetStyle:
		op.node.domElement.Get("style").Set(op.styleKey, op.styleVal)

	case patchReplace:
		if op.parent != nil && !op.parent.domElement.IsNull() {
			parent := op.parent.domElement
			parent.Set("innerHTML", "")
			if !op.node.domElement.IsNull() {
				parent.Call("appendChild", op.node.domElement)
			}
		}

	case patchRemove:
		if op.parent != nil && !op.parent.domElement.IsNull() {
			parent := op.parent.domElement
			if op.node != nil && !op.node.domElement.IsNull() && !op.node.domElement.IsUndefined() {
				parent.Call("removeChild", op.node.domElement)
			}
		}

	case patchInsert:
		if op.parent != nil && !op.parent.domElement.IsNull() {
			parent := op.parent.domElement
			childNodes := parent.Get("childNodes")
			if !op.node.domElement.IsNull() && !op.node.domElement.IsUndefined() {
				ref := childNodes.Call("item", op.index)
				parent.Call("insertBefore", op.node.domElement, ref)
			} else {
				parent.Call("appendChild", op.node.domElement)
			}
		}

	case patchMove:
		if op.parent != nil && !op.parent.domElement.IsNull() {
			parent := op.parent.domElement
			if op.refNode != nil && !op.refNode.domElement.IsNull() {
				// Remove from current position
				parent.Call("removeChild", op.refNode.domElement)
			}
			// Insert at new position
			childNodes := parent.Get("childNodes")
			if !op.node.domElement.IsNull() && !op.node.domElement.IsUndefined() {
				ref := childNodes.Call("item", op.index)
				parent.Call("insertBefore", op.node.domElement, ref)
			} else {
				parent.Call("appendChild", op.node.domElement)
			}
		}
	}
}

// childAt safely gets a child at index from a slice
func childAt(children []*VNode, index int) *VNode {
	if index < 0 || index >= len(children) {
		return nil
	}
	return children[index]
}
