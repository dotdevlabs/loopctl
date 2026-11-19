package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/mail"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type responseShape struct {
	Type                 string
	Format               string
	Pattern              string
	Enum                 []any
	Nullable             bool
	Minimum              *float64
	Maximum              *float64
	MinLength            int
	MaxLength            int
	MinItems             int
	MaxItems             int
	MinProperties        int
	Required             []string
	UniqueItems          bool
	Properties           map[string]*responseShape
	Items                *responseShape
	AdditionalProperties json.RawMessage
	OneOf                []*responseShape
	AnyOf                []*responseShape
	AllOf                []*responseShape
}

func (s *responseShape) children(segment string) []*responseShape {
	var children []*responseShape
	if segment == "*" && s.Items != nil {
		children = append(children, s.Items)
	}
	if property := s.Properties[segment]; property != nil {
		children = append(children, property)
	}
	for _, branches := range [][]*responseShape{s.OneOf, s.AnyOf, s.AllOf} {
		for _, branch := range branches {
			children = append(children, branch.children(segment)...)
		}
	}
	return children
}

func checkProjection(shape *responseShape, path string, target any) error {
	nodes := []*responseShape{shape}
	for _, segment := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if segment == "" {
			continue
		}
		var matching []*responseShape
		for _, node := range nodes {
			matching = append(matching, node.children(segment)...)
		}
		if len(matching) == 0 {
			return fmt.Errorf("undeclared response field %s", path)
		}
		nodes = matching
	}
	return checkProjectionType(nodes, path, reflect.TypeOf(target))
}

var rawJSONType = reflect.TypeFor[json.RawMessage]()

func checkProjectionType(nodes []*responseShape, path string, typ reflect.Type) error {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == rawJSONType {
		return nil
	}
	if typ.Kind() == reflect.Struct {
		for _, field := range reflect.VisibleFields(typ) {
			if !field.IsExported() || field.Anonymous {
				continue
			}
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			if err := checkProjectionChild(nodes, path, name, field.Type); err != nil {
				return err
			}
		}
	} else if typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		return checkProjectionChild(nodes, path, "*", typ.Elem())
	} else if typ.Kind() == reflect.Map {
		for _, node := range nodes {
			if len(node.AdditionalProperties) > 0 && string(node.AdditionalProperties) != "false" {
				return nil
			}
		}
		return fmt.Errorf("structured response object %s decoded into a map", path)
	}
	return nil
}

func checkProjectionChild(nodes []*responseShape, path, name string, typ reflect.Type) error {
	var children []*responseShape
	for _, node := range nodes {
		children = append(children, node.children(name)...)
	}
	if len(children) == 0 {
		return fmt.Errorf("undeclared response field %s/%s", path, name)
	}
	return checkProjectionType(children, path+"/"+name, typ)
}

