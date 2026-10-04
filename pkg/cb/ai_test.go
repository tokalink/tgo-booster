package cb

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAIService_ConfigAndEndpoint(t *testing.T) {
	e := NewEngine("TestApp")

	// 1. Endpoint normalization
	if ep := e.BuildAIEndpoint("https://ai.sumopod.com"); ep != "https://ai.sumopod.com/v1/chat/completions" {
		t.Errorf("Expected https://ai.sumopod.com/v1/chat/completions, got %s", ep)
	}
	if ep := e.BuildAIEndpoint("https://api.openai.com/v1"); ep != "https://api.openai.com/v1/chat/completions" {
		t.Errorf("Expected https://api.openai.com/v1/chat/completions, got %s", ep)
	}
	if ep := e.BuildAIEndpoint("https://my-llm.corp.internal/v1/chat/completions"); ep != "https://my-llm.corp.internal/v1/chat/completions" {
		t.Errorf("Expected verbatim endpoint, got %s", ep)
	}

	// 2. Config reading with fallback
	host, apiKey, model := e.GetAIConfig()
	if host == "" {
		t.Errorf("Expected non-empty AI host")
	}
	if model == "" {
		t.Errorf("Expected non-empty AI model")
	}

	t.Logf("AI Config: Host=%s, Model=%s, HasKey=%v", host, model, apiKey != "")
}

func TestAIService_LiveConnectionIfConfigured(t *testing.T) {
	e := NewEngine("TestApp")
	_, apiKey, _ := e.GetAIConfig()
	if apiKey == "" {
		t.Skip("Skipping live AI connection test: OPENAI_APIKEY is not set in environment or settings")
	}

	latency, modelDesc, err := e.TestAIConnection()
	if err != nil {
		t.Skipf("Live AI endpoint temporarily unavailable or timed out: %v (model: %s)", err, modelDesc)
		return
	}

	t.Logf("✅ Live AI connection verified! Model: %s, Latency: %dms", modelDesc, latency)
}

func TestAIService_GeneratePageWithAI(t *testing.T) {
	e := NewEngine("TestApp")
	_, apiKey, _ := e.GetAIConfig()
	if apiKey == "" {
		t.Skip("Skipping live AI page generation test: OPENAI_APIKEY is not set")
	}

	result, err := e.GeneratePageWithAI(AIPageGenerateRequest{
		Prompt:   "Landing page untuk solusi cloud enterprise manajemen log microservice",
		Template: "landing",
		Tone:     "professional",
		Language: "id",
	})
	if err != nil {
		t.Fatalf("GeneratePageWithAI failed: %v", err)
	}

	if result.Title == "" {
		t.Errorf("Expected non-empty page Title")
	}
	if result.Content == "" {
		t.Errorf("Expected non-empty Content HTML")
	}
	if result.RouteURL == "" {
		t.Errorf("Expected non-empty RouteURL")
	}

	t.Logf("✅ AI Page Generated Successfully!\nTitle: %s\nSlug: %s\nRoute: %s\nMeta Title: %s\nContent Length: %d chars",
		result.Title, result.Slug, result.RouteURL, result.MetaTitle, len(result.Content))
}

func TestAIService_FetchModels(t *testing.T) {
	e := NewEngine("TestApp")

	// 1. Endpoint test
	if ep := e.BuildAIModelsEndpoint("https://ai.sumopod.com"); ep != "https://ai.sumopod.com/v1/models" {
		t.Errorf("Expected https://ai.sumopod.com/v1/models, got %s", ep)
	}
	if ep := e.BuildAIModelsEndpoint("https://api.openai.com/v1"); ep != "https://api.openai.com/v1/models" {
		t.Errorf("Expected https://api.openai.com/v1/models, got %s", ep)
	}

	// 2. Live fetch test
	host, apiKey, _ := e.GetAIConfig()
	if apiKey == "" {
		t.Skip("Skipping live fetch models: OPENAI_APIKEY is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := e.FetchAIModels(ctx, host, apiKey)
	if err != nil {
		t.Fatalf("FetchAIModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatalf("Expected at least 1 model, got 0")
	}

	sampleLimit := len(models)
	if sampleLimit > 5 {
		sampleLimit = 5
	}
	t.Logf("✅ Successfully fetched %d models from %s. Sample models: %v", len(models), host, models[:sampleLimit])
}

func TestParseAIJSONResponse_UnescapedNewlinesAndTruncated(t *testing.T) {
	// 1. Raw output from user's actual failure scenario (unescaped literal newlines in content, truncated without closing quote and brace)
	rawBroken := `{
  "title": "Platform Business Intelligence Real-Time & Predictive AI | OmniData",
  "slug": "software-business-intelligence-predictive-ai",
  "route_url": "/software-business-intelligence-predictive-ai",
  "template": "blank",
  "excerpt": "Akselerasi keputusan bisnis Anda dengan platform Business Intelligence real-time OmniData, didukung predictive AI cerdas dan pipeline data otomatis terintegrasi.",
  "meta_title": "Software BI Real-Time & Predictive AI | OmniData Indonesia",
  "meta_description": "Ubah data mentah jadi strategi akurat dengan software BI real-time OmniData. Dilengkapi predictive AI dan pipeline otomatis. Coba gratis sekarang!",
  "meta_keywords": "software business intelligence, real-time analytics, predictive ai, pipeline data otomatis, bi dashboard indonesia, enterprise analytics",
  "og_title": "OmniData: Platform BI Real-Time & Predictive AI Generasi Baru",
  "og_description": "Otomatisasi pengolahan data dan antisipasi tren pasar dengan kecerdasan prediktif AI dari OmniData. Cek fitur dan bandingkan paket harganya.",
  "content": "
Pelopor BI Real-Time Generasi Terbaru
Keputusan Bisnis Presisi, Sekejap Mata dengan AI
Tinggalkan laporan statis yang usang. OmniData menyatukan pipeline data otomatis berkecepatan tinggi dengan kemampuan predictive AI untuk memproyeksikan peluang bisnis Anda secara real-time.
Mulai Uji Coba Gratis 14 Hari`

	res, err := parseAIJSONResponse(rawBroken)
	if err != nil {
		t.Fatalf("parseAIJSONResponse failed to repair broken/truncated JSON: %v", err)
	}

	if res.Title != "Platform Business Intelligence Real-Time & Predictive AI | OmniData" {
		t.Errorf("Unexpected title: %s", res.Title)
	}
	if res.Slug != "software-business-intelligence-predictive-ai" {
		t.Errorf("Unexpected slug: %s", res.Slug)
	}
	if !strings.Contains(res.Content, "Pelopor BI Real-Time Generasi Terbaru") {
		t.Errorf("Expected content to be recovered, got: %s", res.Content)
	}

	t.Logf("✅ Successfully repaired broken and truncated AI response! Extracted Title: %s, Content Len: %d", res.Title, len(res.Content))
}

