package main

// V27.34: read-only public commercial catalogue for landing and checkout.
// Prices are read from billing_plans (admin configurable). Creative grants are
// sourced from commercial_rules_v2721. Do not expose costs, private invoices or
// customer-specific contractual snapshots in this public endpoint.
import (
	"encoding/json"
	"net/http"
)

type publicOfferPlanV2734 struct {
	Code                 string          `json:"code"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	Monthly              float64         `json:"monthly_usdt"`
	Annual               float64         `json:"annual_usdt"`
	Users                int64           `json:"max_users"`
	Channels             int64           `json:"max_channels"`
	Contacts             int64           `json:"max_contacts"`
	Responses            int64           `json:"max_ai_responses"`
	Products             int64           `json:"max_products"`
	Rules                int64           `json:"max_rules"`
	Agents               int64           `json:"max_agents"`
	WhatsApp             int64           `json:"max_whatsapp"`
	Telegram             int64           `json:"max_telegram"`
	Messenger            int64           `json:"max_messenger"`
	Credits              int64           `json:"monthly_credits"`
	VoiceCreditsPlanned  int64           `json:"voice_credits_planned"`
	VoiceCredits         int64           `json:"voice_credits"`
	StudioCreditsPlanned int64           `json:"studio_credits_planned"`
	Projects             int64           `json:"max_studio_projects"`
	StorageMB            int64           `json:"studio_storage_mb"`
	StudioEditor         bool            `json:"studio_editor"`
	StudioImage          bool            `json:"studio_image_ai"`
	StudioVideo          bool            `json:"studio_video_ai"`
	StudioVoice          bool            `json:"studio_voice_ai"`
	StudioText           bool            `json:"studio_text_ai"`
	Modules              map[string]bool `json:"modules"`
}

func (a *App) publicCommercialOfferV2734Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	rows, err := a.db.Query(`SELECT code,name,description,price_usdt,max_users,max_channels,max_contacts,max_ai_responses,max_products,max_rules,COALESCE(max_agents,0),COALESCE(max_whatsapp,0),COALESCE(max_telegram,0),COALESCE(max_messenger,0),COALESCE(features_json,'{}') FROM billing_plans WHERE active=1 AND code IN ('free','personal','business','enterprise') ORDER BY price_usdt, id`)
	if err != nil {
		http.Error(w, "Catálogo no disponible", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	commercialActive := studioCreditsEnabled()
	topupsActive := a.studioTopupsEnabledV27349()
	moduleGatesActive := commercialModuleGateEnabledV2722()
	plans := make([]publicOfferPlanV2734, 0, 4)
	for rows.Next() {
		var p publicOfferPlanV2734
		var features string
		if err := rows.Scan(&p.Code, &p.Name, &p.Description, &p.Monthly, &p.Users, &p.Channels, &p.Contacts, &p.Responses, &p.Products, &p.Rules, &p.Agents, &p.WhatsApp, &p.Telegram, &p.Messenger, &features); err != nil {
			http.Error(w, "Catálogo incompleto", http.StatusServiceUnavailable)
			return
		}
		p.Annual = roundUSDT(p.Monthly * 12 * 0.8)
		if commercialActive {
			p.Responses = commercialCoreCapForPlan(p.Code, p.Responses)
		}
		rule, err := commercialReadRule(a.db, p.Code)
		if err != nil {
			// Never advertise unknown benefits. Keep other valid plans visible.
			// This plan stays unavailable until its commercial rule is repaired.
			continue
		}
		p.StudioEditor = rule.Editor
		// Never advertise unactivated AI creative benefits as already included.
		p.StudioImage = commercialActive && rule.Image
		p.StudioVideo = commercialActive && rule.Video
		p.StudioVoice = commercialActive && rule.Voice
		p.StudioText = commercialActive && rule.Text
		p.Credits = 0
		p.StudioCreditsPlanned = rule.Monthly
		// Voice is a different ledger. Enterprise grant is only usable when
		// the external provider and commercial voice switch are active.
		if p.Code == "enterprise" {
			p.VoiceCreditsPlanned = a.voiceEnterpriseIncluded()
			if voiceConfigured() {
				p.VoiceCredits = p.VoiceCreditsPlanned
			}
		}
		if commercialActive {
			p.Credits = rule.Monthly
		}
		p.Projects = rule.MaxProjects
		p.StorageMB = rule.StorageMB
		p.Modules = make(map[string]bool, len(commercialModuleCatalogV2722))
		for _, m := range commercialModuleCatalogV2722 {
			// This field represents module eligibility by plan, not an audit of
			// provider credentials or workflow availability.
			p.Modules[m.Key] = commercialModuleAllowedV2722(p.Code, m.Key, features)
		}
		plans = append(plans, p)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Catálogo incompleto", http.StatusServiceUnavailable)
		return
	}
	/* Active plans are independently publishable by superadmin.
	   A disabled plan must not suppress the other plans on the landing. */
	packages := make([]map[string]any, 0)
	if topupsActive {
		pr, e := a.db.Query(`SELECT name,credits,price_cents FROM commercial_packages_v2721 WHERE active=1 ORDER BY price_cents`)
		if e == nil {
			for pr.Next() {
				var name string
				var credits, cents int64
				if pr.Scan(&name, &credits, &cents) == nil {
					packages = append(packages, map[string]any{"name": name, "credits": credits, "price_usdt": float64(cents) / 100})
				}
			}
			pr.Close()
		}
	}
	// Only constant server metadata is public: never return API keys, internal costs or financial records.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	payload := map[string]any{"version": "27.35", "voice_active": voiceConfigured(), "voice_price_per_minute_credits": a.voicePrice(), "voice_packages": voicePackagesPriced(a.voicePrice(), a.voiceCreditPriceCents()), "voice_sales_active": a.voiceTopupsEnabled(), "plans": plans, "creative_commercial_active": commercialActive, "studio_credits_active": commercialActive, "topups_active": topupsActive, "studio_unit_prices": map[string]int64{"image": mustCreativeCreditsV27345("image", 0, 0), "video_5s": mustCreativeCreditsV27345("video", 5, 0), "video_10s": mustCreativeCreditsV27345("video", 10, 0)}, "module_gates_active": moduleGatesActive, "packages": packages, "currency": "USDT", "discount_annual_percent": 20, "notes": []string{"Los importes de checkout se verifican en el servidor; promociones pueden modificarlos.", "Las generaciones IA requieren crédito disponible, proveedor configurado y presupuestos técnicos autorizados; las recargas se habilitan por separado.", "Los servicios externos de Meta, Ads, Luma y otros requieren permisos, configuración y pueden tener cargos adicionales."}}
	_ = json.NewEncoder(w).Encode(payload)
}

// Prices for illustrative plan equivalences come from exactly the same Go quote
// function used to reserve and debit credits before contacting the provider.
func mustCreativeCreditsV27345(action string, duration, chars int) int64 {
	n, err := commercialCreditsQuote(action, duration, chars)
	if err != nil {
		return 0
	}
	return n
}
