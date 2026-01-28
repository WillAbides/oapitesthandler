package handlergen

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/util"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
	"gopkg.in/yaml.v3"
	"mvdan.cc/gofumpt/format"
)

//go:embed helpers/helpers.go
var helpersSource []byte

//go:embed templates
var templatesFS embed.FS

var templates = template.Must(template.ParseFS(templatesFS, "templates/*.tmpl"))

func Run(specPath, configPath, outputPath, modelsPkg string) error {
	rawOpts, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config file")
	}

	var opts codegen.Configuration
	err = yaml.Unmarshal(rawOpts, &opts)
	if err != nil {
		return fmt.Errorf("unmarshaling config file: %w", err)
	}
	opts.PackageName = detectPackageName(outputPath)

	spec, err := loadSpec(opts, specPath)
	if err != nil {
		return err
	}

	err = generateModels(spec, opts, outputPath, modelsPkg)
	if err != nil {
		return fmt.Errorf("generating models code: %w", err)
	}

	err = generateServer(spec, opts, outputPath)
	if err != nil {
		return fmt.Errorf("generating server code: %w", err)
	}

	err = generateTestHandler(spec, opts, outputPath)
	if err != nil {
		return fmt.Errorf("generating test handler: %w", err)
	}

	return nil
}

func loadSpec(opts codegen.Configuration, specPath string) (*openapi3.T, error) {
	overlayOpts := util.LoadSwaggerWithOverlayOpts{
		Path:   opts.OutputOptions.Overlay.Path,
		Strict: true,
	}
	if opts.OutputOptions.Overlay.Strict != nil {
		overlayOpts.Strict = *opts.OutputOptions.Overlay.Strict
	}
	swagger, err := util.LoadSwaggerWithOverlay(specPath, overlayOpts)
	if err != nil {
		return nil, fmt.Errorf("loading OpenAPI spec: %w", err)
	}
	return swagger, nil
}

