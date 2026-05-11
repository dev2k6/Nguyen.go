//go:build js && wasm

package core

import "syscall/js"

// CreateElement creates a new DOM element with the given tag name
func CreateElement(tagName string) js.Value {
	return js.Global().Get("document").Call("createElement", tagName)
}

// CreateTextNode creates a text node
func CreateTextNode(content string) js.Value {
	return js.Global().Get("document").Call("createTextNode", content)
}

// SetAttr sets an attribute on an element
func SetAttr(el js.Value, name string, value string) {
	el.Call("setAttribute", name, value)
}

// AppendChild adds a child element to a parent
func AppendChild(parent, child js.Value) {
	parent.Call("appendChild", child)
}

// SetText sets the text content of an element
func SetText(el js.Value, text string) {
	el.Set("textContent", text)
}

// SetHTML sets the innerHTML of an element
func SetHTML(el js.Value, html string) {
	el.Set("innerHTML", html)
}

// GetElementById finds an element by its ID
func GetElementById(id string) js.Value {
	return js.Global().Get("document").Call("getElementById", id)
}

// BindEvent adds an event listener to an element
// eventName: "click", "input", "change", ...
// handler: the Go function to call when the event fires
func BindEvent(el js.Value, eventName string, handler func()) {
	listener := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		handler()
		return nil
	})
	el.Call("addEventListener", eventName, listener)
}

// ClearContent removes all child elements
func ClearContent(el js.Value) {
	el.Set("innerHTML", "")
}

// GetValue gets the value from an input element
func GetValue(el js.Value) string {
	return el.Get("value").String()
}

// SetValue sets the value of an input element
func SetValue(el js.Value, value string) {
	el.Set("value", value)
}

// AddClass adds a CSS class to an element
func AddClass(el js.Value, className string) {
	classList := el.Get("classList")
	classList.Call("add", className)
}

// RemoveClass removes a CSS class from an element
func RemoveClass(el js.Value, className string) {
	classList := el.Get("classList")
	classList.Call("remove", className)
}

// SetTitle sets the page title
func SetTitle(title string) {
	js.Global().Get("document").Set("title", title)
}
