package acp

import "encoding/json"

// emitUsageFromACP pulls token counts out of Grok (and similar ACP)
// notifications / prompt results and forwards a wire "usage" pulse.
func emitUsageFromACP(raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var top map[string]any
	if json.Unmarshal(raw, &top) != nil {
		return
	}
	emitUsageFromMap(top)
}

func emitUsageFromMap(top map[string]any) {
	// grok: params.update.usage or params.update._meta, or result._meta
	cands := []map[string]any{top}
	if u, ok := top["update"].(map[string]any); ok {
		cands = append(cands, u)
		if uu, ok := u["usage"].(map[string]any); ok {
			cands = append(cands, uu)
		}
		if meta, ok := u["_meta"].(map[string]any); ok {
			cands = append(cands, meta)
		}
	}
	if u, ok := top["usage"].(map[string]any); ok {
		cands = append(cands, u)
	}
	if meta, ok := top["_meta"].(map[string]any); ok {
		cands = append(cands, meta)
	}

	var in, out, cr, cw, total, win int
	for _, m := range cands {
		pick := func(keys ...string) int {
			for _, k := range keys {
				if n := anyInt(m[k]); n > 0 {
					return n
				}
			}
			return 0
		}
		if v := pick("input", "inputTokens", "input_tokens"); v > in {
			in = v
		}
		if v := pick("output", "outputTokens", "output_tokens"); v > out {
			out = v
		}
		if v := pick("cache_read", "cacheRead", "cachedReadTokens", "cached_read_tokens"); v > cr {
			cr = v
		}
		if v := pick("cache_write", "cacheWrite", "cachedWriteTokens"); v > cw {
			cw = v
		}
		if v := pick("total", "totalTokens", "total_tokens"); v > total {
			total = v
		}
		if v := pick("window", "modelContextWindow", "contextWindow"); v > win {
			win = v
		}
	}
	if total == 0 {
		total = in + out + cr
	}
	if total == 0 && in == 0 && out == 0 {
		return
	}
	emit(map[string]any{
		"type":        "usage",
		"input":       in,
		"output":      out,
		"cache_read":  cr,
		"cache_write": cw,
		"total":       total,
		"window":      win,
	})
}

func anyInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}
