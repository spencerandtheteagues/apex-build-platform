package mobile

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type apiEndpointDescriptor struct {
	Name            string
	FunctionName    string
	Method          string
	Path            string
	PathParams      []string
	RequestType     string
	ResponseType    string
	IsMultipart     bool
	RequiresPayload bool
	AuthRequired    bool
}

func apiEndpointDescriptors(contracts []MobileAPIContractSpec) []apiEndpointDescriptor {
	usedNames := map[string]int{}
	descriptors := make([]apiEndpointDescriptor, 0, len(contracts))
	for _, contract := range contracts {
		functionName := uniqueTSIdentifier(tsFunctionName(contract.Name), usedNames)
		method := strings.ToUpper(strings.TrimSpace(contract.Method))
		if method == "" {
			method = "GET"
		}
		path := strings.TrimSpace(contract.Path)
		if path == "" {
			path = "/api/" + functionName
		}
		responseType := tsTypeExpression(contract.Response)
		if responseType == "" {
			responseType = "unknown"
		}
		requestType := tsTypeExpression(contract.Request)
		isMultipart := strings.EqualFold(strings.TrimSpace(contract.Request), "multipart/form-data")
		requiresPayload := method != "GET" && method != "DELETE" && requestType != "" && !isMultipart
		authRequired := !strings.Contains(strings.ToLower(contract.Name+" "+contract.Path), "login")
		descriptors = append(descriptors, apiEndpointDescriptor{
			Name:            strings.TrimSpace(contract.Name),
			FunctionName:    functionName,
			Method:          method,
			Path:            path,
			PathParams:      pathParameterKeys(path),
			RequestType:     requestType,
			ResponseType:    responseType,
			IsMultipart:     isMultipart,
			RequiresPayload: requiresPayload,
			AuthRequired:    authRequired,
		})
	}
	return descriptors
}

func APIContractManifestJSON(spec MobileAppSpec) string {
	manifest := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       spec.App.Name + " Mobile API Contract",
			"version":     manifestDefaultString(spec.Identity.Version, "1.0.0"),
			"description": "Generated from MobileAppSpec.APIContracts. This manifest is the source reference for backend routes and generated mobile endpoint helpers.",
		},
		"servers": []map[string]string{{
			"url":         "${EXPO_PUBLIC_API_BASE_URL}",
			"description": "Runtime API base URL from Expo app config.",
		}},
		"paths":      apiContractPaths(spec),
		"components": apiContractComponents(spec),
		"x-apex-mobile": map[string]any{
			"schema_version":          1,
			"generated_from":          "MobileAppSpec.APIContracts",
			"target_platform":         TargetPlatformMobileExpo,
			"frontend_framework":      spec.Architecture.FrontendFramework,
			"backend_mode":            spec.Architecture.BackendMode,
			"auth_mode":               spec.Architecture.AuthMode,
			"database_mode":           spec.Architecture.DatabaseMode,
			"base_url_env":            "EXPO_PUBLIC_API_BASE_URL",
			"token_storage":           "expo-secure-store:auth_token",
			"authorization_header":    "Authorization: Bearer <token>",
			"offline_fallback_policy": "Generated auth/jobs/estimate flows keep local fallback when backend calls fail.",
			"mobile_helpers":          apiContractHelperIndex(spec),
		},
	}
	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	return string(encoded) + "\n"
}

func APIContractMarkdown(spec MobileAppSpec) string {
	var b strings.Builder
	b.WriteString("# Mobile API Contract\n\n")
	b.WriteString("Generated from `MobileAppSpec.APIContracts`. Keep backend routes, generated mobile endpoint helpers, and release documentation aligned with `mobile/docs/api-contract.json`.\n\n")
	b.WriteString("Base URL: `EXPO_PUBLIC_API_BASE_URL`\n\n")
	b.WriteString("| Helper | Method | Path | Auth | Request | Response |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, endpoint := range apiEndpointDescriptors(spec.APIContracts) {
		auth := "yes"
		if !endpoint.AuthRequired {
			auth = "no"
		}
		request := endpoint.RequestType
		if endpoint.IsMultipart {
			request = "FormData"
		}
		if request == "" {
			request = "-"
		}
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | %s | `%s` | `%s` |\n", endpoint.FunctionName, endpoint.Method, endpoint.Path, auth, request, endpoint.ResponseType))
	}
	if len(spec.APIContracts) == 0 {
		b.WriteString("| `healthCheck` | `GET` | `/api/health` | yes | - | `{ ok: boolean }` |\n")
	}
	b.WriteString("\nAuthentication tokens are stored with Expo SecureStore under `auth_token`. Browser-only storage APIs are intentionally not used.\n")
	return b.String()
}

