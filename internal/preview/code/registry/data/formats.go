package data

import registry "github.com/urdadx/nukri/internal/preview/code/registry/model"

var Formats = []registry.RegistryEntry{
	registry.Entry(
		registry.Language("json", "JSON", registry.Custom, structured(registry.StructuredJSON)),
		[]string{"json"},
		[]string{"package.json", "tsconfig.json", "deno.json", "package-lock.json", "composer.lock", "pipfile.lock", "flake.lock"},
		nil,
		[]string{"json"},
		[]string{"json"},
	),
	registry.Entry(
		registry.Language("jsonc", "JSON with comments", registry.Custom, structured(registry.StructuredJSONC)),
		[]string{"jsonc"},
		[]string{"deno.jsonc"},
		nil,
		[]string{"jsonc"},
		[]string{"jsonc"},
	),
	registry.Entry(
		registry.Language("json5", "JSON5", registry.Custom, structured(registry.StructuredJSON5)),
		[]string{"json5"}, nil, nil, []string{"json5"}, []string{"json5"},
	),
	registry.Entry(
		registry.Language("toml", "TOML", registry.Custom, structured(registry.StructuredTOML)),
		[]string{"toml"},
		[]string{"cargo.lock", "poetry.lock", "uv.lock"},
		nil,
		[]string{"toml"},
		[]string{"toml"},
	),
	registry.Entry(
		registry.Language("yaml", "YAML", registry.Custom, structured(registry.StructuredYAML)),
		[]string{"yaml", "yml"},
		[]string{"compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml", "pnpm-lock.yaml", "pnpm-workspace.yaml"},
		nil,
		[]string{"yaml", "yml"},
		[]string{"yaml", "yml"},
	),
	registry.Entry(
		registry.Language("ini", "INI", registry.Custom, nil),
		[]string{"ini", "keys", "desktop"}, nil, nil,
		[]string{"ini", "dosini"}, []string{"ini"},
	),
	registry.Entry(
		registry.Language("config", "Config", registry.Custom, nil),
		[]string{"conf", "cfg", "env", "lock"},
		[]string{"gemfile.lock", "bun.lock"},
		nil,
		[]string{"config", "conf", "cfg"},
		[]string{"config"},
	),
	registry.Entry(
		registry.Language("dotenv", "Environment", registry.Custom, nil),
		nil, nil, nil, []string{"dotenv"}, []string{"dotenv"},
	),
	registry.Entry(
		registry.Language("log", "Log", registry.Chroma, nil),
		[]string{"log"}, nil, nil, []string{"log"}, []string{"log"},
	),
}

func structured(format registry.StructuredFormat) *registry.StructuredFormat {
	return &format
}
