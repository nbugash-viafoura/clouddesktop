package environment

import (
	"errors"
	"testing"
)

func TestResolve(t *testing.T) {
	main := Environment{Name: "main", InstanceID: "i-main"}
	build := Environment{Name: "build", InstanceID: "i-build"}
	two := []Environment{main, build}

	tests := []struct {
		name     string
		req      ResolveRequest
		wantEnv  string
		wantKind ResolveErrorKind
	}{
		{
			name:    "explicit name found",
			req:     ResolveRequest{Name: "build", Environments: two},
			wantEnv: "build",
		},
		{
			name:     "explicit name not found",
			req:      ResolveRequest{Name: "nope", Environments: two},
			wantKind: UnknownEnvironment,
		},
		{
			name:     "explicit name with no environments",
			req:      ResolveRequest{Name: "build"},
			wantKind: UnknownEnvironment,
		},
		{
			name:    "explicit name for destroy",
			req:     ResolveRequest{Name: "main", Environments: two, Operation: OperationDestroy},
			wantEnv: "main",
		},
		{
			name:     "no environments",
			req:      ResolveRequest{},
			wantKind: NoEnvironments,
		},
		{
			name:     "no environments for destroy",
			req:      ResolveRequest{Operation: OperationDestroy},
			wantKind: NoEnvironments,
		},
		{
			name:    "one environment, no default",
			req:     ResolveRequest{Environments: []Environment{build}},
			wantEnv: "build",
		},
		{
			name:    "one environment, default points elsewhere",
			req:     ResolveRequest{Environments: []Environment{build}, Default: "main"},
			wantEnv: "build",
		},
		{
			name:    "one environment, destroy",
			req:     ResolveRequest{Environments: []Environment{build}, Operation: OperationDestroy},
			wantEnv: "build",
		},
		{
			name:     "several environments, destroy needs a name even with a default",
			req:      ResolveRequest{Environments: two, Default: "main", Operation: OperationDestroy},
			wantKind: NameRequired,
		},
		{
			name:    "several environments, default set and present",
			req:     ResolveRequest{Environments: two, Default: "build"},
			wantEnv: "build",
		},
		{
			name:     "several environments, default set but missing",
			req:      ResolveRequest{Environments: two, Default: "gone"},
			wantKind: DefaultMissing,
		},
		{
			name:     "several environments, no default",
			req:      ResolveRequest{Environments: two},
			wantKind: NoDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := Resolve(tt.req)

			if tt.wantKind != 0 {
				var resolveErr *ResolveError
				if !errors.As(err, &resolveErr) {
					t.Fatalf("Resolve() error = %v, want *ResolveError", err)
				}
				if resolveErr.Kind != tt.wantKind {
					t.Errorf("Kind = %v, want %v", resolveErr.Kind, tt.wantKind)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve() unexpected error: %v", err)
			}
			if env.Name != tt.wantEnv {
				t.Errorf("Resolve() = %q, want %q", env.Name, tt.wantEnv)
			}
		})
	}
}

func TestResolveError_Messages(t *testing.T) {
	available := []string{"build", "main"}

	tests := []struct {
		name string
		err  *ResolveError
		want string
	}{
		{
			name: "unknown environment",
			err:  &ResolveError{Kind: UnknownEnvironment, Name: "nope", Available: available},
			want: "environment \"nope\" not found\nYour environments: build, main. Create one with 'clouddesktop create <name>'.",
		},
		{
			name: "unknown environment with none",
			err:  &ResolveError{Kind: UnknownEnvironment, Name: "nope"},
			want: "environment \"nope\" not found\nYour environments: none. Create one with 'clouddesktop create <name>'.",
		},
		{
			name: "no default",
			err:  &ResolveError{Kind: NoDefault, Available: available},
			want: "you have more than one environment and no default\nPass a name, or set a default with 'clouddesktop use <name>'. Your environments: build, main.",
		},
		{
			name: "default missing",
			err:  &ResolveError{Kind: DefaultMissing, Name: "gone", Available: available},
			want: "default environment \"gone\" no longer exists\nSet a new default with 'clouddesktop use <name>'. Your environments: build, main.",
		},
		{
			name: "destroy needs a name",
			err:  &ResolveError{Kind: NameRequired, Available: available},
			want: "destroy requires an environment name when you have more than one\nRun 'clouddesktop destroy <name> --confirm'. Your environments: build, main.",
		},
		{
			name: "no environments",
			err:  &ResolveError{Kind: NoEnvironments},
			want: "you have no environments\nCreate one with 'clouddesktop create <name>' or 'clouddesktop up'.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestResolve_AvailableNamesSorted(t *testing.T) {
	envs := []Environment{{Name: "service"}, {Name: "build"}, {Name: "main"}}

	_, err := Resolve(ResolveRequest{Name: "nope", Environments: envs})

	var resolveErr *ResolveError
	if !errors.As(err, &resolveErr) {
		t.Fatalf("expected *ResolveError, got %v", err)
	}
	want := []string{"build", "main", "service"}
	for i, name := range want {
		if resolveErr.Available[i] != name {
			t.Fatalf("Available = %v, want %v", resolveErr.Available, want)
		}
	}
}

func TestIsKind(t *testing.T) {
	_, err := Resolve(ResolveRequest{})

	if !IsKind(err, NoEnvironments) {
		t.Error("IsKind(err, NoEnvironments) = false, want true")
	}
	if IsKind(err, NoDefault) {
		t.Error("IsKind(err, NoDefault) = true, want false")
	}
	if IsKind(errors.New("other"), NoEnvironments) {
		t.Error("IsKind(non-resolve error) = true, want false")
	}
}
