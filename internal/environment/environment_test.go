package environment

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "single word", input: "build"},
		{name: "another word", input: "service"},
		{name: "letters and digits", input: "ci2"},
		{name: "hyphenated", input: "perf-test"},
		{name: "twenty characters", input: strings.Repeat("a", 20)},
		{name: "empty", input: "", wantErr: true},
		{name: "uppercase", input: "Build", wantErr: true},
		{name: "leading hyphen", input: "-x", wantErr: true},
		{name: "trailing hyphen", input: "x-", wantErr: true},
		{name: "underscore", input: "a_b", wantErr: true},
		{name: "dot", input: "a.b", wantErr: true},
		{name: "twenty-one characters", input: strings.Repeat("a", 21), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateName(%q) = nil, want error", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateName(%q) = %v, want nil", tt.input, err)
			}
		})
	}
}

func TestValidateName_Message(t *testing.T) {
	err := ValidateName("Build_1")
	if err == nil {
		t.Fatal("expected error")
	}
	want := "invalid environment name \"Build_1\"\nUse 1-20 lowercase letters, digits or hyphens, starting and ending with a letter or digit"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestSSHAliases(t *testing.T) {
	tests := []struct {
		env  string
		want []string
	}{
		{env: "main", want: []string{"clouddesktop-main", "clouddesktop"}},
		{env: "build", want: []string{"clouddesktop-build"}},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			got := Environment{Name: tt.env}.SSHAliases()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SSHAliases() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSSHAlias(t *testing.T) {
	if got := SSHAlias("build"); got != "clouddesktop-build" {
		t.Errorf("SSHAlias(build) = %q, want clouddesktop-build", got)
	}
}

func TestNames_Sorted(t *testing.T) {
	envs := []Environment{{Name: "service"}, {Name: "build"}, {Name: "main"}}

	got := Names(envs)

	want := []string{"build", "main", "service"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
}

func TestValidateNew(t *testing.T) {
	existing := []Environment{{Name: "main"}, {Name: "build"}}

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "new valid name", input: "service"},
		{name: "invalid name", input: "Build_1", wantErr: "invalid environment name \"Build_1\""},
		{name: "taken name", input: "build", wantErr: "environment \"build\" already exists\nChoose another name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNew(tt.input, existing)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateNew(%q) = %v, want nil", tt.input, err)
				}
				return
			}
			if err == nil || !strings.HasPrefix(err.Error(), tt.wantErr) {
				t.Errorf("ValidateNew(%q) = %v, want error starting with %q", tt.input, err, tt.wantErr)
			}
		})
	}
}