func (s *responseShape) example(path string, overrides map[string]any) (any, error) {
	if value, ok := overrides[path]; ok {
		return value, nil
	}
	for _, branches := range [][]*responseShape{s.OneOf, s.AnyOf} {
		if len(branches) > 0 {
			return branches[0].example(path, overrides)
		}
	}
	if len(s.AllOf) > 0 {
		object := map[string]any{}
		for _, branch := range s.AllOf {
			value, err := branch.example(path, overrides)
			if err != nil {
				return nil, err
			}
			properties, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s: expected allOf object", path)
			}
			for key, property := range properties {
				object[key] = property
			}
		}
		return object, nil
	}
	if len(s.Enum) > 0 {
		return s.Enum[0], nil
	}
	switch s.Type {
	case "object", "":
		keys := make([]string, 0, len(s.Properties))
		for key := range s.Properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		object := map[string]any{}
		for _, key := range keys {
			value, err := s.Properties[key].example(path+"/"+key, overrides)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		if len(keys) == 0 && string(s.AdditionalProperties) == "true" {
			object["fixture"] = path
		}
		return object, nil
	case "array":
		if s.Items == nil {
			return nil, fmt.Errorf("%s: array has no item schema", path)
		}
		items := make([]any, max(1, s.MinItems))
		for i := range items {
			value, err := s.Items.example(path+"/"+strconv.Itoa(i), overrides)
			if err != nil {
				return nil, err
			}
			items[i] = value
		}
		return items, nil
	case "string":
		return s.exampleString(path)
	case "boolean":
		return true, nil
	case "integer", "number":
		value := float64(7)
		if s.Minimum != nil {
			value = max(value, *s.Minimum)
		}
		if s.Maximum != nil {
			value = min(value, *s.Maximum)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("%s: unsupported schema type %q", path, s.Type)
	}
}

func (s *responseShape) validate(value any, path string) error {
	if value == nil && s.Nullable {
		return nil
	}
	for _, branch := range s.AllOf {
		if err := branch.validate(value, path); err != nil {
			return err
		}
	}
	for _, branches := range [][]*responseShape{s.OneOf, s.AnyOf} {
		if len(branches) == 0 {
			continue
		}
		matches := 0
		var lastErr error
		for _, branch := range branches {
			if lastErr = branch.validate(value, path); lastErr == nil {
				matches++
			}
		}
		if matches == 0 {
			return fmt.Errorf("%s: fixture matches no schema alternative (%v)", path, lastErr)
		}
		if len(s.OneOf) > 0 && matches != 1 {
			return fmt.Errorf("%s: fixture matches %d alternatives of oneOf", path, matches)
		}
	}
	if len(s.Enum) > 0 && !slices.ContainsFunc(s.Enum, func(option any) bool { return reflect.DeepEqual(option, value) }) {
		return fmt.Errorf("%s: fixture value %v is outside the enum", path, value)
	}
	switch s.Type {
	case "":
		return nil
	case "object":
		object, ok := value.(map[string]any)
		if !ok || len(object) < s.MinProperties {
			return fmt.Errorf("%s: fixture must be an object with at least %d properties", path, s.MinProperties)
		}
		for _, key := range s.Required {
			if _, exists := object[key]; !exists {
				return fmt.Errorf("%s/%s: fixture is missing a required field", path, key)
			}
		}
		for key, property := range object {
			child := s.Properties[key]
			switch {
			case child != nil:
			case string(s.AdditionalProperties) == "false":
				return fmt.Errorf("%s/%s: fixture has an undeclared field", path, key)
			case len(s.AdditionalProperties) != 0 && string(s.AdditionalProperties) != "true":
				child = &responseShape{}
				if err := json.Unmarshal(s.AdditionalProperties, child); err != nil {
					return err
				}
			}
			if child != nil {
				if err := child.validate(property, path+"/"+key); err != nil {
					return err
				}
			}
		}
	case "array":
		items, ok := value.([]any)
		if !ok || len(items) < s.MinItems || (s.MaxItems > 0 && len(items) > s.MaxItems) {
			return fmt.Errorf("%s: invalid fixture array length or type", path)
		}
		for i, item := range items {
			if s.UniqueItems && slices.ContainsFunc(items[:i], func(previous any) bool { return reflect.DeepEqual(previous, item) }) {
				return fmt.Errorf("%s: repeated fixture array element", path)
			}
			if err := s.Items.validate(item, path+"/"+strconv.Itoa(i)); err != nil {
				return err
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok || utf8.RuneCountInString(text) < s.MinLength || (s.MaxLength > 0 && utf8.RuneCountInString(text) > s.MaxLength) {
			return fmt.Errorf("%s: invalid fixture string length or type", path)
		}
		if s.Pattern != "" {
			matches, err := regexp.MatchString(s.Pattern, text)
			if err != nil || !matches {
				return fmt.Errorf("%s: fixture string does not match %q", path, s.Pattern)
			}
		}
		return validateStringFormat(s.Format, text, path)
	case "integer", "number":
		number, ok := value.(float64)
		if !ok || (s.Type == "integer" && math.Trunc(number) != number) ||
			(s.Minimum != nil && number < *s.Minimum) || (s.Maximum != nil && number > *s.Maximum) {
			return fmt.Errorf("%s: fixture number %v violates %s constraints", path, value, s.Type)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: fixture must be a boolean", path)
		}
	default:
		return fmt.Errorf("%s: unsupported schema type %q", path, s.Type)
	}
	return nil
}

func validateStringFormat(format, value, path string) error {
	var err error
	switch format {
	case "", "binary":
	case "uuid":
		if matches, _ := regexp.MatchString(`^[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$`, value); !matches {
			err = fmt.Errorf("invalid UUID")
		}
	case "date-time":
		_, err = time.Parse(time.RFC3339, value)
	case "email":
		_, err = mail.ParseAddress(value)
	case "url", "uri":
		var parsed *url.URL
		parsed, err = url.Parse(value)
		if err == nil && !parsed.IsAbs() {
			err = fmt.Errorf("expected an absolute URL")
		}
	default:
		err = fmt.Errorf("unsupported string format %q", format)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (s *responseShape) exampleString(path string) (string, error) {
	if s.Pattern != "" {
		pattern, err := regexp.Compile(s.Pattern)
		if err != nil {
			return "", err
		}
		values := []string{"422", strings.Repeat("a", 32), strings.Repeat("a", 40), "stage.example", "example", "1"}
		if i := slices.IndexFunc(values, pattern.MatchString); i >= 0 {
			return values[i], nil
		}
		return "", fmt.Errorf("%s: no example for pattern %q", path, s.Pattern)
	}
	switch s.Format {
	case "uuid":
		return "12345678-1234-4abc-8abc-123456789012", nil
	case "date-time":
		return "2026-11-19T20:00:00Z", nil
	case "email":
		return "operator@example.test", nil
	case "uri", "url":
		return "https://example.test" + path, nil
	case "", "binary":
		text := path + strings.Repeat("x", max(0, s.MinLength-len(path)))
		if s.MaxLength > 0 && len(text) > s.MaxLength {
			text = text[len(text)-s.MaxLength:]
		}
		return text, nil
	default:
		return "", fmt.Errorf("%s: unsupported string format %q", path, s.Format)
	}
}
