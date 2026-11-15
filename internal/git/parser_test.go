package git

import (
	"errors"
	"strings"
	"testing"
)

const (
	testSHA    = "0123456789abcdef0123456789abcdef01234567"
	testBranch = "main"
)

func TestParseStatus(t *testing.T) {
	clean := "# branch.oid " + testSHA + "\n# branch.head main\n"
	tests := []struct {
		name string
		out  string
		want RepoInfo
	}{
		{
			name: "normal clean branch", out: clean,
			want: RepoInfo{Branch: testBranch, HeadSHA: testSHA},
		},
		{
			name: "unstaged change", out: clean + "1 .M N... 100644 100644 100644 abc def file\n",
			want: RepoInfo{Branch: testBranch, HeadSHA: testSHA, IsDirty: true},
		},
		{
			name: "staged change", out: clean + "1 M. N... 100644 100644 100644 abc def file\n",
			want: RepoInfo{Branch: testBranch, HeadSHA: testSHA, IsDirty: true},
		},
		{
			name: "unmerged change", out: clean + "u UU N... 100644 100644 100644 100644 a b c file\n",
			want: RepoInfo{Branch: testBranch, HeadSHA: testSHA, IsDirty: true},
		},
		{
			name: "detached head", out: "# branch.oid " + testSHA + "\n# branch.head (detached)\n",
			want: RepoInfo{HeadSHA: testSHA, Detached: true},
		},
		{
			name: "unborn branch", out: "# branch.oid (initial)\n# branch.head main\n",
			want: RepoInfo{Branch: testBranch, Unborn: true},
		},
		{
			name: "upstream counts", out: clean + "# branch.upstream origin/main\n# branch.ab +2 -1\n",
			want: RepoInfo{Branch: testBranch, HeadSHA: testSHA, Upstream: "origin/main", Ahead: 2, Behind: 1},
		},
		{
			name: "no upstream", out: "# branch.head feature/local\n",
			want: RepoInfo{Branch: "feature/local"},
		},
		{
			name: "crlf endings", out: strings.ReplaceAll(clean, "\n", "\r\n") + "\r\n",
			want: RepoInfo{Branch: testBranch, HeadSHA: testSHA},
		},
		{
			name: "unknown headers ignored", out: clean + "# stash 2\n#future-header\n\n",
			want: RepoInfo{Branch: testBranch, HeadSHA: testSHA},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseStatus([]byte(tt.out))
			if err != nil {
				t.Fatalf("ParseStatus() error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseStatus() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseStatusOtherEntries(t *testing.T) {
	for _, entry := range []string{"2 R. rename", "? untracked", "! ignored", "unexpected entry"} {
		t.Run(entry, func(t *testing.T) {
			got, err := ParseStatus([]byte("# branch.head main\n" + entry))
			if err != nil || !got.IsDirty {
				t.Fatalf("ParseStatus() = (%+v, %v), want dirty repository", got, err)
			}
		})
	}
}

func TestParseStatusInvalidOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
	}{
		{name: "missing head", out: "# branch.oid " + testSHA + "\n"},
		{name: "empty output"},
		{name: "nonnumeric ahead", out: "# branch.head main\n# branch.ab +x -1"},
		{name: "nonnumeric behind", out: "# branch.head main\n# branch.ab +2 -x"},
		{name: "missing count", out: "# branch.head main\n# branch.ab +2"},
		{name: "missing signs", out: "# branch.head main\n# branch.ab 2 1"},
		{name: "extra count", out: "# branch.head main\n# branch.ab +2 -1 +3"},
		{name: "negative ahead", out: "# branch.head main\n# branch.ab +-2 -1"},
		{name: "negative behind", out: "# branch.head main\n# branch.ab +2 --1"},
		{name: "double plus", out: "# branch.head main\n# branch.ab ++2 -1"},
		{name: "overflow", out: "# branch.head main\n# branch.ab +999999999999999999999 -1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseStatus([]byte(tt.out))
			if !errors.Is(err, ErrInvalidOutput) {
				t.Fatalf("ParseStatus() error = %v, want ErrInvalidOutput", err)
			}
		})
	}
}
