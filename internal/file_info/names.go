package fileinfo

import (
	"strings"

	"github.com/urdadx/nukri/internal/core"
)

func previewForExactName(name string) PreviewSpec {
	name = normalizeKey(name)
	switch name {
	case "pkgbuild", ".bashrc", ".bash_profile", ".bash_login", ".bash_logout", ".bash_aliases":
		return CodePreview("bash", Chroma, nil)
	case ".profile", ".xprofile", ".xsessionrc", ".envrc":
		return CodePreview("sh", Chroma, nil)
	case ".zshrc", ".zprofile", ".zshenv", ".zlogin", ".zlogout":
		return CodePreview("zsh", Chroma, nil)
	case ".kshrc", ".mkshrc":
		return CodePreview("ksh", Chroma, nil)
	case "makefile", "gnumakefile", "bsdmakefile":
		return CodePreview("make", Chroma, nil)
	case "cmakelists.txt":
		return CodePreview("cmake", Chroma, nil)
	case "dockerfile", "containerfile":
		return CodePreview("dockerfile", Chroma, nil)
	case "terraform.rc", ".terraformrc":
		return CodePreview("terraform", Chroma, nil)
	case ".terraform.lock.hcl":
		return CodePreview("hcl", Chroma, nil)
	case "build.gradle", "settings.gradle", "init.gradle":
		return CodePreview("groovy", Chroma, nil)
	case "build.sbt":
		return CodePreview("scala", Chroma, nil)
	case "project.clj", "deps.edn", "bb.edn", "shadow-cljs.edn":
		return CodePreview("clojure", Chroma, nil)
	case "justfile", ".justfile":
		return CodePreview("just", Chroma, nil)
	case ".rprofile":
		return CodePreview("r", Chroma, nil)
	case "cargo.lock", "poetry.lock", "uv.lock":
		return previewForExtension("toml")
	case "package.json", "tsconfig.json", "deno.json", "package-lock.json", "composer.lock", "pipfile.lock", "flake.lock":
		return previewForExtension("json")
	case "deno.jsonc":
		return previewForExtension("jsonc")
	case "compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml", "pnpm-lock.yaml", "pnpm-workspace.yaml":
		return previewForExtension("yaml")
	case "gemfile.lock", "bun.lock":
		return CodePreview("config", Custom, nil)
	default:
		if isEnvName(name) {
			return CodePreview("config", Custom, nil)
		}
	}
	panic("exact-name registry entry should exist for code preview")
}

func inspectExactName(name string) (FileFacts, bool) {
	name = normalizeKey(name)
	class := core.FileClassConfig
	label := ""

	switch name {
	case "pkgbuild":
		label = "Arch build script"
	case "makefile", "gnumakefile", "bsdmakefile":
		label = "Makefile"
	case "cmakelists.txt":
		label = "CMake project"
	case "dockerfile", "containerfile":
		label = "Docker build file"
	case "terraform.rc", ".terraformrc":
		label = "Terraform CLI config"
	case ".terraform.lock.hcl":
		class, label = core.FileClassData, "Terraform lockfile"
	case "build.gradle", "settings.gradle", "init.gradle":
		label = "Gradle build script"
	case "build.sbt":
		label = "sbt build definition"
	case "project.clj":
		label = "Leiningen project"
	case "deps.edn":
		label = "Clojure deps config"
	case "bb.edn":
		label = "Babashka config"
	case "shadow-cljs.edn":
		label = "shadow-cljs config"
	case "justfile", ".justfile":
		label = "Justfile"
	case ".rprofile":
		label = "R profile"
	case ".bashrc", ".bash_profile", ".bash_login", ".bash_logout", ".bash_aliases":
		label = "Bash config"
	case ".profile", ".xprofile", ".xsessionrc", ".envrc":
		label = "Shell config"
	case ".zshrc", ".zprofile", ".zshenv", ".zlogin", ".zlogout":
		label = "Zsh config"
	case ".kshrc", ".mkshrc":
		label = "KornShell config"
	case "cargo.lock", "poetry.lock", "package-lock.json", "package.json", "tsconfig.json", "deno.json", "compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml", "pnpm-lock.yaml", "pnpm-workspace.yaml":
		if name == "cargo.lock" || name == "poetry.lock" || name == "package-lock.json" {
			class = core.FileClassData
		}
	case "uv.lock", "composer.lock", "pipfile.lock", "flake.lock", "gemfile.lock", "bun.lock":
		class, label = core.FileClassData, "Lockfile"
	case "deno.jsonc":
		label = "JSON with comments"
	default:
		if !isEnvName(name) {
			return FileFacts{}, false
		}
		label = "Environment file"
	}

	return facts(class, label, previewForExactName(name)), true
}

func isEnvName(name string) bool {
	return strings.HasPrefix(name, ".env") || strings.HasPrefix(name, "env.")
}
