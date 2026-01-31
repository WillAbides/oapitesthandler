package handlergen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name          string
		testdataDir   string
		modelsPkgPath string
	}{
		{
			name:        "simple_get",
			testdataDir: "testdata/simple_get",
		},
		{
			name:        "with_bodies",
			testdataDir: "testdata/with_bodies",
		},
		{
			name:          "with_models",
			testdataDir:   "testdata/with_models",
			modelsPkgPath: "./models",
		},
		{
			name:        "with_operation_filter",
			testdataDir: "testdata/with_operation_filter",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(test.testdataDir)
			outputDir := filepath.Join(t.TempDir(), "generated")
			require.NoError(t, os.MkdirAll(outputDir, 0o755))

			// If models package is specified, create a proper module structure
			modelsPkgPath := test.modelsPkgPath

			// Run handlergen
			err := Run("openapi.yaml", "oapi-codegen.yaml", outputDir, modelsPkgPath)
			require.NoError(t, err)

			if os.Getenv("UPDATE_SNAPS") != "" {
				require.NoError(t, os.RemoveAll("generated"))
				require.NoError(t, os.MkdirAll("generated", 0o755))
				require.NoError(t, copyDir(outputDir, "generated"))
			}

			// In compare mode, verify generated files match snapshots
			assertEqualDir(t, "generated", outputDir)

			// Additional checks for specific test cases
			if test.name == "with_operation_filter" {
				// Verify that operations are actually generated when using include-operation-ids
				handlerContent, err := os.ReadFile(filepath.Join(outputDir, "handler.go"))
				require.NoError(t, err)
				assert.Contains(t, string(handlerContent), "ExpectGetUser", "Expected ExpectGetUser method to be generated with include-operation-ids filter")
			}
		})
	}
}

func getDirFiles(t require.TestingT, dir string) []string {
	var files []string
	require.NoError(t, filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, relPath)
		return nil
	}))
	return files
}

func assertEqualDir(t *testing.T, expectedDir, actualDir string) {
	t.Helper()

	expectedFiles := getDirFiles(t, expectedDir)

	assert.Equal(t, expectedFiles, getDirFiles(t, actualDir), "file lists do not match")

	for _, filename := range expectedFiles {
		expectedContent, err := os.ReadFile(filepath.Join(expectedDir, filename))
		require.NoError(t, err)

		actualContent, err := os.ReadFile(filepath.Join(actualDir, filename))
		if assert.NoError(t, err) {
			assert.Equal(t, string(expectedContent), string(actualContent), "file contents do not match for %s", filename)
		}
	}
}

// copyDir copies all files from srcDir to dstDir
func copyDir(srcDir, dstDir string) error {
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, 0o644)
	})
}
