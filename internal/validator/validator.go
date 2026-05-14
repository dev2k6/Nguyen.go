package validator

import (
	"fmt"
	"net/mail"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type RuleFunc func(field string, value interface{}, param string) string

type ValidationErrors map[string][]string

func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}

func (v ValidationErrors) First(field string) string {
	if errs, ok := v[field]; ok && len(errs) > 0 {
		return errs[0]
	}
	return ""
}

type Validator struct {
	rules map[string]RuleFunc
}

func New() *Validator {
	v := &Validator{
		rules: make(map[string]RuleFunc),
	}
	v.registerDefaults()
	return v
}

func (v *Validator) RegisterRule(name string, fn RuleFunc) {
	v.rules[name] = fn
}

func (v *Validator) Validate(data interface{}) ValidationErrors {
	errors := make(ValidationErrors)
	val := reflect.ValueOf(data)

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return errors
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("validate")
		if tag == "" || tag == "-" {
			continue
		}

		fieldName := field.Tag.Get("json")
		if fieldName == "" {
			fieldName = strings.ToLower(field.Name)
		}
		fieldName = strings.Split(fieldName, ",")[0]

		fieldVal := val.Field(i).Interface()
		fieldErrors := v.validateField(fieldName, fieldVal, tag)
		if len(fieldErrors) > 0 {
			errors[fieldName] = fieldErrors
		}
	}

	return errors
}

func (v *Validator) ValidateMap(data map[string]interface{}, rules map[string]string) ValidationErrors {
	errors := make(ValidationErrors)

	for field, ruleStr := range rules {
		val := data[field]
		fieldErrors := v.validateField(field, val, ruleStr)
		if len(fieldErrors) > 0 {
			errors[field] = fieldErrors
		}
	}

	return errors
}

func (v *Validator) validateField(field string, value interface{}, ruleStr string) []string {
	var errors []string
	rules := strings.Split(ruleStr, "|")

	for _, rule := range rules {
		parts := strings.SplitN(rule, ":", 2)
		ruleName := parts[0]
		param := ""
		if len(parts) == 2 {
			param = parts[1]
		}

		fn, ok := v.rules[ruleName]
		if !ok {
			continue
		}

		if msg := fn(field, value, param); msg != "" {
			errors = append(errors, msg)
		}
	}

	return errors
}

func (v *Validator) registerDefaults() {
	v.rules["required"] = ruleRequired
	v.rules["email"] = ruleEmail
	v.rules["min"] = ruleMin
	v.rules["max"] = ruleMax
	v.rules["regex"] = ruleRegex
	v.rules["numeric"] = ruleNumeric
	v.rules["alpha"] = ruleAlpha
	v.rules["alphanum"] = ruleAlphaNum
	v.rules["url"] = ruleURL
	v.rules["in"] = ruleIn
	v.rules["notin"] = ruleNotIn
	v.rules["len"] = ruleLen
	v.rules["between"] = ruleBetween
	v.rules["confirmed"] = ruleConfirmed
}

func ruleRequired(field string, value interface{}, _ string) string {
	if value == nil {
		return fmt.Sprintf("%s is required", field)
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		if strings.TrimSpace(v.String()) == "" {
			return fmt.Sprintf("%s is required", field)
		}
	case reflect.Slice, reflect.Map, reflect.Array:
		if v.Len() == 0 {
			return fmt.Sprintf("%s is required", field)
		}
	case reflect.Ptr, reflect.Interface:
		if v.IsNil() {
			return fmt.Sprintf("%s is required", field)
		}
	}
	return ""
}

func ruleEmail(field string, value interface{}, _ string) string {
	str, ok := toString(value)
	if !ok || str == "" {
		return ""
	}
	if _, err := mail.ParseAddress(str); err != nil {
		return fmt.Sprintf("%s must be a valid email", field)
	}
	return ""
}

