package cb

import (
	"strings"
	"testing"
)

func TestSanitizeIcon_WhitelistAndVectors(t *testing.T) {
	t.Chdir(t.TempDir())

	if got := string(SanitizeIcon("")); got != "" {
		t.Fatalf("expected empty string for empty icon, got %q", got)
	}

	// 1. SVG with onload, script, and onclick on path
	rawSVG := `<svg onload=alert(1)><script>x</script><path d="M1 1" onclick="y"/></svg>`
	gotSVG := string(SanitizeIcon(rawSVG))
	for _, forbidden := range []string{"onload", "script", "onclick", "alert"} {
		if strings.Contains(strings.ToLower(gotSVG), forbidden) {
			t.Fatalf("SanitizeIcon(%q) still contains %q: %s", rawSVG, forbidden, gotSVG)
		}
	}
	if !strings.Contains(gotSVG, `d="M1 1"`) || !strings.Contains(gotSVG, "<path") {
		t.Fatalf("SanitizeIcon(%q) lost allowed path/d attribute: %s", rawSVG, gotSVG)
	}

	// 2. <img src=javascript:alert(1)> -> src removed
	gotImg := string(SanitizeIcon(`<img src=javascript:alert(1) alt="ico">`))
	if strings.Contains(strings.ToLower(gotImg), "src") || strings.Contains(strings.ToLower(gotImg), "javascript") {
		t.Fatalf("SanitizeIcon img javascript src not stripped: %s", gotImg)
	}
	if !strings.Contains(gotImg, "<img") || !strings.Contains(gotImg, `alt="ico"`) {
		t.Fatalf("SanitizeIcon img tag or alt lost: %s", gotImg)
	}

	// Protocol-relative //evil.com/x.png must also be stripped
	if got := string(SanitizeIcon(`<img src="//evil.com/x.png">`)); strings.Contains(got, "src") {
		t.Fatalf("SanitizeIcon protocol-relative img src must be stripped: %s", got)
	}
	// Allowed data:image/png;base64, and /static/ok.png must be preserved
	if got := string(SanitizeIcon(`<img src="/static/ok.png">`)); !strings.Contains(got, `src="/static/ok.png"`) {
		t.Fatalf("SanitizeIcon relative img src lost: %s", got)
	}
	if got := string(SanitizeIcon(`<img src="data:image/png;base64,iVBORw0KGgo=">`)); !strings.Contains(got, `src="data:image/png;base64,iVBORw0KGgo="`) {
		t.Fatalf("SanitizeIcon base64 png img src lost: %s", got)
	}

	// 3. <use href="https://evil/x.svg#a"> -> href stripped; local #a preserved
	gotUseEvil := string(SanitizeIcon(`<svg><use href="https://evil/x.svg#a" xlink:href="https://evil/x.svg#b"/></svg>`))
	if strings.Contains(strings.ToLower(gotUseEvil), "href") || strings.Contains(gotUseEvil, "evil") {
		t.Fatalf("SanitizeIcon external use href not stripped: %s", gotUseEvil)
	}
	gotUseLocal := string(SanitizeIcon(`<svg><use href="#local-icon"/></svg>`))
	if !strings.Contains(gotUseLocal, `href="#local-icon"`) {
		t.Fatalf("SanitizeIcon local # href should be kept: %s", gotUseLocal)
	}

	// 4. Plain text / emoji without '<' -> HTML-escaped
	gotEmoji := string(SanitizeIcon(`🚀 & "Rockets"`))
	if gotEmoji != `🚀 &amp; &#34;Rockets&#34;` {
		t.Fatalf("unexpected escaped emoji/text: %q", gotEmoji)
	}

	// 5. Built-in Booster icon from engine.go (including duplicate x2 attribute & viewBox)
	builtinIcon := `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" x2="8" y1="13" x2="13"/><line x1="16" x2="8" y1="17" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>`
	gotBuiltin := string(SanitizeIcon(builtinIcon))
	if !strings.Contains(gotBuiltin, "viewBox=\"0 0 24 24\"") {
		t.Fatalf("expected viewBox casing preserved in built-in icon, got: %s", gotBuiltin)
	}
	if !strings.Contains(gotBuiltin, "<path") {
		t.Fatalf("expected <path in sanitized built-in icon, got: %s", gotBuiltin)
	}

	// Also verify linearGradient / radialGradient / gradientUnits / gradientTransform casing
	gradSVG := `<svg viewBox="0 0 10 10"><defs><linearGradient id="g1" gradientUnits="userSpaceOnUse" gradientTransform="rotate(45)"><stop offset="0%" stop-color="#fff"/></linearGradient><radialGradient id="g2"></radialGradient></defs></svg>`
	gotGrad := string(SanitizeIcon(gradSVG))
	for _, want := range []string{"linearGradient", "radialGradient", "gradientUnits", "gradientTransform"} {
		if !strings.Contains(gotGrad, want) {
			t.Fatalf("expected %q casing preserved, got: %s", want, gotGrad)
		}
	}
}

func TestSanitizeRichHTML_WhitelistAndVectors(t *testing.T) {
	t.Chdir(t.TempDir())

	// 1. <a href="javascript:alert(1)">x</a> -> without href
	gotJSLink := string(SanitizeRichHTML(`<a href="javascript:alert(1)">x</a>`))
	if strings.Contains(strings.ToLower(gotJSLink), "href") || strings.Contains(strings.ToLower(gotJSLink), "javascript") {
		t.Fatalf("expected javascript href to be stripped: %s", gotJSLink)
	}
	if !strings.Contains(gotJSLink, ">x</a>") {
		t.Fatalf("expected anchor text preserved: %s", gotJSLink)
	}

	// 2. <p onclick=x>ok</p> -> <p>ok</p>
	gotP := string(SanitizeRichHTML(`<p onclick=x style="color:red" id="p1">ok</p>`))
	if gotP != `<p>ok</p>` {
		t.Fatalf("expected <p>ok</p>, got %q", gotP)
	}

	// 3. <iframe> disappears along with its content, plus <script>, <svg>, <form>
	gotDrop := string(SanitizeRichHTML(`<p>safe</p><iframe src="https://evil.com">inside</iframe><script>alert(1)</script><svg onload=alert(1)></svg><form><input name="x"></form>`))
	if gotDrop != `<p>safe</p>` {
		t.Fatalf("expected dropped elements removed with contents, got %q", gotDrop)
	}

	// 4. <a href="https://a"> gets rel="noopener noreferrer"
	gotSafeLink := string(SanitizeRichHTML(`<a href="https://a" target="_blank">link</a>`))
	if !strings.Contains(gotSafeLink, `href="https://a"`) || !strings.Contains(gotSafeLink, `rel="noopener noreferrer"`) || !strings.Contains(gotSafeLink, `target="_blank"`) {
		t.Fatalf("expected href, target=_blank, and rel=\"noopener noreferrer\", got %q", gotSafeLink)
	}

	// Disallowed target value (e.g. target="evil") must be stripped
	gotBadTarget := string(SanitizeRichHTML(`<a href="/docs" target="evil">docs</a>`))
	if strings.Contains(gotBadTarget, "target=") || !strings.Contains(gotBadTarget, `href="/docs"`) || !strings.Contains(gotBadTarget, `rel="noopener noreferrer"`) {
		t.Fatalf("expected disallowed target stripped and rel added, got %q", gotBadTarget)
	}
}
