package semver

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		input                      string
		major, minor, patch, build int
		prerelease                 string
		wantErr                    bool
	}{
		{input: "1.2.3", major: 1, minor: 2, patch: 3},
		{input: "v1.2.3", major: 1, minor: 2, patch: 3},
		{input: "1.1.26245.26815", major: 1, minor: 1, patch: 26245, build: 26815},
		{input: "1.0.0-beta.1", major: 1, prerelease: "beta.1"},
		{input: "2.0.0+build.5", major: 2},
		{input: "", wantErr: true},
		{input: "abc", wantErr: true},
		{input: "1.2.3.4.5", wantErr: true},
		{input: "1..3", wantErr: true},
	}
	for _, c := range cases {
		got, err := Parse(c.input)
		if c.wantErr {
			if err == nil {
				t.Errorf("Parse(%q) 应报错", c.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q) 意外失败: %v", c.input, err)
			continue
		}
		if got.Major != c.major || got.Minor != c.minor || got.Patch != c.patch || got.Build != c.build {
			t.Errorf("Parse(%q) = %d.%d.%d.%d，期望 %d.%d.%d.%d",
				c.input, got.Major, got.Minor, got.Patch, got.Build, c.major, c.minor, c.patch, c.build)
		}
		if got.Prerelease != c.prerelease {
			t.Errorf("Parse(%q) 预发布标识 = %q，期望 %q", c.input, got.Prerelease, c.prerelease)
		}
	}
}

func TestCompareUsesNumericOrder(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.10.0", "1.2.3", 1},   // 字符串比较会误判，必须按数值
		{"1.1.26245.26815", "1.1.26181.28434", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "1.0.0", 0},
		{"1.0.0-beta", "1.0.0", -1}, // 预发布低于正式版
		{"1.0.1", "1.0.0", 1},
	}
	for _, c := range cases {
		got := Compare(MustParse(c.a), MustParse(c.b))
		if got != c.want {
			t.Errorf("Compare(%s, %s) = %d，期望 %d", c.a, c.b, got, c.want)
		}
	}
}
