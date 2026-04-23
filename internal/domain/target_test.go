package domain

import "testing"

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Target
		wantErr bool
	}{
		{
			name:  "user target",
			input: "yorukot",
			want: Target{
				Input:            "yorukot",
				NormalizedTarget: "yorukot",
				Mode:             ModeUserOrOrg,
				Owner:            "yorukot",
			},
		},
		{
			name:  "repo target",
			input: "yorukot/superfile",
			want: Target{
				Input:            "yorukot/superfile",
				NormalizedTarget: "yorukot/superfile",
				Mode:             ModeSingleRepo,
				Owner:            "yorukot",
				Repo:             "superfile",
			},
		},
		{name: "invalid empty", input: "", wantErr: true},
		{name: "invalid trailing slash", input: "yorukot/", wantErr: true},
		{name: "invalid too many parts", input: "a/b/c", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTarget(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected target: %#v", got)
			}
		})
	}
}
