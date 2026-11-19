package schema

import (
	"fmt"
	"strconv"
	"strings"
)

func responseRef(operation OperationKey, status int) string {
	path := operation.Path
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, ":") {
			segments[i] = "{" + segment[1:] + "}"
		}
	}
	escape := strings.NewReplacer("~", "~0", "/", "~1")
	return strings.Join([]string{"#/paths", escape.Replace(strings.Join(segments, "/")), strings.ToLower(operation.Method),
		"responses", strconv.Itoa(status)}, "/")
}

func ResponseSchema(operation OperationKey, status int, mediaType string) (string, error) {
	media := strings.NewReplacer("~", "~0", "/", "~1").Replace(mediaType)
	ref := responseRef(operation, status) + "/content/" + media + "/schema"
	result, err := SpecSchema(ref, false)
	if err != nil {
		return "", fmt.Errorf("%s %s response %d %s: %w", operation.Method, operation.Path, status, mediaType, err)
	}
	return result, nil
}

func ResponseHasNoContent(operation OperationKey, status int) error {
	root, err := specRoot()
	if err != nil {
		return err
	}
	path := responseRef(operation, status)
	for range maxDepth {
		nodes, err := resolveAll(root, root, path, 0)
		if err != nil {
			return err
		}
		if ref, ok := nodes[0]["$ref"].(string); ok {
			path = ref
			continue
		}
		if _, hasContent := nodes[0]["content"]; hasContent {
			return fmt.Errorf("%s %s response %d declares content", operation.Method, operation.Path, status)
		}
		return nil
	}
	return fmt.Errorf("%s %s response %d: too many references", operation.Method, operation.Path, status)
}
