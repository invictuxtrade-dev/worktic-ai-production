package main

import "testing"

func TestMeterV2726TokenCostsConfiguredOnly(t *testing.T) {
	t.Setenv("WORKTIC_METER_OPENAI_MODEL", "gpt-5-mini")
	t.Setenv("WORKTIC_METER_OPENAI_INPUT_MICROUSD_PER_MILLION", "250000")
	t.Setenv("WORKTIC_METER_OPENAI_OUTPUT_MICROUSD_PER_MILLION", "2000000")
	c := meterTokenPriceV2726("gpt-5-mini", 1000000, 1000000)
	if c == nil || *c != 2250000 {
		t.Fatalf("unexpected microUSD cost: %v", c)
	}
	if meterTokenPriceV2726("another-model", 1000000, 0) != nil {
		t.Fatal("different model must not borrow a rate")
	}
	t.Setenv("WORKTIC_METER_OPENAI_INPUT_MICROUSD_PER_MILLION", "")
	if meterTokenPriceV2726("gpt-5-mini", 100, 50) != nil {
		t.Fatal("missing rate must remain unknown")
	}
}

func TestMeterV2726VideoAndImageRates(t *testing.T) {
	t.Setenv("WORKTIC_METER_VIDEO_MODEL", "ray-3.2")
	t.Setenv("WORKTIC_METER_VIDEO_5S_720P_MICROUSD", "300000")
	c := meterFlatPriceV2726("studio_video", "ray-3.2", "720p", 5)
	if c == nil || *c != 300000 {
		t.Fatal("wrong five-second price")
	}
	if meterFlatPriceV2726("studio_video", "ray-3.2", "1080p", 5) != nil {
		t.Fatal("wrong resolution must be unpriced")
	}
	if meterFlatPriceV2726("studio_video", "ray-3.2", "720p", 9) != nil {
		t.Fatal("unsupported duration must be unpriced")
	}
	t.Setenv("WORKTIC_METER_IMAGE_MODEL", "gpt-image-1")
	t.Setenv("WORKTIC_METER_IMAGE_1024_MICROUSD", "40000")
	if c = meterFlatPriceV2726("studio_image", "gpt-image-1", "1024x1024", 1); c == nil || *c != 40000 {
		t.Fatal("wrong image estimate")
	}
	if meterFlatPriceV2726("studio_image", "other-model", "1024x1024", 1) != nil {
		t.Fatal("unmatched image model should be unpriced")
	}
}

func TestMeterV2726OpenAIUsage(t *testing.T) {
	raw := []byte(`{"id":"resp_x","model":"gpt-5-mini","usage":{"input_tokens":123,"output_tokens":45}}`)
	id, model, in, out, ok := meterParseOpenAIUsageV2726(raw)
	if !ok || id != "resp_x" || model != "gpt-5-mini" || in != 123 || out != 45 {
		t.Fatal("OpenAI usage parsing failed")
	}
	_, _, _, _, ok = meterParseOpenAIUsageV2726([]byte(`{"error":{"message":"failed"}}`))
	if ok {
		t.Fatal("error envelope must not count as known usage")
	}
}