func apiContractPaths(spec MobileAppSpec) map[string]any {
	paths := map[string]any{}
	for _, endpoint := range apiEndpointDescriptors(spec.APIContracts) {
		methodKey := strings.ToLower(endpoint.Method)
		operation := map[string]any{
			"operationId":     endpoint.FunctionName,
			"summary":         manifestDefaultString(endpoint.Name, endpoint.FunctionName),
			"x-mobile-helper": endpoint.FunctionName,
			"x-request-type":  manifestDefaultString(endpoint.RequestType, "none"),
			"x-response-type": endpoint.ResponseType,
			"x-path-params":   endpoint.PathParams,
			"responses": map[string]any{
				"200": map[string]any{
					"description":       "Successful response",
					"x-typescript-type": endpoint.ResponseType,
				},
			},
		}
		if endpoint.AuthRequired {
			operation["security"] = []map[string][]string{{"bearerAuth": {}}}
		} else {
			operation["security"] = []any{}
		}
		operation["x-mobile-path"] = endpoint.Path
		if len(endpoint.PathParams) > 0 {
			params := make([]map[string]any, 0, len(endpoint.PathParams))
			for _, param := range endpoint.PathParams {
				params = append(params, map[string]any{
					"name":     param,
					"in":       "path",
					"required": true,
					"schema":   map[string]string{"type": "string"},
				})
			}
			operation["parameters"] = params
		}
		if endpoint.IsMultipart {
			operation["requestBody"] = map[string]any{
				"required": true,
				"content":  map[string]any{"multipart/form-data": map[string]any{"schema": map[string]string{"type": "object"}}},
			}
		} else if endpoint.RequiresPayload {
			operation["requestBody"] = map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema":            schemaRef(endpoint.RequestType),
						"x-typescript-type": endpoint.RequestType,
					},
				},
			}
		}
		manifestPath := openAPIPath(endpoint.Path)
		operations, _ := paths[manifestPath].(map[string]any)
		if operations == nil {
			operations = map[string]any{}
			paths[manifestPath] = operations
		}
		operations[methodKey] = operation
	}
	if len(spec.APIContracts) == 0 {
		paths["/api/health"] = map[string]any{
			"get": map[string]any{
				"operationId":     "healthCheck",
				"summary":         "Health check",
				"x-mobile-helper": "healthCheck",
				"x-response-type": "{ ok: boolean }",
				"responses": map[string]any{"200": map[string]any{
					"description":       "Successful response",
					"x-typescript-type": "{ ok: boolean }",
				}},
			},
		}
	}
	return paths
}

func openAPIPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func apiContractComponents(spec MobileAppSpec) map[string]any {
	schemas := map[string]any{
		"AuthSession": map[string]any{
			"type":     "object",
			"required": []string{"token", "user"},
			"properties": map[string]any{
				"token": map[string]string{"type": "string"},
				"user":  map[string]string{"type": "object"},
			},
		},
		"LoginRequest": map[string]any{
			"type":     "object",
			"required": []string{"email", "password"},
			"properties": map[string]any{
				"email":    map[string]string{"type": "string"},
				"password": map[string]string{"type": "string"},
			},
		},
	}
	for _, model := range spec.DataModels {
		typeName := exportedTypeName(model.Name)
		if typeName == "" {
			continue
		}
		properties := map[string]any{}
		keys := make([]string, 0, len(model.Fields))
		for key := range model.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			properties[tsPropertyName(key)] = map[string]string{"type": openAPIPrimitiveType(model.Fields[key])}
		}
		schemas[typeName] = map[string]any{
			"type":       "object",
			"properties": properties,
		}
	}
	for _, endpoint := range apiEndpointDescriptors(spec.APIContracts) {
		for _, typeName := range []string{endpoint.RequestType, endpoint.ResponseType} {
			typeName = strings.TrimSuffix(typeName, "[]")
			if typeName == "" || strings.Contains(typeName, "{") {
				continue
			}
			if _, ok := schemas[typeName]; !ok && !isPrimitiveTSType(typeName) {
				schemas[typeName] = map[string]any{"type": "object"}
			}
		}
	}
	return map[string]any{
		"securitySchemes": map[string]any{
			"bearerAuth": map[string]string{"type": "http", "scheme": "bearer"},
		},
		"schemas": schemas,
	}
}

func apiContractHelperIndex(spec MobileAppSpec) []map[string]any {
	helpers := []map[string]any{}
	for _, endpoint := range apiEndpointDescriptors(spec.APIContracts) {
		bodyMode := "none"
		if endpoint.IsMultipart {
			bodyMode = "multipart"
		} else if endpoint.RequiresPayload {
			bodyMode = "json"
		}
		helpers = append(helpers, map[string]any{
			"name":          endpoint.Name,
			"helper":        endpoint.FunctionName,
			"method":        endpoint.Method,
			"path":          endpoint.Path,
			"auth_required": endpoint.AuthRequired,
			"request_type":  manifestDefaultString(endpoint.RequestType, "none"),
			"response_type": endpoint.ResponseType,
			"body_mode":     bodyMode,
			"path_params":   endpoint.PathParams,
		})
	}
	return helpers
}

func schemaRef(typeName string) map[string]any {
	typeName = strings.TrimSuffix(strings.TrimSpace(typeName), "[]")
	if typeName == "" || isPrimitiveTSType(typeName) {
		return map[string]any{"type": openAPIPrimitiveType(typeName)}
	}
	return map[string]any{"$ref": "#/components/schemas/" + typeName}
}

func openAPIPrimitiveType(typeName string) string {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "number", "integer", "int", "float", "decimal":
		return "number"
	case "boolean":
		return "boolean"
	case "object", "record":
		return "object"
	case "array":
		return "array"
	default:
		return "string"
	}
}

func manifestDefaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
