package data

import (
	registry "github.com/urdadx/nukri/internal/preview/code/registry/model"
)

var Web = []registry.RegistryEntry{
	registry.Entry(
		registry.Language("html", "HTML", registry.Chroma, nil),
		[]string{"html", "htm", "xhtml"},
		nil,
		nil,
		[]string{"html"},
		[]string{"html"},
	),
	registry.Entry(
		registry.Language("xml", "XML", registry.Chroma, nil),
		[]string{"xml", "xsd", "xsl", "xslt", "svg"},
		nil,
		nil,
		[]string{"xml", "svg", "markup"},
		[]string{"xml", "xhtml", "svg", "markup"},
	),
	registry.Entry(
		registry.Language("css", "CSS", registry.Chroma, nil),
		[]string{"css"},
		nil,
		nil,
		[]string{"css"},
		[]string{"css"},
	),
	registry.Entry(
		registry.Language("scss", "SCSS", registry.Chroma, nil),
		[]string{"scss"},
		nil,
		nil,
		[]string{"scss"},
		[]string{"scss"},
	),
	registry.Entry(
		registry.Language("sass", "Sass", registry.Chroma, nil),
		[]string{"sass"},
		nil,
		nil,
		[]string{"sass"},
		[]string{"sass"},
	),
	registry.Entry(
		registry.Language("less", "Less", registry.Chroma, nil),
		[]string{"less"},
		nil,
		nil,
		[]string{"less"},
		[]string{"less"},
	),
	registry.Entry(
		registry.Language("javascript", "JavaScript", registry.Chroma, nil),
		[]string{"js", "mjs", "cjs"},
		nil,
		nil,
		[]string{"javascript"},
		[]string{"js", "javascript"},
	),
	registry.Entry(
		registry.Language("jsx", "JSX", registry.Chroma, nil),
		[]string{"jsx"},
		nil,
		nil,
		[]string{"jsx"},
		[]string{"jsx"},
	),
	registry.Entry(
		registry.Language("typescript", "TypeScript", registry.Chroma, nil),
		[]string{"ts", "mts", "cts"},
		nil,
		nil,
		[]string{"typescript"},
		[]string{"ts", "typescript"},
	),
	registry.Entry(
		registry.Language("tsx", "TSX", registry.Chroma, nil),
		[]string{"tsx"},
		nil,
		nil,
		[]string{"tsx"},
		[]string{"tsx"},
	),
	registry.Entry(
		registry.Language("qml", "QML", registry.Chroma, nil),
		[]string{"qml"},
		nil,
		nil,
		[]string{"qml"},
		[]string{"qml"},
	),
}
