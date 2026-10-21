// Package queryutil parses tagged list-request structs and applies their
// search, filters, sort, preload, and pagination to GORM queries.
package queryutil

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/boilerplate/microservice-boilerplate/pkg/apperror"
	"gorm.io/gorm"
)

var ErrInvalidQuery = &apperror.Error{Code: "invalid_query", Message: "invalid list query"}

type Filter struct {
	Column string
	Op     string
	Value  any
}

type ListParams struct {
	Search        string
	SearchColumns []string
	Filters       []Filter
	SortBy        string
	SortDir       string
	Preload       []string
	Offset        int
	Limit         int
}

// Bind fills dst from URL values using query tags. Invalid optional scalar
// values stay unset. Slices preserve repeated values; add ,list to split one
// comma-separated value (used by preload).
func Bind(values url.Values, dst any) error {
	v, err := structValue(dst)
	if err != nil {
		return err
	}
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		key, opts := queryTag(field)
		if key == "" || key == "-" {
			continue
		}
		raw := values.Get(key)
		if raw == "" {
			raw = opts["default"]
		}
		if raw == "" {
			continue
		}
		if v.Field(i).Kind() == reflect.Slice {
			items := values[key]
			if opts["list"] == "true" {
				items = split(raw)
			}
			v.Field(i).Set(reflect.ValueOf(items))
			continue
		}
		set, err := setValue(v.Field(i), raw, opts)
		if err != nil {
			return fmt.Errorf("query field %s: %w", key, err)
		}
		if !set {
			continue
		}
	}
	return nil
}

// BuildListParams derives safe GORM options from compile-time request tags:
// search:"name,description", sort:"name,price", preload:"Category", and
// filter:"name:string:eq,price:float64:eq|gte|lte". Never derive SQL
// identifiers from entity fields or raw URL values.
func BuildListParams(src any) (ListParams, error) {
	v, err := structValue(src)
	if err != nil {
		return ListParams{}, err
	}

	p := ListParams{SortDir: "asc", Limit: 20}
	page := 1
	for i := 0; i < v.NumField(); i++ {
		field, value := v.Type().Field(i), v.Field(i)
		key, _ := queryTag(field)
		switch {
		case field.Tag.Get("search") != "":
			p.Search = value.String()
			p.SearchColumns = split(field.Tag.Get("search"))
		case field.Tag.Get("filter") != "":
			filters, err := parseFilters(value.Interface().([]string), field.Tag.Get("filter"))
			if err != nil {
				return ListParams{}, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
			}
			p.Filters = filters
		case field.Tag.Get("sort") != "":
			sortBy, sortDir, err := parseSort(value.String(), split(field.Tag.Get("sort")))
			if err != nil {
				return ListParams{}, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
			}
			p.SortBy, p.SortDir = sortBy, sortDir
		case field.Tag.Get("preload") != "":
			p.Preload = value.Interface().([]string)
			for _, relation := range p.Preload {
				if !contains(split(field.Tag.Get("preload")), relation) {
					return ListParams{}, fmt.Errorf("%w: invalid preload %q", ErrInvalidQuery, relation)
				}
			}
		case key == "page":
			page = intValue(value)
		case key == "page_size":
			if limit := intValue(value); limit > 0 {
				p.Limit = limit
			}
		}
	}
	p.Offset = (page - 1) * p.Limit
	return p, nil
}

func ApplySearchAndFilters(query *gorm.DB, p ListParams) *gorm.DB {
	if p.Search != "" && len(p.SearchColumns) > 0 {
		like := "%" + strings.ToLower(p.Search) + "%"
		conditions := make([]string, len(p.SearchColumns))
		args := make([]any, len(p.SearchColumns))
		for i, column := range p.SearchColumns {
			conditions[i] = "LOWER(" + column + ") LIKE ?"
			args[i] = like
		}
		query = query.Where(strings.Join(conditions, " OR "), args...)
	}
	for _, filter := range p.Filters {
		query = query.Where(filter.Column+" "+filter.Op+" ?", filter.Value)
	}
	return query
}