func ruleMin(field string, value interface{}, param string) string {
	min, err := strconv.Atoi(param)
	if err != nil {
		return ""
	}

	str, ok := toString(value)
	if ok {
		if utf8.RuneCountInString(str) < min {
			return fmt.Sprintf("%s must be at least %d characters", field, min)
		}
		return ""
	}

	num, ok := toFloat(value)
	if ok && num < float64(min) {
		return fmt.Sprintf("%s must be at least %d", field, min)
	}
	return ""
}

func ruleMax(field string, value interface{}, param string) string {
	max, err := strconv.Atoi(param)
	if err != nil {
		return ""
	}

	str, ok := toString(value)
	if ok {
		if utf8.RuneCountInString(str) > max {
			return fmt.Sprintf("%s must be at most %d characters", field, max)
		}
		return ""
	}

	num, ok := toFloat(value)
	if ok && num > float64(max) {
		return fmt.Sprintf("%s must be at most %d", field, max)
	}
	return ""
}

func ruleRegex(field string, value interface{}, param string) string {
	str, ok := toString(value)
	if !ok || str == "" {
		return ""
	}
	re, err := regexp.Compile(param)
	if err != nil {
		return ""
	}
	if !re.MatchString(str) {
		return fmt.Sprintf("%s format is invalid", field)
	}
	return ""
}

func ruleNumeric(field string, value interface{}, _ string) string {
	str, ok := toString(value)
	if !ok || str == "" {
		return ""
	}
	if _, err := strconv.ParseFloat(str, 64); err != nil {
		return fmt.Sprintf("%s must be numeric", field)
	}
	return ""
}

func ruleAlpha(field string, value interface{}, _ string) string {
	str, ok := toString(value)
	if !ok || str == "" {
		return ""
	}
	re := regexp.MustCompile(`^[a-zA-Z]+$`)
	if !re.MatchString(str) {
		return fmt.Sprintf("%s must contain only letters", field)
	}
	return ""
}

func ruleAlphaNum(field string, value interface{}, _ string) string {
	str, ok := toString(value)
	if !ok || str == "" {
		return ""
	}
	re := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	if !re.MatchString(str) {
		return fmt.Sprintf("%s must contain only letters and numbers", field)
	}
	return ""
}

func ruleURL(field string, value interface{}, _ string) string {
	str, ok := toString(value)
	if !ok || str == "" {
		return ""
	}
	re := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	if !re.MatchString(str) {
		return fmt.Sprintf("%s must be a valid URL", field)
	}
	return ""
}

func ruleIn(field string, value interface{}, param string) string {
	str, ok := toString(value)
	if !ok || str == "" {
		return ""
	}
	allowed := strings.Split(param, ",")
	for _, a := range allowed {
		if str == a {
			return ""
		}
	}
	return fmt.Sprintf("%s must be one of: %s", field, param)
}

func ruleNotIn(field string, value interface{}, param string) string {
	str, ok := toString(value)
	if !ok || str == "" {
		return ""
	}
	disallowed := strings.Split(param, ",")
	for _, d := range disallowed {
		if str == d {
			return fmt.Sprintf("%s must not be one of: %s", field, param)
		}
	}
	return ""
}

func ruleLen(field string, value interface{}, param string) string {
	length, err := strconv.Atoi(param)
	if err != nil {
		return ""
	}
	str, ok := toString(value)
	if ok && utf8.RuneCountInString(str) != length {
		return fmt.Sprintf("%s must be exactly %d characters", field, length)
	}
	return ""
}

func ruleBetween(field string, value interface{}, param string) string {
	parts := strings.Split(param, ",")
	if len(parts) != 2 {
		return ""
	}
	min, err1 := strconv.Atoi(parts[0])
	max, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return ""
	}

	num, ok := toFloat(value)
	if ok {
		if num < float64(min) || num > float64(max) {
			return fmt.Sprintf("%s must be between %d and %d", field, min, max)
		}
	}
	return ""
}

func ruleConfirmed(_ string, _ interface{}, _ string) string {
	return ""
}

func toString(value interface{}) (string, bool) {
	if value == nil {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return v, true
	case fmt.Stringer:
		return v.String(), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func toFloat(value interface{}) (float64, bool) {
	if value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}
