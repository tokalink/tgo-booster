package cb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	iconCacheMu sync.RWMutex
	iconCache   = make(map[string]template.HTML, 256)
)

const maxIconCacheEntries = 1024

var iconAllowedElements = map[string]string{
	"svg":            "svg",
	"g":              "g",
	"path":           "path",
	"circle":         "circle",
	"ellipse":        "ellipse",
	"line":           "line",
	"polyline":       "polyline",
	"polygon":        "polygon",
	"rect":           "rect",
	"defs":           "defs",
	"lineargradient": "linearGradient",
	"radialgradient": "radialGradient",
	"stop":           "stop",
	"title":          "title",
	"desc":           "desc",
	"use":            "use",
	"i":              "i",
	"span":           "span",
	"img":            "img",
}

var iconDropWithContent = map[string]bool{
	"script":        true,
	"style":         true,
	"foreignobject": true,
	"iframe":        true,
	"object":        true,
	"embed":         true,
}

var iconSelfClosingSVGElements = map[string]bool{
	"path":     true,
	"circle":   true,
	"ellipse":  true,
	"line":     true,
	"polyline": true,
	"polygon":  true,
	"rect":     true,
	"stop":     true,
	"use":      true,
}

var iconAllowedAttrs = map[string]string{
	"class":             "class",
	"width":             "width",
	"height":            "height",
	"viewbox":           "viewBox",
	"fill":              "fill",
	"stroke":            "stroke",
	"stroke-width":      "stroke-width",
	"stroke-linecap":    "stroke-linecap",
	"stroke-linejoin":   "stroke-linejoin",
	"stroke-dasharray":  "stroke-dasharray",
	"stroke-dashoffset": "stroke-dashoffset",
	"stroke-miterlimit": "stroke-miterlimit",
	"stroke-opacity":    "stroke-opacity",
	"fill-opacity":      "fill-opacity",
	"fill-rule":         "fill-rule",
	"clip-rule":         "clip-rule",
	"opacity":           "opacity",
	"d":                 "d",
	"cx":                "cx",
	"cy":                "cy",
	"r":                 "r",
	"rx":                "rx",
	"ry":                "ry",
	"x":                 "x",
	"y":                 "y",
	"x1":                "x1",
	"y1":                "y1",
	"x2":                "x2",
	"y2":                "y2",
	"points":            "points",
	"transform":         "transform",
	"xmlns":             "xmlns",
	"offset":            "offset",
	"stop-color":        "stop-color",
	"stop-opacity":      "stop-opacity",
	"gradientunits":     "gradientUnits",
	"gradienttransform": "gradientTransform",
	"id":                "id",
	"aria-hidden":       "aria-hidden",
	"role":              "role",
	"focusable":         "focusable",
	"alt":               "alt",
}

var richAllowedElements = map[string]bool{
	"p":          true,
	"br":         true,
	"b":          true,
	"strong":     true,
	"i":          true,
	"em":         true,
	"u":          true,
	"s":          true,
	"strike":     true,
	"del":        true,
	"sub":        true,
	"sup":        true,
	"h1":         true,
	"h2":         true,
	"h3":         true,
	"h4":         true,
	"h5":         true,
	"h6":         true,
	"ul":         true,
	"ol":         true,
	"li":         true,
	"blockquote": true,
	"pre":        true,
	"code":       true,
	"span":       true,
	"div":        true,
	"hr":         true,
	"a":          true,
	"img":        true,
	"table":      true,
	"thead":      true,
	"tbody":      true,
	"tfoot":      true,
	"tr":         true,
	"th":         true,
	"td":         true,
	"figure":     true,
	"figcaption": true,
}

var richDropWithContent = map[string]bool{
	"script":   true,
	"style":    true,
	"iframe":   true,
	"object":   true,
	"embed":    true,
	"form":     true,
	"input":    true,
	"button":   true,
	"textarea": true,
	"select":   true,
	"svg":      true,
	"math":     true,
	"template": true,
	"noscript": true,
}

