package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDatasets checks where loadConfig looks for terrain, canopy and HF data.
func TestDatasets(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"canopy1", "hf1"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	base := func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://x")
		t.Setenv("RADIOPATH_REGISTRATION", "false")
		// unset, not empty: an empty RADIOPATH_DATASET_CANOPY or RADIOPATH_DATASET_HF means none
		for _, k := range []string{"RADIOPATH_DATASET_DIR", "RADIOPATH_DATASET_S3_ENDPOINT", "RADIOPATH_DATASET_S3_BUCKET",
			"RADIOPATH_DATASET_DEM", "RADIOPATH_DATASET_CANOPY", "RADIOPATH_DATASET_HF", "RADIOPATH_HF_DIR"} {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}

	t.Run("defaults", func(t *testing.T) {
		base(t)
		t.Setenv("RADIOPATH_DATASET_DIR", root)
		c, err := loadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if c.demDir != filepath.Join(root, "srtm1") || c.canopyDir != filepath.Join(root, "canopy1") {
			t.Errorf("dem %q, canopy %q", c.demDir, c.canopyDir)
		}
		if c.hfDir != filepath.Join(root, "hf1") || c.hfPrefix != "" {
			t.Errorf("hf %q, %q", c.hfDir, c.hfPrefix)
		}
	})

	t.Run("hf off or elsewhere", func(t *testing.T) {
		base(t)
		t.Setenv("RADIOPATH_DATASET_DIR", root)
		t.Setenv("RADIOPATH_DATASET_HF", "")
		if c, err := loadConfig(); err != nil || c.hfDir != "" {
			t.Errorf("hf %q, %v", c.hfDir, err)
		}
		t.Setenv("RADIOPATH_DATASET_HF", "hf2") // missing folder
		if c, err := loadConfig(); err != nil || c.hfDir != "" {
			t.Errorf("hf %q, %v", c.hfDir, err)
		}
		if err := os.Mkdir(filepath.Join(root, "hf2"), 0o755); err != nil {
			t.Fatal(err)
		}
		if c, err := loadConfig(); err != nil || c.hfDir != filepath.Join(root, "hf2") {
			t.Errorf("hf %q, %v", c.hfDir, err)
		}
	})

	t.Run("canopy off", func(t *testing.T) {
		base(t)
		t.Setenv("RADIOPATH_DATASET_DIR", root)
		t.Setenv("RADIOPATH_DATASET_CANOPY", "")
		if c, err := loadConfig(); err != nil || c.canopyDir != "" {
			t.Errorf("canopy %q, %v", c.canopyDir, err)
		}
		t.Setenv("RADIOPATH_DATASET_CANOPY", "canopy2") // missing folder
		if c, err := loadConfig(); err != nil || c.canopyDir != "" {
			t.Errorf("canopy %q, %v", c.canopyDir, err)
		}
	})

	t.Run("s3", func(t *testing.T) {
		base(t)
		t.Setenv("RADIOPATH_DATASET_S3_ENDPOINT", "https://s3.example.org")
		t.Setenv("RADIOPATH_DATASET_S3_BUCKET", "datasets")
		t.Setenv("RADIOPATH_DATASET_DEM", "glo30")
		c, err := loadConfig()
		if err != nil {
			t.Fatal(err)
		}
		if c.s3Prefix != "glo30/" || c.canopyPrefix != "canopy1/" || c.demDir != "" {
			t.Errorf("dem %q, canopy %q, dir %q", c.s3Prefix, c.canopyPrefix, c.demDir)
		}
		if c.hfPrefix != "hf1/" || c.hfDir != "" {
			t.Errorf("hf %q, dir %q", c.hfPrefix, c.hfDir)
		}
		t.Setenv("RADIOPATH_DATASET_HF", "")
		if c, err := loadConfig(); err != nil || c.hfPrefix != "" {
			t.Errorf("hf %q, %v", c.hfPrefix, err)
		}
	})

	for name, env := range map[string]map[string]string{
		"no root":     {},
		"old name":    {"RADIOPATH_DATASET_DIR": root, "RADIOPATH_DEM_DIR": root},
		"old hf name": {"RADIOPATH_DATASET_DIR": root, "RADIOPATH_HF_DIR": root},
		"no endpoint": {"RADIOPATH_DATASET_S3_BUCKET": "datasets"},
	} {
		t.Run(name, func(t *testing.T) {
			base(t)
			for k, v := range env {
				t.Setenv(k, v)
			}
			if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "RADIOPATH_") {
				t.Errorf("want an error, got %v", err)
			}
		})
	}
}

// TestMode checks that a web process needs no datasets and that an unknown mode is refused.
func TestMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("RADIOPATH_REGISTRATION", "false")
	for _, k := range []string{"RADIOPATH_DATASET_DIR", "RADIOPATH_DATASET_S3_BUCKET"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "RADIOPATH_DATASET_DIR") {
		t.Errorf("mode all without datasets: err = %v, want the dataset root required", err)
	}
	t.Setenv("RADIOPATH_MODE", "web")
	if c, err := loadConfig(); err != nil || c.mode != "web" || c.demDir != "" {
		t.Errorf("mode web without datasets: %+v %v", c, err)
	}
	t.Setenv("RADIOPATH_MODE", "cluster")
	if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "RADIOPATH_MODE") {
		t.Errorf("unknown mode: err = %v", err)
	}
}
