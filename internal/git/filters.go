package git

import (
	"strconv"
	"strings"
)

// filterConfigPattern matches every filter driver setting in any config scope.
const filterConfigPattern = `^filter\.`

// filterDriverKeys are the driver settings neutralised during inspection.
// "clean" and "process" are what `git status` may execute when it re-reads a
// stat-dirty tracked file; "smudge" is cleared for completeness and "required"
// is relaxed so the disabled drivers do not turn into errors.
var filterDriverKeys = []struct{ key, value string }{
	{"clean", ""},
	{"smudge", ""},
	{"process", ""},
	{"required", "false"},
}

// trustedConfigScopes are config scopes owned by the user or the system
// administrator rather than by the inspected repository. Filter drivers from
// these scopes (e.g. a globally installed git-lfs) already run in the user's
// own Git commands, so they stay enabled and keep the dirty state accurate.
var trustedConfigScopes = map[string]struct{}{"global": {}, "system": {}}

// untrustedFilterDrivers extracts unique driver names that are (re)defined by
// the repository itself (local, worktree or any other non-trusted scope) from
// the output of `git config --show-scope --name-only --get-regexp ^filter\.`
// (lines like "local\tfilter.lfs.clean"). Driver names may contain dots, so
// the variable is cut at the last dot; the empty name ("filter..clean", from
// [filter ""]) is a valid driver too.
func untrustedFilterDrivers(out []byte) []string {
	seen := make(map[string]struct{})
	var names []string
	for line := range strings.SplitSeq(string(out), "\n") {
		scope, entry, found := strings.Cut(strings.TrimSuffix(line, "\r"), "\t")
		if !found {
			entry = scope
			scope = ""
		}
		if _, trusted := trustedConfigScopes[scope]; trusted {
			continue
		}
		key, ok := strings.CutPrefix(entry, "filter.")
		if !ok {
			continue
		}
		dot := strings.LastIndexByte(key, '.')
		if dot < 0 {
			continue
		}
		name := key[:dot]
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

// filterOverrideEnv returns GIT_CONFIG_COUNT/KEY/VALUE entries that disable the
// named filter drivers for one git invocation. The environment form is used
// instead of "-c key=value" because driver names may themselves contain "=".
func filterOverrideEnv(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	env := make([]string, 0, 1+2*len(names)*len(filterDriverKeys))
	n := 0
	for _, name := range names {
		for _, setting := range filterDriverKeys {
			index := strconv.Itoa(n)
			env = append(env,
				"GIT_CONFIG_KEY_"+index+"=filter."+name+"."+setting.key,
				"GIT_CONFIG_VALUE_"+index+"="+setting.value,
			)
			n++
		}
	}
	return append(env, "GIT_CONFIG_COUNT="+strconv.Itoa(n))
}