func resolvePackage(pkgPath string) (*packages.Package, error) {
	pkgs, err := packages.Load(&packages.Config{Mode: packages.LoadAllSyntax}, pkgPath)
	if err != nil {
		return nil, fmt.Errorf("loading models package: %w", err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no package found for models package: %s", pkgPath)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("errors loading models package: %v", pkg.Errors)
	}
	return pkg, nil
}

func detectPackageName(outDir string) string {
	if !path.IsAbs(outDir) && !strings.HasPrefix(outDir, ".") {
		return filepath.Base(outDir)
	}
	abs, err := filepath.Abs(filepath.FromSlash(outDir))
	if err != nil {
		return "testhandler"
	}
	return filepath.Base(abs)
}

func generateModels(spec *openapi3.T, opts codegen.Configuration, outDir, modelsPkg string) (errOut error) {
	opts.Generate = codegen.GenerateOptions{
		Models: true,
	}

	generated, err := codegen.Generate(spec, opts)
	if err != nil {
		return fmt.Errorf("generating server code: %w", err)
	}

	if modelsPkg != "" {
		generated, err = aliasModels(modelsPkg, generated)
		if err != nil {
			return err
		}
	}

	return writeFile(filepath.Join(outDir, "oapi_models_gen.go"), []byte(generated))
}

func getModelExports(modelsPkg *packages.Package) (map[string]bool, error) {
	modelExports := map[string]bool{}

	inspector.New(modelsPkg.Syntax).Nodes([]ast.Node{&ast.TypeSpec{}, &ast.ValueSpec{}}, func(
		n ast.Node,
		_ bool,
	) (proceed bool) {
		switch spec := n.(type) {
		case *ast.TypeSpec:
			if spec.Name.IsExported() {
				modelExports[spec.Name.Name] = true
			}
		case *ast.ValueSpec:
			for _, name := range spec.Names {
				if name.IsExported() {
					modelExports[name.Name] = true
				}
			}
		}
		return false
	})

	return modelExports, nil
}

func aliasModels(modelsPkgPath, generated string) (string, error) {
	modelsPkg, err := resolvePackage(modelsPkgPath)
	if err != nil {
		return "", fmt.Errorf("resolving models package: %w", err)
	}

	modelExports, err := getModelExports(modelsPkg)
	if err != nil {
		return "", fmt.Errorf("getting model exports: %w", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", generated, 0)
	if err != nil {
		return "", fmt.Errorf("parsing generated code: %w", err)
	}

	astutil.AddNamedImport(fset, file, "clientpkg", modelsPkg.PkgPath)

	replaceWithClientPkgRefs(file, modelExports)

	var buf bytes.Buffer
	err = printer.Fprint(&buf, fset, file)
	if err != nil {
		return "", fmt.Errorf("printing modified code: %w", err)
	}

	return buf.String(), nil
}

func replaceWithClientPkgRefs(file *ast.File, modelExports map[string]bool) {
	inspector.New([]*ast.File{file}).Nodes(
		[]ast.Node{&ast.TypeSpec{}, &ast.ValueSpec{}},
		func(n ast.Node, _ bool) bool {
			switch spec := n.(type) {
			case *ast.TypeSpec:
				if modelExports[spec.Name.Name] {
					spec.Assign = token.Pos(1) // Mark as type alias
					spec.Type = &ast.SelectorExpr{
						X:   &ast.Ident{Name: "clientpkg"},
						Sel: &ast.Ident{Name: spec.Name.Name},
					}
				}
			case *ast.ValueSpec:
				spec.Type = nil // Let type be inferred
				for i, name := range spec.Names {
					if modelExports[name.Name] && i < len(spec.Values) {
						spec.Values[i] = &ast.SelectorExpr{
							X:   &ast.Ident{Name: "clientpkg"},
							Sel: &ast.Ident{Name: name.Name},
						}
					}
				}
			}
			return false
		},
	)
}

func generateServer(spec *openapi3.T, opts codegen.Configuration, outDir string) (errOut error) {
	opts.Generate = codegen.GenerateOptions{
		StdHTTPServer: true,
		Strict:        true,
	}

	generated, err := codegen.Generate(spec, opts)
	if err != nil {
		return fmt.Errorf("generating server code: %w", err)
	}

	out, err := os.Create(filepath.Join(outDir, "oapi_gen.go"))
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() {
		errOut = errors.Join(errOut, out.Close())
	}()

	_, err = out.WriteString(generated)
	if err != nil {
		return fmt.Errorf("writing generated code to file: %w", err)
	}

	return nil
}

func generateTestHandler(spec *openapi3.T, opts codegen.Configuration, outDir string) error {
	operations, err := codegen.OperationDefinitions(spec, opts.OutputOptions.InitialismOverrides)
	if err != nil {
		return fmt.Errorf("getting operation definitions: %w", err)
	}

	err = writeHelpers(opts.PackageName, outDir)
	if err != nil {
		return fmt.Errorf("writing helpers.go: %w", err)
	}

	handlerCode, err := buildTestHandler(operations, opts.PackageName)
	if err != nil {
		return fmt.Errorf("building test handler: %w", err)
	}
	handlerPath := filepath.Join(outDir, "handler.go")
	err = writeFile(handlerPath, []byte(handlerCode))
	if err != nil {
		return fmt.Errorf("writing handler.go: %w", err)
	}

	serverCode, err := buildTestServer(operations, opts.PackageName)
	if err != nil {
		return fmt.Errorf("building test server: %w", err)
	}
	serverPath := filepath.Join(outDir, "server.go")
	err = writeFile(serverPath, []byte(serverCode))
	if err != nil {
		return fmt.Errorf("writing server.go: %w", err)
	}

	return nil
}

type bodyData struct {
	MethodSuffix        string
	FieldName           string
	TypeName            string
	IsPointer           bool
	RequiresContentType bool
}

type responseTypeData struct {
	TypeName       string
	MethodName     string
	StatusCode     string
	ContentType    string
	IsEmpty        bool
	UnderlyingType string // For type aliases, this is the underlying type
	IsAlias        bool   // True if this is a type alias
}

func parseResponseTypes(op codegen.OperationDefinition) []responseTypeData {
	var responseTypes []responseTypeData

	for _, respDef := range op.Responses {
		statusCode := respDef.StatusCode

		// Skip non-integer status codes like "default"
		if !isIntegerStatusCode(statusCode) {
			continue
		}

		// Handle responses without content
		if len(respDef.Contents) == 0 {
			responseTypes = append(responseTypes, responseTypeData{
				TypeName:    op.OperationId + statusCode + "Response",
				MethodName:  "Respond" + statusCode,
				StatusCode:  statusCode,
				ContentType: "",
				IsEmpty:     true,
			})
			continue
		}

		// Handle responses with content
		// Only include contents with non-empty NameTag
		for _, content := range respDef.Contents {
			nameTag := content.NameTag
			if nameTag == "" {
				continue
			}

			// Check if this response type is a simple type alias.
			// Responses are generated as structs (not simple aliases) when:
			// - The response is a $ref to a component response (embedded struct)
			// - The response has headers (struct with Body and Headers fields)
			// - The schema has inline properties (struct with fields)
			// Otherwise, it's a simple type alias (type FooResponse Bar).

			isAlias := !respDef.IsRef() &&
				len(respDef.Headers) == 0 &&
				content.Schema.GoType != "" &&
				!strings.Contains(content.Schema.GoType, "\n") &&
				len(content.Schema.Properties) == 0

			var underlyingType string
			if isAlias {
				underlyingType = content.Schema.GoType
			}

			responseTypes = append(responseTypes, responseTypeData{
				TypeName:       op.OperationId + statusCode + nameTag + "Response",
				MethodName:     "Respond" + nameTag + statusCode,
				StatusCode:     statusCode,
				ContentType:    nameTag,
				UnderlyingType: underlyingType,
				IsAlias:        isAlias,
			})
		}
	}

	return responseTypes
}

func isIntegerStatusCode(statusCode string) bool {
	for _, c := range statusCode {
		if c < '0' || c > '9' {
			return false
		}
	}
	return statusCode != ""
}

func parseBodies(op codegen.OperationDefinition) []bodyData {
	if !op.HasBody() {
		return nil
	}
	var bodies []bodyData
	for _, bodyDef := range op.Bodies {

		// When there's only one body type, oapi-codegen uses "Body" regardless of type
		// When there are multiple body types, it uses "{NameTag}Body" for typed and "Body" for generic
		fieldName := bodyDef.NameTag + "Body"
		if len(op.Bodies) == 1 {
			fieldName = "Body"
		}

		// Determine type name
		typeName := "[]byte"
		isPointer := false
		if bodyDef.NameTag != "" {
			typeName = op.OperationId + bodyDef.NameTag + "RequestBody"
			isPointer = true
		}

		bodies = append(bodies, bodyData{
			MethodSuffix:        bodyDef.Suffix(),
			FieldName:           fieldName,
			TypeName:            typeName,
			IsPointer:           isPointer,
			RequiresContentType: bodyDef.NameTag == "",
		})
	}
	return bodies
}

type operationData struct {
	OperationID          string
	LowercaseOperationID string
	MethodName           string
	RequestType          string
	ResponseType         string
	ExpectationField     string
	HasBody              bool
	HasGenericBody       bool
	PathParams           []codegen.ParameterDefinition
	HasQueryParams       bool
	QueryParamsType      string
	Bodies               []bodyData
	BuilderTypeName      string
	ResponseTypes        []responseTypeData
}

func buildTestHandler(operations []codegen.OperationDefinition, packageName string) (string, error) {
	var ops []operationData
	for i := range operations {
		op := operations[i]

		// Determine if operation has Params field (query/header params)
		hasQueryParams := op.RequiresParamObject()
		queryParamsType := ""
		if hasQueryParams {
			queryParamsType = op.OperationId + "Params"
		}

		bodies := parseBodies(op)

		// Check if operation has generic Body field (io.Reader)
		hasGenericBody := false
		for _, body := range bodies {
			if body.RequiresContentType {
				hasGenericBody = true
				break
			}
		}

		ops = append(ops, operationData{
			OperationID:          op.OperationId,
			LowercaseOperationID: codegen.LowercaseFirstCharacter(op.OperationId),
			MethodName:           "Expect" + op.OperationId,
			RequestType:          op.OperationId + "RequestObject",
			ResponseType:         op.OperationId + "ResponseObject",
			ExpectationField:     codegen.LowercaseFirstCharacter(op.OperationId) + "ExpectResponses",
			HasBody:              op.HasBody(),
			HasGenericBody:       hasGenericBody,
			PathParams:           op.PathParams,
			HasQueryParams:       hasQueryParams,
			QueryParamsType:      queryParamsType,
			Bodies:               bodies,
			BuilderTypeName:      op.OperationId + "Expectation",
			ResponseTypes:        parseResponseTypes(op),
		})
	}

	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "handler.tmpl", map[string]any{
		"PackageName": packageName,
		"Operations":  ops,
	})
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

func buildTestServer(operations []codegen.OperationDefinition, packageName string) (string, error) {
	var ops []operationData
	for i := range operations {
		op := operations[i]

		// Check if operation has generic Body field (io.Reader)
		hasGenericBody := false
		if op.HasBody() {
			for _, bodyDef := range op.Bodies {
				if bodyDef.NameTag == "" {
					hasGenericBody = true
					break
				}
			}
		}

		ops = append(ops, operationData{
			OperationID:      op.OperationId,
			MethodName:       "Expect" + op.OperationId,
			RequestType:      op.OperationId + "RequestObject",
			ResponseType:     op.OperationId + "ResponseObject",
			ExpectationField: codegen.LowercaseFirstCharacter(op.OperationId) + "ExpectResponses",
			HasGenericBody:   hasGenericBody,
		})
	}

	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "server.tmpl", map[string]any{
		"PackageName": packageName,
		"Operations":  ops,
	})
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

func writeHelpers(packageName, outDir string) error {
	source := bytes.Replace(helpersSource, []byte("package helpers"), []byte("package "+packageName), 1)

	filename := filepath.Join(outDir, "helpers.go")

	return writeFile(filename, source)
}

func writeFile(filename string, content []byte) (errOut error) {
	const header = "// Code generated by github.com/willabides/oapitesthandler/cmd/oapitesthandler. DO NOT EDIT.\n\n"

	source, err := imports.Process(filename, append([]byte(header), content...), nil)
	if err != nil {
		return fmt.Errorf("running goimports: %w", err)
	}
	source, err = format.Source(source, format.Options{ExtraRules: true})
	if err != nil {
		return fmt.Errorf("running gofumpt: %w", err)
	}

	out, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}

	defer func() { errOut = errors.Join(errOut, out.Close()) }()

	_, err = out.Write(source)
	if err != nil {
		return fmt.Errorf("writing to output file: %w", err)
	}

	return nil
}
