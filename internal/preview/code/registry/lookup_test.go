package registry

import "testing"

func TestRegistryIsValid(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLanguageLookups(t *testing.T) {
	tests := []struct {
		name string
		find func() (RegisteredLanguage, bool)
		want string
	}{
		{"extension", func() (RegisteredLanguage, bool) { return LanguageForExtension(" YML ") }, "yaml"},
		{"exact filename", func() (RegisteredLanguage, bool) { return LanguageForExactFilename("Package.JSON") }, "json"},
		{"shebang", func() (RegisteredLanguage, bool) { return LanguageForShebangInterpreter("PYTHON3") }, "python"},
		{"modeline", func() (RegisteredLanguage, bool) { return LanguageForModeline("shell-script") }, "sh"},
		{"fence", func() (RegisteredLanguage, bool) { return LanguageForMarkdownFence("golang") }, "go"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			language, ok := test.find()
			if !ok || language.CanonicalID != test.want {
				t.Fatalf("language = %#v, ok = %v, want %q", language, ok, test.want)
			}
		})
	}
}

func TestEnvironmentFilenameBoundaries(t *testing.T) {
	for _, name := range []string{".env", ".env.local", "env.production"} {
		language, ok := LanguageForExactFilename(name)
		if !ok || language.CanonicalID != "dotenv" {
			t.Errorf("%q did not resolve to dotenv", name)
		}
	}
	for _, name := range []string{".environment", ".envelope", "env.go"} {
		if _, ok := LanguageForExactFilename(name); ok {
			t.Errorf("%q should not resolve as an environment file", name)
		}
	}
}

func TestStructuredFormatMetadata(t *testing.T) {
	language, ok := LanguageForExtension("jsonc")
	if !ok || language.StructuredFormat == nil || *language.StructuredFormat != StructuredJSONC {
		t.Fatalf("language = %#v, want JSONC structured metadata", language)
	}
}
