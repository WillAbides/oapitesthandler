package handlergen

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"golang.org/x/tools/imports"
	"gopkg.in/yaml.v3"
	"mvdan.cc/gofumpt/format"
)

//go:embed helpers/helpers.go
var helpersSource []byte

func Run(specPath, configPath, outputPath string) error {
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

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	spec, err := loader.LoadFromFile(specPath)
	if err != nil {
		return fmt.Errorf("loading OpenAPI spec: %w", err)
	}

	// Generate strict server types (RequestObject, ResponseObject, etc.)
	err = generateServer(spec, opts, outputPath)
	if err != nil {
		return fmt.Errorf("generating server code: %w", err)
	}

	// Generate test handler
	err = generateTestHandler(spec, opts, outputPath)
	if err != nil {
		return fmt.Errorf("generating test handler: %w", err)
	}

	return nil
}

func detectPackageName(outDir string) string {
	base := filepath.Base(outDir)
	if base != "." && base != "/" {
		return base
	}
	abs, err := filepath.Abs(outDir)
	if err != nil {
		return "testhandler"
	}
	return filepath.Base(abs)
}

func generateServer(spec *openapi3.T, opts codegen.Configuration, outDir string) (errOut error) {
	opts.Generate = codegen.GenerateOptions{
		Models:        true,
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
	// Get operations
	operations, err := codegen.OperationDefinitions(spec, opts.OutputOptions.InitialismOverrides)
	if err != nil {
		return fmt.Errorf("getting operation definitions: %w", err)
	}

	// Write helpers.go
	err = writeHelpers(opts.PackageName, outDir)
	if err != nil {
		return fmt.Errorf("writing helpers.go: %w", err)
	}

	// Generate and write handler.go
	handlerCode, err := buildTestHandler(operations, opts.PackageName)
	if err != nil {
		return fmt.Errorf("building test handler: %w", err)
	}
	handlerPath := filepath.Join(outDir, "handler.go")
	err = writeFile(handlerPath, []byte(handlerCode))
	if err != nil {
		return fmt.Errorf("writing handler.go: %w", err)
	}

	// Generate and write server.go
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

type operationData struct {
	OperationID      string
	MethodName       string
	RequestType      string
	ResponseType     string
	ExpectationField string
}

func buildTestHandler(operations []codegen.OperationDefinition, packageName string) (string, error) {
	// Prepare operation data
	var ops []operationData
	for i := range operations {
		op := operations[i]
		ops = append(ops, operationData{
			OperationID:      op.OperationId,
			MethodName:       "Expect" + op.OperationId,
			RequestType:      op.OperationId + "RequestObject",
			ResponseType:     op.OperationId + "ResponseObject",
			ExpectationField: strings.ToLower(string(op.OperationId[0])) + op.OperationId[1:] + "Expectations",
		})
	}

	tmpl, err := template.New("testhandler").Parse(testHandlerTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{
		"PackageName": packageName,
		"Operations":  ops,
	})
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

func buildTestServer(operations []codegen.OperationDefinition, packageName string) (string, error) {
	// Prepare operation data
	var ops []operationData
	for i := range operations {
		op := operations[i]
		ops = append(ops, operationData{
			OperationID:      op.OperationId,
			MethodName:       "Expect" + op.OperationId,
			RequestType:      op.OperationId + "RequestObject",
			ResponseType:     op.OperationId + "ResponseObject",
			ExpectationField: strings.ToLower(string(op.OperationId[0])) + op.OperationId[1:] + "Expectations",
		})
	}

	tmpl, err := template.New("testserver").Parse(testServerTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{
		"PackageName": packageName,
		"Operations":  ops,
	})
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

func writeHelpers(packageName, outDir string) error {
	// replace package name
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

const testHandlerTemplate = `
package {{.PackageName}}

import (
	"net/http"
)

type TestHandler struct {
	tb TB

	handler http.Handler
{{range .Operations}}
	{{.ExpectationField}} expectations[{{.RequestType}}, {{.ResponseType}}]
{{- end}}
}

func (s *TestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func NewTestHandler(tb TB) *TestHandler {
	th := &TestHandler{tb: tb}
	s := &testServer{tb: tb, handler: th}
	th.handler = Handler(NewStrictHandler(s, nil))
	return th
}
{{range .Operations}}
func (s *TestHandler) {{.MethodName}}(
	req {{.RequestType}},
	resp {{.ResponseType}},
	opts ...ExpectOption,
) {
	s.{{.ExpectationField}}.Expect(s.tb, req, resp, opts...)
}
{{end}}
`

const testServerTemplate = `
package {{.PackageName}}

import (
	"context"
)

type testServer struct {
	tb     TB
	handler *TestHandler
}
{{range .Operations}}
func (t *testServer) {{.OperationID}}(_ context.Context, req {{.RequestType}}) ({{.ResponseType}}, error) {
	return t.handler.{{.ExpectationField}}.GetResponse(t.tb, req)
}
{{end}}
`
