package api

import "testing"

func TestCleanPathTraversal(t *testing.T) {
	tests := []struct {
		root, in, want string
		wantErr        bool
	}{
		{"/", "", "/", false},
		{"/", "/", "/", false},
		{"/", "/var/log", "/var/log", false},
		{"/", "/var/../etc", "/etc", false},
		{"/", "var/log", "/var/log", false},
		{"/", "/a/b/../../..", "/", false},
		{"/", "/etc\x00/passwd", "", true},

		{"/home/u", "/home/u/docs", "/home/u/docs", false},
		{"/home/u", "docs/a.txt", "/home/u/docs/a.txt", false},
		{"/home/u", "/home/u", "/home/u", false},
		{"/home/u", "/home/u/../..", "", true},
		{"/home/u", "/etc/passwd", "", true},
		{"/home/u", "../../etc/passwd", "", true},
		{"/home/u", "/home/user2", "", true},
	}

	for _, tc := range tests {
		got, err := CleanPath(tc.root, tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("CleanPath(%q, %q) = %q, want error", tc.root, tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("CleanPath(%q, %q): unexpected error %v", tc.root, tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("CleanPath(%q, %q) = %q, want %q", tc.root, tc.in, got, tc.want)
		}
	}
}
