package cli

import (
	"testing"

	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
)

func TestRenderEnvironmentTable(t *testing.T) {
	envs := []environment.Environment{
		{Name: "main", State: "running", InstanceType: "m7i.xlarge", RootVolumeSizeGB: 100},
		{Name: "build", State: "stopped", InstanceType: "m7i.4xlarge", RootVolumeSizeGB: 300},
	}
	out := renderEnvironmentTable(envs, "main")

	want := "NAME   STATE    TYPE         DISK    DEFAULT\n" +
		"build  stopped  m7i.4xlarge  300 GB\n" +
		"main   running  m7i.xlarge   100 GB  *\n"
	if out != want {
		t.Errorf("table =\n%s\nwant\n%s", out, want)
	}
}

func TestRenderEnvironmentTable_MissingDefault(t *testing.T) {
	envs := []environment.Environment{{Name: "main", State: "running", InstanceType: "m7i.xlarge"}}
	out := renderEnvironmentTable(envs, "gone")

	want := "NAME  STATE    TYPE        DISK  DEFAULT\n" +
		"main  running  m7i.xlarge  -\n" +
		"\n" +
		"Default environment \"gone\" no longer exists. Set one with 'clouddesktop use <name>'.\n"
	if out != want {
		t.Errorf("table =\n%q\nwant\n%q", out, want)
	}
}
