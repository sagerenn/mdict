package stardict

import "strings"

func isImageFile(name string) bool {
	s := simplifyName(name)
	return hasAnySuffix(s, ".jpg", ".jpeg", ".jpe", ".png", ".gif", ".bmp", ".tif", ".tiff", ".tga", ".pcx", ".ico", ".webp", ".svg")
}

func isSoundFile(name string) bool {
	s := simplifyName(name)
	return hasAnySuffix(s, ".wav", ".au", ".voc", ".ogg", ".oga", ".mp3", ".m4a", ".aac", ".flac", ".mid", ".kar",
		".mpc", ".wma", ".wv", ".ape", ".spx", ".opus", ".mpa", ".mp2")
}

func simplifyName(name string) string {
	name = strings.TrimSpace(name)
	return strings.ToLower(name)
}

func isCSSFile(name string) bool {
	s := simplifyName(name)
	return strings.HasSuffix(s, ".css")
}

func hasAnySuffix(s string, suffixes ...string) bool {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}
