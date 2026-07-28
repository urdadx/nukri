package fileinfo

import (
	"github.com/urdadx/nukri/internal/core"
	"github.com/urdadx/nukri/internal/preview/code/registry"
)

type exactNameFacts struct {
	class core.FileClass
	label string
}

var exactNames = map[string]exactNameFacts{
	"pkgbuild":            {core.FileClassConfig, "Arch build script"},
	"makefile":            {core.FileClassConfig, "Makefile"},
	"gnumakefile":         {core.FileClassConfig, "Makefile"},
	"bsdmakefile":         {core.FileClassConfig, "Makefile"},
	"cmakelists.txt":      {core.FileClassConfig, "CMake project"},
	"dockerfile":          {core.FileClassConfig, "Docker build file"},
	"containerfile":       {core.FileClassConfig, "Docker build file"},
	"terraform.rc":        {core.FileClassConfig, "Terraform CLI config"},
	".terraformrc":        {core.FileClassConfig, "Terraform CLI config"},
	".terraform.lock.hcl": {core.FileClassData, "Terraform lockfile"},
	"build.gradle":        {core.FileClassConfig, "Gradle build script"},
	"settings.gradle":     {core.FileClassConfig, "Gradle build script"},
	"init.gradle":         {core.FileClassConfig, "Gradle build script"},
	"build.sbt":           {core.FileClassConfig, "sbt build definition"},
	"project.clj":         {core.FileClassConfig, "Leiningen project"},
	"deps.edn":            {core.FileClassConfig, "Clojure deps config"},
	"bb.edn":              {core.FileClassConfig, "Babashka config"},
	"shadow-cljs.edn":     {core.FileClassConfig, "shadow-cljs config"},
	"justfile":            {core.FileClassConfig, "Justfile"},
	".justfile":           {core.FileClassConfig, "Justfile"},
	".rprofile":           {core.FileClassConfig, "R profile"},
	".bashrc":             {core.FileClassConfig, "Bash config"},
	".bash_profile":       {core.FileClassConfig, "Bash config"},
	".bash_login":         {core.FileClassConfig, "Bash config"},
	".bash_logout":        {core.FileClassConfig, "Bash config"},
	".bash_aliases":       {core.FileClassConfig, "Bash config"},
	".profile":            {core.FileClassConfig, "Shell config"},
	".xprofile":           {core.FileClassConfig, "Shell config"},
	".xsessionrc":         {core.FileClassConfig, "Shell config"},
	".envrc":              {core.FileClassConfig, "Shell config"},
	".zshrc":              {core.FileClassConfig, "Zsh config"},
	".zprofile":           {core.FileClassConfig, "Zsh config"},
	".zshenv":             {core.FileClassConfig, "Zsh config"},
	".zlogin":             {core.FileClassConfig, "Zsh config"},
	".zlogout":            {core.FileClassConfig, "Zsh config"},
	".kshrc":              {core.FileClassConfig, "KornShell config"},
	".mkshrc":             {core.FileClassConfig, "KornShell config"},
	"cargo.lock":          {core.FileClassData, ""},
	"poetry.lock":         {core.FileClassData, ""},
	"uv.lock":             {core.FileClassData, "Lockfile"},
	"package.json":        {core.FileClassConfig, ""},
	"tsconfig.json":       {core.FileClassConfig, ""},
	"deno.json":           {core.FileClassConfig, ""},
	"package-lock.json":   {core.FileClassData, ""},
	"composer.lock":       {core.FileClassData, "Lockfile"},
	"pipfile.lock":        {core.FileClassData, "Lockfile"},
	"flake.lock":          {core.FileClassData, "Lockfile"},
	"gemfile.lock":        {core.FileClassData, "Lockfile"},
	"bun.lock":            {core.FileClassData, "Lockfile"},
	"deno.jsonc":          {core.FileClassConfig, "JSON with comments"},
	"compose.yml":         {core.FileClassConfig, ""},
	"compose.yaml":        {core.FileClassConfig, ""},
	"docker-compose.yml":  {core.FileClassConfig, ""},
	"docker-compose.yaml": {core.FileClassConfig, ""},
	"pnpm-lock.yaml":      {core.FileClassConfig, ""},
	"pnpm-workspace.yaml": {core.FileClassConfig, ""},
}

func inspectExactName(name string) (FileFacts, bool) {
	name = normalizeKey(name)
	metadata, ok := exactNames[name]
	if !ok {
		if !isEnvName(name) {
			return FileFacts{}, false
		}
		metadata = exactNameFacts{core.FileClassConfig, "Environment file"}
	}
	language, ok := registry.LanguageForExactFilename(name)
	if !ok {
		return FileFacts{}, false
	}
	return facts(metadata.class, metadata.label, previewForLanguage(language)), true
}

func isEnvName(name string) bool {
	language, ok := registry.LanguageForExactFilename(name)
	return ok && language.CanonicalID == "dotenv"
}