var richVoidElements = map[string]bool{
	"br":  true,
	"hr":  true,
	"img": true,
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func isSafeImageSrc(raw string) bool {
	v := strings.TrimSpace(raw)
	if v == "" || hasControlChars(v) {
		return false
	}
	lower := strings.ToLower(v)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return true
	}
	if strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") {
		return true
	}
	for _, prefix := range []string{
		"data:image/png;base64,",
		"data:image/jpeg;base64,",
		"data:image/gif;base64,",
		"data:image/webp;base64,",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func isSafeAnchorHref(raw string) bool {
	v := strings.TrimSpace(raw)
	if v == "" || hasControlChars(v) {
		return false
	}
	lower := strings.ToLower(v)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "mailto:") {
		return true
	}
	if strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") {
		return true
	}
	if strings.HasPrefix(v, "#") {
		return true
	}
	return false
}

// SanitizeIcon sanitizes an icon string (SVG, icon tag, or emoji/text) using a strict HTML/SVG whitelist.
func SanitizeIcon(s string) template.HTML {
	if s == "" {
		return ""
	}

	iconCacheMu.RLock()
	if cached, ok := iconCache[s]; ok {
		iconCacheMu.RUnlock()
		return cached
	}
	iconCacheMu.RUnlock()

	res := sanitizeIconUncached(s)

	iconCacheMu.Lock()
	if len(iconCache) >= maxIconCacheEntries {
		iconCache = make(map[string]template.HTML, 256)
	}
	iconCache[s] = res
	iconCacheMu.Unlock()

	return res
}

func sanitizeIconUncached(s string) template.HTML {
	if !strings.Contains(s, "<") {
		return template.HTML(template.HTMLEscapeString(s))
	}

	ctxNode := &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	}
	nodes, err := html.ParseFragment(strings.NewReader(s), ctxNode)
	if err != nil {
		return template.HTML(template.HTMLEscapeString(s))
	}

	var b strings.Builder
	for _, n := range nodes {
		renderSanitizedIconNode(&b, n)
	}
	return template.HTML(b.String())
}

func renderSanitizedIconNode(b *strings.Builder, n *html.Node) {
	if n == nil {
		return
	}
	switch n.Type {
	case html.TextNode:
		b.WriteString(html.EscapeString(n.Data))
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderSanitizedIconNode(b, c)
		}
	case html.ElementNode:
		tag := strings.ToLower(strings.TrimSpace(n.Data))
		if iconDropWithContent[tag] {
			return
		}
		canonicalTag, ok := iconAllowedElements[tag]
		if !ok {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				renderSanitizedIconNode(b, c)
			}
			return
		}

		b.WriteByte('<')
		b.WriteString(canonicalTag)

		seenAttrs := make(map[string]bool, len(n.Attr))
		for _, a := range n.Attr {
			rawKey := strings.ToLower(strings.TrimSpace(a.Key))
			if a.Namespace != "" {
				rawKey = strings.ToLower(strings.TrimSpace(a.Namespace)) + ":" + rawKey
			}
			if rawKey == "" || strings.HasPrefix(rawKey, "on") || rawKey == "style" {
				continue
			}

			var outKey string
			var outVal string

			if rawKey == "href" || rawKey == "xlink:href" {
				if tag != "use" {
					continue
				}
				trimmedVal := strings.TrimSpace(a.Val)
				if !strings.HasPrefix(trimmedVal, "#") || hasControlChars(trimmedVal) {
					continue
				}
				outKey = rawKey
				outVal = trimmedVal
			} else if rawKey == "src" {
				if tag != "img" || !isSafeImageSrc(a.Val) {
					continue
				}
				outKey = "src"
				outVal = strings.TrimSpace(a.Val)
			} else {
				mappedKey, allowed := iconAllowedAttrs[rawKey]
				if !allowed {
					continue
				}
				outKey = mappedKey
				outVal = a.Val
			}

			dedupKey := strings.ToLower(outKey)
			if seenAttrs[dedupKey] {
				continue
			}
			seenAttrs[dedupKey] = true

			b.WriteByte(' ')
			b.WriteString(outKey)
			b.WriteString(`="`)
			b.WriteString(html.EscapeString(outVal))
			b.WriteByte('"')
		}

		if tag == "img" {
			b.WriteByte('>')
			return
		}
		if iconSelfClosingSVGElements[tag] && n.FirstChild == nil {
			b.WriteString("/>")
			return
		}

		b.WriteByte('>')
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderSanitizedIconNode(b, c)
		}
		b.WriteString("</")
		b.WriteString(canonicalTag)
		b.WriteByte('>')
	}
}

