//go:build js && wasm

package core

import (
	"fmt"
	"strconv"
	"syscall/js"
)

// FieldValidator defines validation rules for a single form field.
type FieldValidator struct {
	Required  bool
	MinLength int
	MaxLength int
	Pattern   string
	Message   string // custom error message
}

// Validate checks a string value against the validator rules.
func (f *FieldValidator) Validate(value string) string {
	if f.Required && value == "" {
		if f.Message != "" {
			return f.Message
		}
		return "This field is required"
	}
	if f.MinLength > 0 && len(value) < f.MinLength {
		return fmt.Sprintf("Must be at least %d characters", f.MinLength)
	}
	if f.MaxLength > 0 && len(value) > f.MaxLength {
		return fmt.Sprintf("Must be at most %d characters", f.MaxLength)
	}
	return ""
}

// FormState holds the current state of a form managed by UseForm.
type FormState struct {
	Values       map[string]interface{}
	Errors       map[string]string
	Touched      map[string]bool
	IsValid      bool
	IsSubmitting bool
	SetValue     func(name string, value interface{})
	Validate     func() bool
	Reset        func()
}

// UseForm manages form state with validation.
//
// Usage in .gox frontmatter:
//
//	validators := map[string]*core.FieldValidator{
//	    "email": {Required: true, MinLength: 3},
//	    "password": {Required: true, MinLength: 6},
//	}
//	form := core.UseForm(map[string]interface{}{"email": "", "password": ""}, validators)
//
// In template:
//
//	<input name="email" @input="onEmailChange" />
//	<div>{form.Errors["email"]}</div>
//
// The event handler must be declared in frontmatter:
//
//	func onEmailChange(e js.Value) {
//	    form.SetValue("email", e.Get("target").Get("value").String())
//	}
func UseForm(initialValues map[string]interface{}, validators map[string]*FieldValidator) *FormState {
	valuesRef := UseRef(copyMapInterface(initialValues))
	errorsRef := UseRef(make(map[string]string))
	touchedRef := UseRef(make(map[string]bool))
	validRef := UseRef(true)
	submittingRef := UseRef(false)

	_, setVersion := UseState(0)

	// Force re-render helper
	forceUpdate := func() {
		setVersion(func(v interface{}) interface{} {
			if n, ok := v.(int); ok {
				return n + 1
			}
			return 1
		})
	}

	setValue := func(name string, value interface{}) {
		m := valuesRef.Value.(*mapContainer)
		if m == nil {
			m = &mapContainer{m: make(map[string]interface{})}
			valuesRef.Value = m
		}
		m.m[name] = value

		// Mark touched
		t := touchedRef.Value.(*mapContainerBool)
		if t == nil {
			t = &mapContainerBool{m: make(map[string]bool)}
			touchedRef.Value = t
		}
		t.m[name] = true

		// Validate field
		e := errorsRef.Value.(*mapContainerStr)
		if e == nil {
			e = &mapContainerStr{m: make(map[string]string)}
			errorsRef.Value = e
		}
		if validator, ok := validators[name]; ok {
			if strVal, isStr := value.(string); isStr {
				if msg := validator.Validate(strVal); msg != "" {
					e.m[name] = msg
				} else {
					delete(e.m, name)
				}
			}
		}

		forceUpdate()
	}

	validateAll := func() bool {
		valid := true
		e := errorsRef.Value.(*mapContainerStr)
		if e == nil {
			e = &mapContainerStr{m: make(map[string]string)}
			errorsRef.Value = e
		}
		m := valuesRef.Value.(*mapContainer)
		if m == nil {
			m = &mapContainer{m: make(map[string]interface{})}
		}
		for name, validator := range validators {
			var val string
			if v, ok := m.m[name]; ok {
				if s, ok := v.(string); ok {
					val = s
				} else if v != nil {
					val = fmt.Sprintf("%v", v)
				}
			}
			if msg := validator.Validate(val); msg != "" {
				e.m[name] = msg
				valid = false
			} else {
				delete(e.m, name)
			}
		}
		validRef.Value = valid
		return valid
	}

	reset := func() {
		valuesRef.Value = &mapContainer{m: copyMapInterface(initialValues)}
		errorsRef.Value = &mapContainerStr{m: make(map[string]string)}
		touchedRef.Value = &mapContainerBool{m: make(map[string]bool)}
		validRef.Value = true
		submittingRef.Value = false
		forceUpdate()
	}

	return &FormState{
		Values:       valuesRef.Value.(*mapContainer).m,
		Errors:       errorsRef.Value.(*mapContainerStr).m,
		Touched:      touchedRef.Value.(*mapContainerBool).m,
		IsValid:      validRef.Value.(bool),
		IsSubmitting: submittingRef.Value.(bool),
		SetValue:     setValue,
		Validate:     validateAll,
		Reset:        reset,
	}
}

// Helper types that satisfy interface{} without needing generics.
type mapContainer struct {
	m map[string]interface{}
}

type mapContainerStr struct {
	m map[string]string
}

type mapContainerBool struct {
	m map[string]bool
}

func copyMapInterface(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return make(map[string]interface{})
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// InputChangeHandler returns a js.Func suitable for @input or @change events.
// It reads e.target.name and e.target.value, then updates the form.
//
// Usage:
//
//	form := core.UseForm(...)
//	onInput := core.InputChangeHandler(form)
func InputChangeHandler(form *FormState) func(js.Value) {
	return func(e js.Value) {
		target := e.Get("target")
		name := target.Get("name").String()
		if name == "" {
			return
		}
		value := target.Get("value").String()
		form.SetValue(name, value)
	}
}

// InputCheckedHandler returns a js.Func for checkbox/radio @change events.
func InputCheckedHandler(form *FormState) func(js.Value) {
	return func(e js.Value) {
		target := e.Get("target")
		name := target.Get("name").String()
		if name == "" {
			return
		}
		checked := target.Get("checked").Bool()
		form.SetValue(name, strconv.FormatBool(checked))
	}
}

// SelectChangeHandler returns a js.Func for <select> @change events.
func SelectChangeHandler(form *FormState) func(js.Value) {
	return func(e js.Value) {
		target := e.Get("target")
		name := target.Get("name").String()
		if name == "" {
			return
		}
		value := target.Get("value").String()
		form.SetValue(name, value)
	}
}

// SubmitHandler wraps a callback to run validation before submit.
//
// Usage:
//
//	func handleSubmit(e js.Value) {
//	    e.Call("preventDefault")
//	    core.SubmitHandler(form, func(values map[string]interface{}) {
//	        // send to server
//	    })(e)
//	}
func SubmitHandler(form *FormState, callback func(map[string]interface{})) func(js.Value) {
	return func(e js.Value) {
		_ = e.Call("preventDefault")
		if form.Validate() {
			callback(form.Values)
		}
	}
}