func ApplySortPreloadPage(query *gorm.DB, p ListParams) *gorm.DB {
	if p.SortBy != "" {
		query = query.Order(p.SortBy + " " + p.SortDir)
	}
	for _, relation := range p.Preload {
		query = query.Preload(relation)
	}
	return query.Offset(p.Offset).Limit(p.Limit)
}

func parseFilters(values []string, tag string) ([]Filter, error) {
	specs := make(map[string]filterSpec)
	for _, raw := range split(tag) {
		parts := strings.Split(raw, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid filter tag %q", raw)
		}
		specs[parts[0]] = filterSpec{column: parts[0], kind: parts[1], ops: splitOps(parts[2])}
	}
	filters := make([]Filter, 0, len(values))
	for _, raw := range values {
		parts := strings.SplitN(raw, ":", 3)
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return nil, fmt.Errorf("invalid filter %q; expected field:operator:value", raw)
		}
		spec, ok := specs[parts[0]]
		if !ok || !contains(spec.ops, parts[1]) {
			return nil, fmt.Errorf("invalid filter %q", raw)
		}
		value, err := parseFilterValue(parts[2], spec.kind)
		if err != nil {
			return nil, fmt.Errorf("invalid filter %q: %w", raw, err)
		}
		filters = append(filters, Filter{Column: spec.column, Op: sqlOperator(parts[1]), Value: value})
	}
	return filters, nil
}

type filterSpec struct {
	column string
	kind   string
	ops    []string
}

func parseSort(raw string, allowed []string) (string, string, error) {
	if raw == "" {
		return "", "asc", nil
	}
	parts := strings.Split(raw, ":")
	if len(parts) != 2 || !contains(allowed, parts[0]) || (parts[1] != "asc" && parts[1] != "desc") {
		return "", "", fmt.Errorf("invalid sort %q; expected field:asc or field:desc", raw)
	}
	return parts[0], parts[1], nil
}

func parseFilterValue(raw, kind string) (any, error) {
	switch kind {
	case "string":
		return raw, nil
	case "int":
		value, err := strconv.Atoi(raw)
		if err != nil {
			return nil, err
		}
		return value, nil
	case "float64":
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, err
		}
		return value, nil
	case "bool":
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, err
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported filter type %q", kind)
	}
}

func sqlOperator(operator string) string {
	return map[string]string{"eq": "=", "ne": "!=", "gt": ">", "gte": ">=", "lt": "<", "lte": "<=", "like": "LIKE"}[operator]
}

func splitOps(raw string) []string { return strings.Split(raw, "|") }

func queryTag(field reflect.StructField) (string, map[string]string) {
	parts := strings.Split(field.Tag.Get("query"), ",")
	opts := make(map[string]string, len(parts)-1)
	for _, part := range parts[1:] {
		key, value, found := strings.Cut(part, "=")
		if found {
			opts[key] = value
		} else {
			opts[key] = "true"
		}
	}
	return parts[0], opts
}

func setValue(field reflect.Value, raw string, opts map[string]string) (bool, error) {
	if field.Kind() == reflect.Pointer {
		value := reflect.New(field.Type().Elem())
		set, err := setValue(value.Elem(), raw, opts)
		if err != nil || !set {
			return set, err
		}
		field.Set(value)
		return true, nil
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Int, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return false, nil
		}
		if min, ok := opts["min"]; ok {
			minimum, _ := strconv.ParseInt(min, 10, 64)
			if value < minimum {
				value = minimum
			}
		}
		field.SetInt(value)
	default:
		return false, fmt.Errorf("unsupported type %s", field.Type())
	}
	return true, nil
}

func structValue(value any) (reflect.Value, error) {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}, fmt.Errorf("nil request")
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("request must be a struct")
	}
	return v, nil
}

func intValue(value reflect.Value) int {
	if value.Kind() == reflect.Int || value.Kind() == reflect.Int64 {
		return int(value.Int())
	}
	return 0
}

func split(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