// SanitizeRichHTML sanitizes rich-text / WYSIWYG HTML content using a strict element and attribute whitelist.
func SanitizeRichHTML(s string) template.HTML {
	if s == "" {
		return ""
	}

	ctxNode := &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	}
	nodes, err := html.ParseFragment(strings.NewReader(s), ctxNode)
	if err != nil {
		return template.HTML(template.HTMLEscapeString(s))
	}

	var b strings.Builder
	for _, n := range nodes {
		renderSanitizedRichNode(&b, n)
	}
	return template.HTML(b.String())
}

func renderSanitizedRichNode(b *strings.Builder, n *html.Node) {
	if n == nil {
		return
	}
	switch n.Type {
	case html.TextNode:
		b.WriteString(html.EscapeString(n.Data))
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderSanitizedRichNode(b, c)
		}
	case html.ElementNode:
		tag := strings.ToLower(strings.TrimSpace(n.Data))
		if richDropWithContent[tag] {
			return
		}
		if !richAllowedElements[tag] {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				renderSanitizedRichNode(b, c)
			}
			return
		}

		b.WriteByte('<')
		b.WriteString(tag)

		seenAttrs := make(map[string]bool, len(n.Attr)+1)
		for _, a := range n.Attr {
			if a.Namespace != "" {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(a.Key))
			if key == "" || strings.HasPrefix(key, "on") || key == "style" || key == "id" || key == "rel" {
				continue
			}

			allowed := false
			outVal := a.Val

			switch key {
			case "title", "class":
				allowed = true
			case "href":
				if tag == "a" && isSafeAnchorHref(a.Val) {
					allowed = true
					outVal = strings.TrimSpace(a.Val)
				}
			case "target":
				if tag == "a" && strings.TrimSpace(a.Val) == "_blank" {
					allowed = true
					outVal = "_blank"
				}
			case "src":
				if tag == "img" && isSafeImageSrc(a.Val) {
					allowed = true
					outVal = strings.TrimSpace(a.Val)
				}
			case "alt", "width", "height":
				if tag == "img" {
					allowed = true
				}
			case "colspan", "rowspan":
				if tag == "td" || tag == "th" {
					allowed = true
				}
			}

			if !allowed || seenAttrs[key] {
				continue
			}
			seenAttrs[key] = true

			b.WriteByte(' ')
			b.WriteString(key)
			b.WriteString(`="`)
			b.WriteString(html.EscapeString(outVal))
			b.WriteByte('"')
		}

		if tag == "a" {
			b.WriteString(` rel="noopener noreferrer"`)
		}

		if richVoidElements[tag] {
			b.WriteByte('>')
			return
		}

		b.WriteByte('>')
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderSanitizedRichNode(b, c)
		}
		b.WriteString("</")
		b.WriteString(tag)
		b.WriteByte('>')
	}
}

// isChecked evaluates whether a checkbox value represents a truthy/checked state.
func isChecked(v interface{}) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if raw, ok := v.([]byte); ok {
		v = string(raw)
	}
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(v))) {
	case "1", "true", "t", "yes", "y", "on", "active", "aktif":
		return true
	default:
		return false
	}
}

// sanitizeSchemaJSON validates JSON-LD input and escapes HTML/script-breaking characters via json.HTMLEscape.
func sanitizeSchemaJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return ""
	}
	var buf bytes.Buffer
	json.HTMLEscape(&buf, []byte(trimmed))
	return buf.String()
}
