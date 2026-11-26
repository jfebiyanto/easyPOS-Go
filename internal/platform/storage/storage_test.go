package storage

import "testing"

func TestJoin(t *testing.T) {
	root := t.TempDir()
	for rel, ok := range map[string]bool{
		"products/a.png": true, "a.png": true, "/products/a.png": true,
		"": false, "/": false, "..": false, "../x": true, "products/../../x": true, // dibersihkan dari "/" sehingga tetap di dalam root

		`products\..\..\x`: false, "C:/x": false, "a\x00.png": false,
	} {
		if _, got := Join(root, rel); got != ok {
			t.Errorf("Join(%q) = %v, want %v", rel, got, ok)
		}
	}
	a, _ := Join(root, "a.png")
	if b, _ := Join(root, "../../a.png"); a != b {
		t.Errorf("../../a.png harus jatuh ke root: %q != %q", b, a)
	}
}
