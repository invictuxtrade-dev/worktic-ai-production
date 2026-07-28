package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

type Config struct {
	Port                     string
	AppName                  string
	AppEnv                   string
	BaseURL                  string
	DataDir                  string
	DatabaseDSN              string
	MaxMessageLength         int
	SendCooldownSeconds      int
	AutoReplyCooldownSeconds int
	AllowGroupMessages       bool
	TelegramBotToken         string
	OpenAIAPIKey             string
	OpenAIModel              string
	MessengerVerifyToken     string
	MetaGraphVersion         string
	USDTBEP20Address         string
	USDTTRC20Address         string
	PaymentConfirmations     int
	ChannelEncryptionKey     string
	MetaAppID                string
	MetaAppSecret            string
	LinkedInClientID         string
	LinkedInClientSecret     string
	TikTokClientKey          string
	TikTokClientSecret       string
	GoogleClientID           string
	GoogleClientSecret       string
}

type StoredMessage struct {
	ID          int64  `json:"id"`
	Channel     string `json:"channel"`
	WAID        string `json:"wa_id"`
	ChatJID     string `json:"chat_jid"`
	SenderJID   string `json:"sender_jid"`
	Direction   string `json:"direction"`
	MessageType string `json:"message_type"`
	Text        string `json:"text"`
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
}

type Conversation struct {
	ChatJID       string `json:"chat_jid"`
	Channel       string `json:"channel"`
	Phone         string `json:"phone"`
	Name          string `json:"name"`
	LastText      string `json:"last_text"`
	LastDirection string `json:"last_direction"`
	LastTimestamp string `json:"last_timestamp"`
	Unread        int    `json:"unread"`
	MessageCount  int    `json:"message_count"`
}

type AutoRule struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	MatchType string `json:"match_type"`
	Keyword   string `json:"keyword"`
	Response  string `json:"response"`
	Enabled   bool   `json:"enabled"`
}

type AgentConfig struct {
	Name         string `json:"name"`
	Company      string `json:"company"`
	Objective    string `json:"objective"`
	Tone         string `json:"tone"`
	Instructions string `json:"instructions"`
	Knowledge    string `json:"knowledge"`
	Enabled      bool   `json:"enabled"`
}

type CRMContact struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Channel   string `json:"channel"`
	Stage     string `json:"stage"`
	Tags      string `json:"tags"`
	Notes     string `json:"notes"`
	UpdatedAt string `json:"updated_at"`
}

type Opportunity struct {
	ID        int64   `json:"id"`
	ContactID int64   `json:"contact_id"`
	Title     string  `json:"title"`
	Stage     string  `json:"stage"`
	Value     float64 `json:"value"`
	Owner     string  `json:"owner"`
	UpdatedAt string  `json:"updated_at"`
}

type User struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Active      bool   `json:"active"`
	CreatedAt   string `json:"created_at"`
	Company     string `json:"company"`
	TenantID    int64  `json:"tenant_id"`
	AccountType string `json:"account_type"`
}

type Product struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	Stock       int     `json:"stock"`
	Active      bool    `json:"active"`
	UpdatedAt   string  `json:"updated_at"`
}

type Appointment struct {
	ID              int64  `json:"id"`
	ContactName     string `json:"contact_name"`
	ContactPhone    string `json:"contact_phone"`
	Service         string `json:"service"`
	StartsAt        string `json:"starts_at"`
	DurationMinutes int    `json:"duration_minutes"`
	Status          string `json:"status"`
	Notes           string `json:"notes"`
	CreatedAt       string `json:"created_at"`
}

type Plan struct {
	ID             int64   `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	PriceUSDT      float64 `json:"price_usdt"`
	BillingDays    int     `json:"billing_days"`
	MaxUsers       int     `json:"max_users"`
	MaxChannels    int     `json:"max_channels"`
	MaxContacts    int     `json:"max_contacts"`
	MaxAIResponses int     `json:"max_ai_responses"`
	MaxProducts    int     `json:"max_products"`
	MaxRules       int     `json:"max_rules"`
	Active         bool    `json:"active"`
}

type Subscription struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	PlanCode  string `json:"plan_code"`
	PlanName  string `json:"plan_name"`
	Status    string `json:"status"`
	StartsAt  string `json:"starts_at"`
	EndsAt    string `json:"ends_at"`
	CreatedAt string `json:"created_at"`
}

type CryptoPayment struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	UserName   string  `json:"user_name"`
	UserEmail  string  `json:"user_email"`
	PlanCode   string  `json:"plan_code"`
	PlanName   string  `json:"plan_name"`
	Network    string  `json:"network"`
	Wallet     string  `json:"wallet"`
	AmountUSDT float64 `json:"amount_usdt"`
	TxHash     string  `json:"tx_hash"`
	Status     string  `json:"status"`
	AdminNote  string  `json:"admin_note"`
	CreatedAt  string  `json:"created_at"`
	ReviewedAt string  `json:"reviewed_at"`
}

type Entitlements struct {
	PlanCode       string `json:"plan_code"`
	PlanName       string `json:"plan_name"`
	Subscription   string `json:"subscription_status"`
	EndsAt         string `json:"ends_at"`
	MaxUsers       int    `json:"max_users"`
	MaxChannels    int    `json:"max_channels"`
	MaxContacts    int    `json:"max_contacts"`
	MaxAIResponses int    `json:"max_ai_responses"`
	MaxProducts    int    `json:"max_products"`
	MaxRules       int    `json:"max_rules"`
	UsersUsed      int    `json:"users_used"`
	ContactsUsed   int    `json:"contacts_used"`
	ProductsUsed   int    `json:"products_used"`
	RulesUsed      int    `json:"rules_used"`
	AIUsed         int    `json:"ai_used"`
}

type App struct {
	cfg            Config
	db             *sql.DB
	client         *whatsmeow.Client
	mu             sync.RWMutex
	qrDataURL      string
	qrState        string
	lastError      string
	lastSend       time.Time
	autoLast       map[string]time.Time
	tgOffset       int64
	tgStop         chan struct{}
	channelManager *ChannelManager
}

func main() {
	loadEnvFile(".env")
	cfg := loadConfig()
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		log.Fatalf("no se pudo preparar DATA_DIR: %v", err)
	}
	dbPath := strings.TrimPrefix(strings.Split(cfg.DatabaseDSN, "?")[0], "file:")
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}

	db, err := sql.Open("sqlite", cfg.DatabaseDSN)
	if err != nil {
		log.Fatal(err)
	}
	if err = initSchema(db); err != nil {
		log.Fatal(err)
	}
	if err = initSocialHubSchema(db); err != nil {
		log.Fatalf("social hub schema: %v", err)
	}
	if err = initMarketingSchema(db); err != nil {
		log.Fatal(err)
	}
	if err = initLandingSchema(db); err != nil {
		log.Fatal(err)
	}
	if err = initGroupsSchema(&App{db: db}); err != nil {
		log.Fatal(err)
	}
	if err = initMultiAgentSchema(db); err != nil {
		log.Fatal(err)
	}
	if err = initChannelTenantSchema(db); err != nil {
		log.Fatal(err)
	}
	if err = initMessengerProductionSchema(db); err != nil {
		log.Fatalf("messenger production schema: %v", err)
	}
	if err = initCRMContactsPremiumSchema(db); err != nil {
		log.Fatalf("crm contacts schema: %v", err)
	}
	if err = initCRMOpportunitiesPremiumSchema(db); err != nil {
		log.Fatalf("crm opportunities schema: %v", err)
	}
	if err = initAgendaPremiumSchema(db); err != nil {
		log.Fatal(err)
	}
	if err = initCatalogPremiumSchema(db); err != nil {
		log.Fatal(err)
	}
	if err = migrateAgentTenants(db); err != nil {
		log.Fatalf("agent tenant migration: %v", err)
	}

	container, err := sqlstore.New(context.Background(), "sqlite", cfg.DatabaseDSN, waLog.Stdout("DB", "WARN", true))
	if err != nil {
		log.Fatal(err)
	}
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	client := whatsmeow.NewClient(deviceStore, waLog.Stdout("WA", "INFO", true))

	app := &App{cfg: cfg, db: db, client: client, qrState: "idle", autoLast: map[string]time.Time{}, tgStop: make(chan struct{})}
	go app.runSocialPublisher()
	app.channelManager = NewChannelManager(app)
	go app.channelManager.restoreActive()
	go app.runMessengerConversationSync()
	go app.runMessengerOutboxWorker()
	go app.runMessengerTokenMonitor()
	client.AddEventHandler(app.handleWAEvent)
	if cfg.TelegramBotToken != "" {
		go app.telegramLoop(cfg.TelegramBotToken)
	}
	if client.Store.ID != nil {
		app.qrState = "reconnecting"
		if err := client.Connect(); err != nil {
			app.setError(err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", app.loginHandler)
	mux.HandleFunc("/api/auth/register", app.registerHandler)
	mux.HandleFunc("/api/auth/logout", app.authLogoutHandler)
	mux.HandleFunc("/api/auth/me", app.meHandler)
	mux.HandleFunc("/api/account/profile", app.accountProfileHandler)
	mux.HandleFunc("/api/admin/users", app.usersHandler)
	mux.HandleFunc("/api/team/invitations", app.teamInvitationsHandler)
	mux.HandleFunc("/api/team/invitations/accept", app.acceptTeamInvitationHandler)
	mux.HandleFunc("/api/products", app.catalogProductsHandler)
	mux.HandleFunc("/api/catalog/upload", app.catalogImageUploadHandler)
	mux.HandleFunc("/uploads/catalog/", app.catalogUploadFileHandler)
	mux.HandleFunc("/api/appointments", app.appointmentsHandler)
	mux.HandleFunc("/api/agenda/settings", app.agendaSettingsHandler)
	mux.HandleFunc("/api/agenda/professionals", app.agendaProfessionalsHandler)
	mux.HandleFunc("/api/agenda/services", app.agendaServicesHandler)
	mux.HandleFunc("/api/agenda/hours", app.agendaHoursHandler)
	mux.HandleFunc("/api/agenda/availability", app.agendaAvailabilityHandler)
	mux.HandleFunc("/healthz", app.healthHandler)
	mux.HandleFunc("/readyz", app.readyHandler)
	mux.HandleFunc("/api/status", app.statusHandler)
	mux.HandleFunc("/api/channels/connections", app.channelConnectionsHandler)
	mux.HandleFunc("/api/channels/action", app.channelActionHandler)
	mux.HandleFunc("/api/channels/qr", app.channelQRHandler)
	mux.HandleFunc("/api/channels/health", app.channelHealthHandler)
	mux.HandleFunc("/api/connect", app.connectHandler)
	mux.HandleFunc("/api/disconnect", app.disconnectHandler)
	mux.HandleFunc("/api/logout", app.logoutHandler)
	mux.HandleFunc("/api/qr", app.qrHandler)
	mux.HandleFunc("/api/conversations", app.conversationsHandler)
	mux.HandleFunc("/api/messages", app.messagesHandler)
	mux.HandleFunc("/api/read", app.readHandler)
	mux.HandleFunc("/api/contact", app.contactHandler)
	mux.HandleFunc("/api/send", app.sendHandler)
	mux.HandleFunc("/api/rules", app.rulesHandler)
	mux.HandleFunc("/api/telegram", app.telegramHandler)
	mux.HandleFunc("/webhooks/telegram/", app.telegramWebhookHandler)
	mux.HandleFunc("/api/messenger", app.messengerHandler)
	mux.HandleFunc("/webhooks/meta/messenger", app.messengerWebhookHandler)
	mux.HandleFunc("/webhooks/messenger/", app.messengerTenantWebhookHandler)
	mux.HandleFunc("/api/agent", app.agentHandler)
	mux.HandleFunc("/api/agent/test", app.agentTestHandler)
	mux.HandleFunc("/api/agents", app.agentsHandler)
	mux.HandleFunc("/api/agents/test", app.agentInstanceTestHandler)
	mux.HandleFunc("/api/agents/routes", app.agentRoutesHandler)
	mux.HandleFunc("/api/agents/metrics", app.agentMetricsHandler)
	mux.HandleFunc("/api/agents/permissions", app.agentPermissionsHandler)
	mux.HandleFunc("/api/crm/contacts", app.crmContactsPremiumHandler)
	mux.HandleFunc("/api/crm/opportunities", app.opportunitiesPremiumHandler)
	mux.HandleFunc("/api/dashboard", app.dashboardHandler)
	mux.HandleFunc("/api/plans", app.plansHandler)
	mux.HandleFunc("/api/billing/entitlements", app.entitlementsHandler)
	mux.HandleFunc("/api/billing/config", app.billingConfigHandler)
	mux.HandleFunc("/api/billing/checkout", app.checkoutHandler)
	mux.HandleFunc("/api/billing/payments", app.paymentsHandler)
	mux.HandleFunc("/api/admin/overview", app.adminOverviewHandler)
	mux.HandleFunc("/api/admin/payments", app.adminPaymentsHandler)
	mux.HandleFunc("/api/admin/subscriptions", app.adminSubscriptionsHandler)
	mux.HandleFunc("/api/admin/plans", app.adminPlansHandler)
	mux.HandleFunc("/api/admin/full/overview", app.adminFullOverviewHandler)
	mux.HandleFunc("/api/admin/full/users", app.adminFullUsersHandler)
	mux.HandleFunc("/api/admin/full/plans", app.adminFullPlansHandler)
	mux.HandleFunc("/api/admin/full/payments", app.adminFullPaymentsHandler)
	mux.HandleFunc("/api/admin/full/subscriptions", app.adminFullSubscriptionsHandler)
	mux.HandleFunc("/api/social/overview", app.socialOverviewHandler)
	mux.HandleFunc("/api/social/analytics", app.socialAnalyticsHandler)
	mux.HandleFunc("/api/social/metrics/ingest", app.socialMetricsIngestHandler)
	mux.HandleFunc("/api/social/connections", app.socialConnectionsHandler)
	mux.HandleFunc("/api/social/posts", app.socialPostsHandler)
	mux.HandleFunc("/api/social/publish", app.socialPublishHandler)
	mux.HandleFunc("/api/social/oauth/start", app.socialOAuthStartHandler)
	mux.HandleFunc("/api/social/oauth/callback/", app.socialOAuthCallbackHandler)
	mux.HandleFunc("/api/social/test", app.socialConnectionTestHandler)
	mux.HandleFunc("/api/marketing/overview", app.marketingOverviewHandler)
	mux.HandleFunc("/api/marketing/limits", app.marketingLimitsHandler)
	mux.HandleFunc("/api/marketing/campaigns", app.marketingCampaignsHandler)
	mux.HandleFunc("/api/marketing/generate", app.marketingGenerateHandler)
	mux.HandleFunc("/api/marketing/content", app.marketingContentHandler)
	mux.HandleFunc("/api/marketing/forms", app.marketingFormsHandler)
	mux.HandleFunc("/api/marketing/leads", app.marketingLeadsHandler)
	mux.HandleFunc("/api/marketing/creatives", app.marketingCreativesHandler)
	mux.HandleFunc("/api/marketing/landings", app.landingsHandler)
	mux.HandleFunc("/api/marketing/landings/generate", app.landingGenerateHandler)
	mux.HandleFunc("/api/marketing/landings/channels", app.landingChannelsHandler)
	mux.HandleFunc("/api/marketing/landings/upload", app.landingImageUploadHandler)
	mux.HandleFunc("/uploads/landings/", app.landingUploadFileHandler)
	mux.HandleFunc("/api/groups/limits", app.groupLimitsHandler)
	mux.HandleFunc("/api/groups", app.groupsHandler)
	mux.HandleFunc("/api/groups/send", app.groupSendHandler)
	mux.HandleFunc("/api/groups/discovery", app.groupDiscoveryHandler)
	mux.HandleFunc("/api/groups/prospects", app.groupProspectsHandler)
	mux.HandleFunc("/f/", app.publicLeadFormHandler)
	mux.HandleFunc("/l/", app.publicLandingHandler)
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           secureHeaders(app.authMiddleware(mux)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		log.Printf("%s activo en :%s (%s)", cfg.AppName, cfg.Port, cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("servidor HTTP: %v", err)
		}
	}()

	<-stop
	log.Printf("apagado controlado iniciado")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	if app.channelManager != nil {
		app.channelManager.shutdown()
	}
	if app.client != nil {
		app.client.Disconnect()
	}
	_ = db.Close()
	log.Printf("apagado completado")
}

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (a *App) readyHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.db.PingContext(ctx); err != nil {
		writeError(w, errors.New("base de datos no disponible"), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]any{"status": "ready", "environment": a.cfg.AppEnv})
}

func loadConfig() Config {
	dataDir := env("DATA_DIR", "data")
	port := env("PORT", env("APP_PORT", "8080"))
	dsn := env("DATABASE_DSN", "")
	if dsn == "" {
		dsn = "file:" + filepath.Join(dataDir, "worktic.db") + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}
	return Config{
		Port:                     port,
		AppName:                  env("APP_NAME", "Worktic AI V14 Render Production"),
		AppEnv:                   strings.ToLower(env("APP_ENV", "development")),
		BaseURL:                  strings.TrimRight(env("BASE_URL", "http://localhost:"+port), "/"),
		DataDir:                  dataDir,
		DatabaseDSN:              dsn,
		MaxMessageLength:         envInt("MAX_MESSAGE_LENGTH", 2000),
		SendCooldownSeconds:      envInt("SEND_COOLDOWN_SECONDS", 3),
		AutoReplyCooldownSeconds: envInt("AUTO_REPLY_COOLDOWN_SECONDS", 30),
		AllowGroupMessages:       strings.EqualFold(env("ALLOW_GROUP_MESSAGES", "false"), "true"),
		TelegramBotToken:         env("TELEGRAM_BOT_TOKEN", ""),
		OpenAIAPIKey:             env("OPENAI_API_KEY", ""),
		OpenAIModel:              env("OPENAI_MODEL", "gpt-5-mini"),
		MessengerVerifyToken:     env("MESSENGER_VERIFY_TOKEN", "worktic_messenger_verify"),
		MetaGraphVersion:         env("META_GRAPH_VERSION", "v25.0"),
		USDTBEP20Address:         env("USDT_BEP20_ADDRESS", ""),
		USDTTRC20Address:         env("USDT_TRC20_ADDRESS", ""),
		PaymentConfirmations:     envInt("PAYMENT_CONFIRMATIONS", 12),
		ChannelEncryptionKey:     env("CHANNEL_ENCRYPTION_KEY", env("APP_NAME", "change-me-v13")),
		MetaAppID:                env("META_APP_ID", ""),
		MetaAppSecret:            env("META_APP_SECRET", ""),
		LinkedInClientID:         env("LINKEDIN_CLIENT_ID", ""),
		LinkedInClientSecret:     env("LINKEDIN_CLIENT_SECRET", ""),
		TikTokClientKey:          env("TIKTOK_CLIENT_KEY", ""),
		TikTokClientSecret:       env("TIKTOK_CLIENT_SECRET", ""),
		GoogleClientID:           env("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:       env("GOOGLE_CLIENT_SECRET", ""),
	}
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS worktic_messages (
 id INTEGER PRIMARY KEY AUTOINCREMENT, wa_id TEXT NOT NULL, chat_jid TEXT NOT NULL,
 sender_jid TEXT NOT NULL, direction TEXT NOT NULL, message_type TEXT NOT NULL DEFAULT 'text',
 text TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'received', timestamp TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_worktic_messages_chat ON worktic_messages(chat_jid,id);
CREATE INDEX IF NOT EXISTS idx_worktic_messages_ts ON worktic_messages(timestamp DESC);
CREATE TABLE IF NOT EXISTS worktic_contacts (
 chat_jid TEXT PRIMARY KEY, phone TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '',
 unread INTEGER NOT NULL DEFAULT 0, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS worktic_auto_rules (
 id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, match_type TEXT NOT NULL DEFAULT 'contains',
 keyword TEXT NOT NULL, response TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS worktic_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS worktic_agent (id INTEGER PRIMARY KEY CHECK(id=1), name TEXT NOT NULL DEFAULT 'Sofía', company TEXT NOT NULL DEFAULT 'Mi empresa', objective TEXT NOT NULL DEFAULT 'Atender clientes y captar oportunidades', tone TEXT NOT NULL DEFAULT 'Profesional y cercano', instructions TEXT NOT NULL DEFAULT '', knowledge TEXT NOT NULL DEFAULT '', enabled INTEGER NOT NULL DEFAULT 0);
INSERT OR IGNORE INTO worktic_agent(id) VALUES(1);
CREATE TABLE IF NOT EXISTS crm_contacts (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL DEFAULT '', phone TEXT NOT NULL DEFAULT '', email TEXT NOT NULL DEFAULT '', channel TEXT NOT NULL DEFAULT '', stage TEXT NOT NULL DEFAULT 'Nuevo', tags TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS crm_opportunities (id INTEGER PRIMARY KEY AUTOINCREMENT, contact_id INTEGER NOT NULL DEFAULT 0, title TEXT NOT NULL, stage TEXT NOT NULL DEFAULT 'Nuevo', value REAL NOT NULL DEFAULT 0, owner TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS app_users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'agent', company TEXT NOT NULL DEFAULT '', active INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS app_sessions (token TEXT PRIMARY KEY, user_id INTEGER NOT NULL, expires_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS crm_products (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', price REAL NOT NULL DEFAULT 0, currency TEXT NOT NULL DEFAULT 'COP', stock INTEGER NOT NULL DEFAULT 0, active INTEGER NOT NULL DEFAULT 1, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS crm_appointments (id INTEGER PRIMARY KEY AUTOINCREMENT, contact_name TEXT NOT NULL, contact_phone TEXT NOT NULL DEFAULT '', service TEXT NOT NULL, starts_at TEXT NOT NULL, duration_minutes INTEGER NOT NULL DEFAULT 30, status TEXT NOT NULL DEFAULT 'Programada', notes TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS billing_plans (
 id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
 price_usdt REAL NOT NULL DEFAULT 0, billing_days INTEGER NOT NULL DEFAULT 30, max_users INTEGER NOT NULL DEFAULT 1,
 max_channels INTEGER NOT NULL DEFAULT 1, max_contacts INTEGER NOT NULL DEFAULT 100, max_ai_responses INTEGER NOT NULL DEFAULT 50,
 max_products INTEGER NOT NULL DEFAULT 5, max_rules INTEGER NOT NULL DEFAULT 1, active INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS billing_subscriptions (
 id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, plan_code TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active',
 starts_at TEXT NOT NULL, ends_at TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_user ON billing_subscriptions(user_id,status,ends_at);
CREATE TABLE IF NOT EXISTS billing_payments (
 id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL, plan_code TEXT NOT NULL, network TEXT NOT NULL,
 wallet TEXT NOT NULL, amount_usdt REAL NOT NULL, tx_hash TEXT NOT NULL UNIQUE, status TEXT NOT NULL DEFAULT 'pending',
 admin_note TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, reviewed_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS usage_monthly (
 user_id INTEGER NOT NULL, period TEXT NOT NULL, ai_responses INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(user_id,period)
);
CREATE TABLE IF NOT EXISTS team_invitations (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, invited_by INTEGER NOT NULL, name TEXT NOT NULL DEFAULT '',
 phone TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'agent', area TEXT NOT NULL DEFAULT '', token TEXT NOT NULL UNIQUE, status TEXT NOT NULL DEFAULT 'pending',
 expires_at TEXT NOT NULL, accepted_user_id INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, accepted_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_team_invites_tenant ON team_invitations(tenant_id,status,created_at DESC);
`)
	if err != nil {
		return err
	}
	// Migración transparente desde versiones anteriores.
	_, _ = db.Exec(`ALTER TABLE app_users ADD COLUMN company TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE worktic_messages ADD COLUMN message_type TEXT NOT NULL DEFAULT 'text'`)
	_, _ = db.Exec(`ALTER TABLE worktic_messages ADD COLUMN channel TEXT NOT NULL DEFAULT 'whatsapp'`)
	_, _ = db.Exec(`ALTER TABLE worktic_contacts ADD COLUMN channel TEXT NOT NULL DEFAULT 'whatsapp'`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO billing_plans(code,name,description,price_usdt,billing_days,max_users,max_channels,max_contacts,max_ai_responses,max_products,max_rules,active) VALUES
	 ('free','Free','Para conocer la plataforma con funciones básicas',0,3650,1,1,100,50,5,1,1),
	 ('personal','Personal','Para profesionales independientes',25,30,1,2,500,2000,100,10,1),
	 ('business','Negocio','Para equipos comerciales pequeños',75,30,5,3,3000,10000,1000,50,1),
	 ('enterprise','Empresa','Para operaciones con varios asesores y canales',150,30,15,6,10000,30000,5000,250,1)`)
	var userCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM app_users`).Scan(&userCount)
	if userCount == 0 {
		salt := randomToken(16)
		_, _ = db.Exec(`INSERT INTO app_users(name,email,password_hash,role,company,active,created_at) VALUES(?,?,?,?,?,1,?)`, "Superadministrador", "admin@worktic.local", hashPassword("Admin123!", salt), "superadmin", "Worktic", time.Now().UTC().Format(time.RFC3339))
		_, _ = db.Exec(`INSERT OR REPLACE INTO worktic_settings(key,value) VALUES('admin_salt',?)`, salt)
	}
	var adminID int64
	if db.QueryRow(`SELECT id FROM app_users WHERE role='superadmin' ORDER BY id LIMIT 1`).Scan(&adminID) == nil {
		var sc int
		_ = db.QueryRow(`SELECT COUNT(*) FROM billing_subscriptions WHERE user_id=?`, adminID).Scan(&sc)
		if sc == 0 {
			now := time.Now().UTC()
			_, _ = db.Exec(`INSERT INTO billing_subscriptions(user_id,plan_code,status,starts_at,ends_at,created_at) VALUES(?,'enterprise','active',?,?,?)`, adminID, now.Format(time.RFC3339), now.AddDate(10, 0, 0).Format(time.RFC3339), now.Format(time.RFC3339))
		}
	}
	return nil
}

func (a *App) handleWAEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.IsFromMe {
			return
		}
		if v.Info.IsGroup && !a.cfg.AllowGroupMessages {
			return
		}
		text, typ := extractContent(v.Message)
		if strings.TrimSpace(text) == "" {
			text = "[Mensaje " + typ + "]"
		}
		chat := v.Info.Chat.String()
		sender := v.Info.Sender.String()
		msg := StoredMessage{Channel: "whatsapp", WAID: v.Info.ID, ChatJID: chat, SenderJID: sender, Direction: "in", MessageType: typ, Text: text, Status: "received", Timestamp: v.Info.Timestamp.UTC().Format(time.RFC3339)}
		if err := a.saveMessage(msg); err != nil {
			a.setError(err)
			return
		}
		name := strings.TrimSpace(v.Info.PushName)
		_ = a.upsertContact(chat, shortJID(chat), name, 1)
		_ = a.syncLegacyOpportunityIfSingleTenant(chat, "whatsapp", text, msg.Timestamp)
		go a.maybeAutoReply("whatsapp", chat, text)
	case *events.Connected:
		a.mu.Lock()
		a.qrState = "connected"
		a.qrDataURL = ""
		a.lastError = ""
		a.mu.Unlock()
	case *events.Disconnected:
		a.mu.Lock()
		if a.client.Store.ID != nil {
			a.qrState = "disconnected"
		} else {
			a.qrState = "idle"
		}
		a.mu.Unlock()
		if a.client.Store.ID != nil {
			go a.reconnectWithBackoff()
		}
	case *events.LoggedOut:
		a.mu.Lock()
		a.qrState = "logged_out"
		a.qrDataURL = ""
		a.mu.Unlock()
	}
}

func extractContent(m *waProto.Message) (string, string) {
	if m == nil {
		return "", "unknown"
	}
	if m.GetConversation() != "" {
		return m.GetConversation(), "text"
	}
	if x := m.GetExtendedTextMessage(); x != nil {
		return x.GetText(), "text"
	}
	if x := m.GetImageMessage(); x != nil {
		return x.GetCaption(), "image"
	}
	if x := m.GetVideoMessage(); x != nil {
		return x.GetCaption(), "video"
	}
	if x := m.GetDocumentMessage(); x != nil {
		n := x.GetFileName()
		if x.GetCaption() != "" {
			n += " — " + x.GetCaption()
		}
		return n, "document"
	}
	if m.GetAudioMessage() != nil {
		return "Nota de voz o audio", "audio"
	}
	if m.GetStickerMessage() != nil {
		return "Sticker", "sticker"
	}
	if m.GetContactMessage() != nil {
		return "Contacto compartido", "contact"
	}
	if m.GetLocationMessage() != nil {
		return "Ubicación compartida", "location"
	}
	return "", "unknown"
}

func (a *App) reconnectWithBackoff() {
	for i := 1; i <= 5; i++ {
		time.Sleep(time.Duration(i*3) * time.Second)
		if a.client.IsConnected() || a.client.Store.ID == nil {
			return
		}
		a.mu.Lock()
		a.qrState = "reconnecting"
		a.mu.Unlock()
		if err := a.client.Connect(); err == nil {
			return
		} else {
			a.setError(err)
		}
	}
}

func (a *App) maybeAutoReply(channel, chat, text string) {
	a.mu.Lock()
	if t, ok := a.autoLast[chat]; ok && time.Since(t) < time.Duration(a.cfg.AutoReplyCooldownSeconds)*time.Second {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	rows, err := a.db.Query(`SELECT id,name,match_type,keyword,response,enabled FROM worktic_auto_rules WHERE enabled=1 ORDER BY id`)
	if err != nil {
		a.setError(err)
		return
	}
	defer rows.Close()
	input := strings.ToLower(strings.TrimSpace(text))
	for rows.Next() {
		var r AutoRule
		var enabled int
		if rows.Scan(&r.ID, &r.Name, &r.MatchType, &r.Keyword, &r.Response, &enabled) != nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(r.Keyword))
		match := false
		switch r.MatchType {
		case "exact":
			match = input == key
		case "starts":
			match = strings.HasPrefix(input, key)
		default:
			match = strings.Contains(input, key)
		}
		if match {
			time.Sleep(1200 * time.Millisecond)
			if _, err := a.sendText(context.Background(), chat, r.Response, "auto"); err != nil {
				a.setError(err)
				return
			}
			a.mu.Lock()
			a.autoLast[chat] = time.Now()
			a.mu.Unlock()
			return
		}
	}
	a.maybeAIReply(channel, chat, text)
}

func (a *App) connectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if a.client.Store.ID == nil {
		if err := a.quotaAllowed(r, "channels"); err != nil {
			writeError(w, err, 403)
			return
		}
	}
	if a.client.Store.ID != nil {
		if !a.client.IsConnected() {
			if err := a.client.Connect(); err != nil {
				writeError(w, err, 500)
				return
			}
		}
		writeJSON(w, map[string]any{"ok": true, "message": "Sesión reconectada"})
		return
	}
	qrChan, err := a.client.GetQRChannel(context.Background())
	if err != nil && !errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
		writeError(w, err, 500)
		return
	}
	a.mu.Lock()
	a.qrState = "waiting_qr"
	a.qrDataURL = ""
	a.lastError = ""
	a.mu.Unlock()
	if err := a.client.Connect(); err != nil {
		writeError(w, err, 500)
		return
	}
	go func() {
		for evt := range qrChan {
			switch evt.Event {
			case "code":
				png, e := qrcode.Encode(evt.Code, qrcode.Medium, 320)
				if e != nil {
					a.setError(e)
					continue
				}
				a.mu.Lock()
				a.qrDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
				a.qrState = "qr_ready"
				a.mu.Unlock()
			case "success":
				a.mu.Lock()
				a.qrState = "connected"
				a.qrDataURL = ""
				a.mu.Unlock()
			case "timeout":
				a.mu.Lock()
				a.qrState = "timeout"
				a.qrDataURL = ""
				a.mu.Unlock()
			default:
				a.mu.Lock()
				a.qrState = evt.Event
				a.mu.Unlock()
			}
		}
	}()
	writeJSON(w, map[string]any{"ok": true, "message": "Generando QR"})
}
func (a *App) disconnectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	a.client.Disconnect()
	writeJSON(w, map[string]any{"ok": true})
}
func (a *App) logoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if a.client.Store.ID != nil {
		if err := a.client.Logout(context.Background()); err != nil {
			writeError(w, err, 500)
			return
		}
	}
	a.mu.Lock()
	a.qrState = "idle"
	a.qrDataURL = ""
	a.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true})
}
func (a *App) statusHandler(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	state, lastErr := a.qrState, a.lastError
	a.mu.RUnlock()
	jid := ""
	if a.client.Store.ID != nil {
		jid = a.client.Store.ID.String()
	}
	var convs, msgs, unread int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_contacts`).Scan(&convs)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_messages`).Scan(&msgs)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(unread),0) FROM worktic_contacts`).Scan(&unread)
	writeJSON(w, map[string]any{"connected": a.client.IsConnected() && a.client.IsLoggedIn(), "socket_connected": a.client.IsConnected(), "logged_in": a.client.IsLoggedIn(), "jid": jid, "state": state, "last_error": lastErr, "unofficial": true, "conversations": convs, "messages": msgs, "unread": unread, "telegram_configured": a.telegramToken() != "", "ai_configured": a.openAIKey() != ""})
}
func (a *App) qrHandler(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	writeJSON(w, map[string]any{"state": a.qrState, "data_url": a.qrDataURL, "error": a.lastError})
}

func (a *App) conversationsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT c.chat_jid,c.channel,c.phone,c.name,c.unread,
      COALESCE((SELECT text FROM worktic_messages m WHERE m.chat_jid=c.chat_jid ORDER BY m.id DESC LIMIT 1),''),
      COALESCE((SELECT direction FROM worktic_messages m WHERE m.chat_jid=c.chat_jid ORDER BY m.id DESC LIMIT 1),''),
      COALESCE((SELECT timestamp FROM worktic_messages m WHERE m.chat_jid=c.chat_jid ORDER BY m.id DESC LIMIT 1),c.updated_at),
      (SELECT COUNT(*) FROM worktic_messages m WHERE m.chat_jid=c.chat_jid)
      FROM worktic_contacts c ORDER BY 8 DESC`)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []Conversation{}
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ChatJID, &c.Channel, &c.Phone, &c.Name, &c.Unread, &c.LastText, &c.LastDirection, &c.LastTimestamp, &c.MessageCount); err != nil {
			writeError(w, err, 500)
			return
		}
		out = append(out, c)
	}
	writeJSON(w, out)
}
func (a *App) messagesHandler(w http.ResponseWriter, r *http.Request) {
	chat := strings.TrimSpace(r.URL.Query().Get("chat"))
	q := `SELECT id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp FROM worktic_messages`
	args := []any{}
	if chat != "" {
		q += ` WHERE chat_jid=?`
		args = append(args, chat)
	}
	q += ` ORDER BY id ASC LIMIT 500`
	rows, err := a.db.Query(q, args...)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []StoredMessage{}
	for rows.Next() {
		var m StoredMessage
		if err := rows.Scan(&m.ID, &m.Channel, &m.WAID, &m.ChatJID, &m.SenderJID, &m.Direction, &m.MessageType, &m.Text, &m.Status, &m.Timestamp); err != nil {
			writeError(w, err, 500)
			return
		}
		out = append(out, m)
	}
	writeJSON(w, out)
}
func (a *App) readHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		Chat string `json:"chat"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Chat == "" {
		writeError(w, errors.New("chat obligatorio"), 400)
		return
	}
	_, err := a.db.Exec(`UPDATE worktic_contacts SET unread=0 WHERE chat_jid=?`, req.Chat)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
func (a *App) contactHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		Chat string `json:"chat"`
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Chat == "" {
		writeError(w, errors.New("chat obligatorio"), 400)
		return
	}
	_, err := a.db.Exec(`UPDATE worktic_contacts SET name=?,updated_at=? WHERE chat_jid=?`, strings.TrimSpace(req.Name), time.Now().UTC().Format(time.RFC3339), req.Chat)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
func (a *App) sendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		To   string `json:"to"`
		Chat string `json:"chat"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err, 400)
		return
	}
	target := strings.TrimSpace(req.Chat)
	if target == "" {
		p := normalizePhone(req.To)
		if p != "" {
			target = types.NewJID(p, types.DefaultUserServer).String()
		}
	}
	if target == "" || strings.TrimSpace(req.Text) == "" {
		writeError(w, errors.New("destinatario y mensaje son obligatorios"), 400)
		return
	}
	var resp string
	var err error
	if strings.HasPrefix(target, "messenger:") {
		tenantID, _, tenantErr := a.tenantForRequest(r)
		if tenantErr != nil {
			writeError(w, tenantErr, 409)
			return
		}
		resp, err = a.sendTenantMessengerText(r.Context(), tenantID, target, req.Text)
	} else {
		resp, err = a.sendText(r.Context(), target, req.Text, "manual")
	}
	if err != nil {
		writeError(w, err, 502)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message_id": resp})
}
func (a *App) sendText(ctx context.Context, chat, text, source string) (string, error) {
	if strings.HasPrefix(chat, "messenger:") {
		psid := strings.TrimPrefix(chat, "messenger:")
		return a.sendMessengerText(ctx, psid, text)
	}

	text = strings.TrimSpace(text)
	if strings.HasPrefix(chat, "telegram:") {
		return a.sendTelegram(ctx, strings.TrimPrefix(chat, "telegram:"), text, source)
	}
	if len([]rune(text)) > a.cfg.MaxMessageLength {
		return "", fmt.Errorf("mensaje supera %d caracteres", a.cfg.MaxMessageLength)
	}
	if !a.client.IsConnected() || !a.client.IsLoggedIn() {
		return "", errors.New("WhatsApp no está conectado")
	}
	if source == "manual" {
		a.mu.Lock()
		wait := time.Duration(a.cfg.SendCooldownSeconds)*time.Second - time.Since(a.lastSend)
		if wait > 0 {
			a.mu.Unlock()
			return "", fmt.Errorf("espera %d segundos antes de otro envío", int(wait.Seconds())+1)
		}
		a.lastSend = time.Now()
		a.mu.Unlock()
	}
	jid, err := types.ParseJID(chat)
	if err != nil {
		return "", errors.New("destinatario inválido")
	}
	resp, err := a.client.SendMessage(ctx, jid, &waProto.Message{Conversation: proto.String(text)})
	if err != nil {
		return "", err
	}
	sender := ""
	if a.client.Store.ID != nil {
		sender = a.client.Store.ID.String()
	}
	status := "sent"
	if source == "auto" {
		status = "auto_sent"
	}
	m := StoredMessage{Channel: "whatsapp", WAID: resp.ID, ChatJID: jid.String(), SenderJID: sender, Direction: "out", MessageType: "text", Text: text, Status: status, Timestamp: time.Now().UTC().Format(time.RFC3339)}
	if err := a.saveMessage(m); err != nil {
		return "", err
	}
	_ = a.upsertContact(jid.String(), shortJID(jid.String()), "", 0)
	return resp.ID, nil
}

func (a *App) rulesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,name,match_type,keyword,response,enabled FROM worktic_auto_rules ORDER BY id DESC`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []AutoRule{}
		for rows.Next() {
			var x AutoRule
			var e int
			if rows.Scan(&x.ID, &x.Name, &x.MatchType, &x.Keyword, &x.Response, &e) != nil {
				continue
			}
			x.Enabled = e == 1
			out = append(out, x)
		}
		writeJSON(w, out)
	case http.MethodPost:
		if err := a.quotaAllowed(r, "rules"); err != nil {
			writeError(w, err, 403)
			return
		}
		var x AutoRule
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil {
			writeError(w, err, 400)
			return
		}
		x.Name = strings.TrimSpace(x.Name)
		x.Keyword = strings.TrimSpace(x.Keyword)
		x.Response = strings.TrimSpace(x.Response)
		if x.Name == "" || x.Keyword == "" || x.Response == "" {
			writeError(w, errors.New("nombre, palabra y respuesta son obligatorios"), 400)
			return
		}
		if x.MatchType != "exact" && x.MatchType != "starts" {
			x.MatchType = "contains"
		}
		enabled := 0
		if x.Enabled {
			enabled = 1
		}
		res, err := a.db.Exec(`INSERT INTO worktic_auto_rules(name,match_type,keyword,response,enabled) VALUES(?,?,?,?,?)`, x.Name, x.MatchType, x.Keyword, x.Response, enabled)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		x.ID, _ = res.LastInsertId()
		writeJSON(w, x)
	case http.MethodPut:
		var x AutoRule
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil || x.ID == 0 {
			writeError(w, errors.New("regla inválida"), 400)
			return
		}
		enabled := 0
		if x.Enabled {
			enabled = 1
		}
		_, err := a.db.Exec(`UPDATE worktic_auto_rules SET name=?,match_type=?,keyword=?,response=?,enabled=? WHERE id=?`, strings.TrimSpace(x.Name), x.MatchType, strings.TrimSpace(x.Keyword), strings.TrimSpace(x.Response), enabled, x.ID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id == 0 {
			writeError(w, errors.New("id obligatorio"), 400)
			return
		}
		_, err := a.db.Exec(`DELETE FROM worktic_auto_rules WHERE id=?`, id)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) saveMessage(m StoredMessage) error {
	_, err := a.db.Exec(`INSERT OR IGNORE INTO worktic_messages(channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?)`, defaultChannel(m.Channel), m.WAID, m.ChatJID, m.SenderJID, m.Direction, m.MessageType, m.Text, m.Status, m.Timestamp)
	return err
}
func (a *App) upsertContact(chat, phone, name string, unreadAdd int) error {
	channel := "whatsapp"
	if strings.HasPrefix(chat, "telegram:") {
		channel = "telegram"
	}
	if strings.HasPrefix(chat, "messenger:") {
		channel = "messenger"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := a.db.Exec(`INSERT INTO worktic_contacts(chat_jid,channel,phone,name,unread,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(chat_jid) DO UPDATE SET channel=excluded.channel,phone=excluded.phone,name=CASE WHEN excluded.name<>'' THEN excluded.name ELSE worktic_contacts.name END,unread=worktic_contacts.unread+?,updated_at=excluded.updated_at`, chat, channel, phone, name, unreadAdd, now, unreadAdd)
	if err != nil {
		return err
	}
	// Compatibilidad con la arquitectura legacy: solo adjudicamos el contacto
	// automáticamente cuando la instalación tiene un único tenant.
	var tenantCount int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&tenantCount)
	if tenantCount == 1 {
		var tenantID int64
		if a.db.QueryRow(`SELECT id FROM tenants LIMIT 1`).Scan(&tenantID) == nil && tenantID > 0 {
			crmPhone := phone
			if channel != "whatsapp" {
				crmPhone = ""
			}
			return a.syncCRMContactAt(tenantID, name, crmPhone, "", channel, "conversation", chat, now)
		}
	}
	return nil
}
func (a *App) setError(err error) {
	a.mu.Lock()
	a.lastError = err.Error()
	a.mu.Unlock()
	log.Printf("error: %v", err)
}

func defaultChannel(v string) string {
	if v == "" {
		return "whatsapp"
	}
	return v
}
func (a *App) setting(key string) string {
	var v string
	_ = a.db.QueryRow(`SELECT value FROM worktic_settings WHERE key=?`, key).Scan(&v)
	return v
}
func (a *App) setSetting(key, value string) {
	_, _ = a.db.Exec(`INSERT INTO worktic_settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
}
func (a *App) telegramToken() string {
	if v := a.setting("telegram_token"); v != "" {
		return v
	}
	return a.cfg.TelegramBotToken
}
func (a *App) openAIKey() string { return a.cfg.OpenAIAPIKey }

func (a *App) telegramHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		t := a.telegramToken()
		name := ""
		ok := false
		if t != "" {
			var res struct {
				Ok     bool `json:"ok"`
				Result struct {
					Username  string `json:"username"`
					FirstName string `json:"first_name"`
				} `json:"result"`
			}
			if a.tgCall(t, "getMe", nil, &res) == nil && res.Ok {
				ok = true
				name = "@" + res.Result.Username
			}
		}
		writeJSON(w, map[string]any{"configured": t != "", "connected": ok, "bot": name})
	case http.MethodPost:
		if a.telegramToken() == "" {
			if err := a.quotaAllowed(r, "channels"); err != nil {
				writeError(w, err, 403)
				return
			}
		}
		var q struct {
			Token string `json:"token"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || strings.TrimSpace(q.Token) == "" {
			writeError(w, errors.New("token obligatorio"), 400)
			return
		}
		var res struct {
			Ok          bool   `json:"ok"`
			Description string `json:"description"`
			Result      struct {
				Username string `json:"username"`
			} `json:"result"`
		}
		if err := a.tgCall(q.Token, "getMe", nil, &res); err != nil || !res.Ok {
			if err == nil {
				err = errors.New(res.Description)
			}
			writeError(w, err, 400)
			return
		}
		a.setSetting("telegram_token", strings.TrimSpace(q.Token))
		go a.telegramLoop(strings.TrimSpace(q.Token))
		writeJSON(w, map[string]any{"ok": true, "bot": "@" + res.Result.Username})
	case http.MethodDelete:
		a.setSetting("telegram_token", "")
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
func (a *App) tgCall(token, method string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.telegram.org/bot"+token+"/"+method, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 35 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram: %s", strings.TrimSpace(string(b)))
	}
	return json.Unmarshal(b, out)
}
func (a *App) telegramLoop(token string) {
	if token == "" {
		return
	}
	log.Printf("Telegram iniciado")
	for {
		if token != a.telegramToken() {
			return
		}
		var res struct {
			Ok     bool `json:"ok"`
			Result []struct {
				UpdateID int64 `json:"update_id"`
				Message  *struct {
					MessageID int64  `json:"message_id"`
					Text      string `json:"text"`
					Date      int64  `json:"date"`
					Chat      struct {
						ID        int64  `json:"id"`
						FirstName string `json:"first_name"`
						LastName  string `json:"last_name"`
						Username  string `json:"username"`
					} `json:"chat"`
					From struct {
						ID        int64  `json:"id"`
						FirstName string `json:"first_name"`
						LastName  string `json:"last_name"`
					} `json:"from"`
				} `json:"message"`
			} `json:"result"`
		}
		err := a.tgCall(token, "getUpdates", map[string]any{"offset": a.tgOffset, "timeout": 25, "allowed_updates": []string{"message"}}, &res)
		if err != nil {
			a.setError(err)
			time.Sleep(5 * time.Second)
			continue
		}
		for _, u := range res.Result {
			a.tgOffset = u.UpdateID + 1
			if u.Message == nil || strings.TrimSpace(u.Message.Text) == "" {
				continue
			}
			id := strconv.FormatInt(u.Message.Chat.ID, 10)
			chat := "telegram:" + id
			name := strings.TrimSpace(u.Message.Chat.FirstName + " " + u.Message.Chat.LastName)
			m := StoredMessage{Channel: "telegram", WAID: "tg-" + strconv.FormatInt(u.Message.MessageID, 10), ChatJID: chat, SenderJID: id, Direction: "in", MessageType: "text", Text: u.Message.Text, Status: "received", Timestamp: time.Unix(u.Message.Date, 0).UTC().Format(time.RFC3339)}
			_ = a.saveMessage(m)
			_ = a.upsertContact(chat, id, name, 1)
			_ = a.syncLegacyOpportunityIfSingleTenant(chat, "telegram", u.Message.Text, m.Timestamp)
			go a.maybeAutoReply("telegram", chat, u.Message.Text)
		}
	}
}
func (a *App) sendTelegram(ctx context.Context, chatID, text, source string) (string, error) {
	token := a.telegramToken()
	if token == "" {
		return "", errors.New("Telegram no está conectado")
	}
	var res struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	err := a.tgCall(token, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, &res)
	if err != nil {
		return "", err
	}
	if !res.Ok {
		return "", errors.New(res.Description)
	}
	id := "tg-" + strconv.FormatInt(res.Result.MessageID, 10)
	status := "sent"
	if source == "auto" || source == "ai" {
		status = source + "_sent"
	}
	_ = a.saveMessage(StoredMessage{Channel: "telegram", WAID: id, ChatJID: "telegram:" + chatID, SenderJID: "bot", Direction: "out", MessageType: "text", Text: text, Status: status, Timestamp: time.Now().UTC().Format(time.RFC3339)})
	_ = a.upsertContact("telegram:"+chatID, chatID, "", 0)
	return id, nil
}

func (a *App) loadAgent() AgentConfig {
	var x AgentConfig
	var e int
	_ = a.db.QueryRow(`SELECT name,company,objective,tone,instructions,knowledge,enabled FROM worktic_agent WHERE id=1`).Scan(&x.Name, &x.Company, &x.Objective, &x.Tone, &x.Instructions, &x.Knowledge, &e)
	x.Enabled = e == 1
	return x
}
func (a *App) agentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		x := a.loadAgent()
		writeJSON(w, map[string]any{"agent": x, "openai_configured": a.openAIKey() != "", "model": a.cfg.OpenAIModel})
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		AgentConfig
		APIKey string `json:"api_key"`
		Model  string `json:"model"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		writeError(w, errors.New("datos inválidos"), 400)
		return
	}
	e := 0
	if q.Enabled {
		e = 1
	}
	_, err := a.db.Exec(`UPDATE worktic_agent SET name=?,company=?,objective=?,tone=?,instructions=?,knowledge=?,enabled=? WHERE id=1`, q.Name, q.Company, q.Objective, q.Tone, q.Instructions, q.Knowledge, e)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
func (a *App) agentTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Text string `json:"text"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	out, err := a.aiResponse(q.Text, "")
	if err != nil {
		writeError(w, err, 502)
		return
	}
	writeJSON(w, map[string]any{"reply": out})
}
func (a *App) maybeAIReply(channel, chat, text string) {
	agent := a.loadAgent()
	if !agent.Enabled || a.openAIKey() == "" {
		return
	}
	a.mu.Lock()
	if t, ok := a.autoLast["ai:"+chat]; ok && time.Since(t) < time.Duration(a.cfg.AutoReplyCooldownSeconds)*time.Second {
		a.mu.Unlock()
		return
	}
	a.autoLast["ai:"+chat] = time.Now()
	a.mu.Unlock()
	history := a.recentHistory(chat, 8)
	reply, err := a.aiResponse(text, history)
	if err != nil {
		a.setError(err)
		return
	}
	if strings.TrimSpace(reply) == "" {
		return
	}
	time.Sleep(900 * time.Millisecond)
	_, err = a.sendText(context.Background(), chat, reply, "ai")
	if err != nil {
		a.setError(err)
	}
}
func (a *App) recentHistory(chat string, n int) string {
	rows, err := a.db.Query(`SELECT direction,text FROM worktic_messages WHERE chat_jid=? ORDER BY id DESC LIMIT ?`, chat, n)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var d, t string
		_ = rows.Scan(&d, &t)
		who := "Cliente"
		if d == "out" {
			who = "Asistente"
		}
		lines = append([]string{who + ": " + t}, lines...)
	}
	return strings.Join(lines, "\n")
}
func (a *App) aiResponse(userText, history string) (string, error) {
	key := a.openAIKey()
	if key == "" {
		return "", errors.New("falta OPENAI_API_KEY")
	}
	ag := a.loadAgent()
	products := []string{}
	tid := int64(0)
	_ = a.db.QueryRow(`SELECT id FROM tenants ORDER BY id LIMIT 1`).Scan(&tid)
	if rows, err := a.db.Query(`SELECT name,description,CASE WHEN promotional_price>0 THEN promotional_price ELSE price END,currency,stock FROM crm_products WHERE tenant_id=? AND active=1 ORDER BY featured DESC,name`, tid); err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, desc, currency string
			var price float64
			var stock int
			if rows.Scan(&name, &desc, &price, &currency, &stock) == nil {
				products = append(products, fmt.Sprintf("- %s: %s | Precio: %s %.2f | Stock/cupos: %d", name, desc, currency, price, stock))
			}
		}
	}
	catalog := strings.Join(products, "\n")
	prompt := fmt.Sprintf("Eres %s, asistente de %s. Objetivo: %s. Tono: %s. Reglas: %s. Información verificada del negocio: %s. Catálogo vigente:\n%s\nResponde de forma humana, clara y breve. No inventes precios, stock, horarios ni disponibilidad. Cuando falte información, dilo y ofrece transferir a una persona. Historial reciente:\n%s\nMensaje actual del cliente: %s", ag.Name, ag.Company, ag.Objective, ag.Tone, ag.Instructions, ag.Knowledge, catalog, history, userText)
	payload := map[string]any{"model": a.cfg.OpenAIModel, "input": prompt, "store": false}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("OpenAI: %s", strings.TrimSpace(string(raw)))
	}
	var x struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(raw, &x) != nil {
		return "", errors.New("respuesta OpenAI inválida")
	}
	for _, o := range x.Output {
		for _, c := range o.Content {
			if c.Type == "output_text" && c.Text != "" {
				return strings.TrimSpace(c.Text), nil
			}
		}
	}
	return "", errors.New("OpenAI no devolvió texto")
}

func (a *App) crmContactsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,name,phone,email,channel,stage,tags,notes,updated_at FROM crm_contacts ORDER BY updated_at DESC`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []CRMContact{}
		for rows.Next() {
			var x CRMContact
			_ = rows.Scan(&x.ID, &x.Name, &x.Phone, &x.Email, &x.Channel, &x.Stage, &x.Tags, &x.Notes, &x.UpdatedAt)
			out = append(out, x)
		}
		writeJSON(w, out)
	case http.MethodPost:
		if err := a.quotaAllowed(r, "contacts"); err != nil {
			writeError(w, err, 403)
			return
		}
		var x CRMContact
		if json.NewDecoder(r.Body).Decode(&x) != nil {
			writeError(w, errors.New("datos inválidos"), 400)
			return
		}
		x.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		res, err := a.db.Exec(`INSERT INTO crm_contacts(name,phone,email,channel,stage,tags,notes,updated_at) VALUES(?,?,?,?,?,?,?,?)`, x.Name, x.Phone, x.Email, x.Channel, x.Stage, x.Tags, x.Notes, x.UpdatedAt)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		x.ID, _ = res.LastInsertId()
		writeJSON(w, x)
	case http.MethodPut:
		var x CRMContact
		if json.NewDecoder(r.Body).Decode(&x) != nil || x.ID == 0 {
			writeError(w, errors.New("contacto inválido"), 400)
			return
		}
		x.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		_, err := a.db.Exec(`UPDATE crm_contacts SET name=?,phone=?,email=?,channel=?,stage=?,tags=?,notes=?,updated_at=? WHERE id=?`, x.Name, x.Phone, x.Email, x.Channel, x.Stage, x.Tags, x.Notes, x.UpdatedAt, x.ID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id == 0 {
			writeError(w, errors.New("contacto inválido"), 400)
			return
		}
		_, err := a.db.Exec(`DELETE FROM crm_contacts WHERE id=?`, id)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
func (a *App) opportunitiesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,contact_id,title,stage,value,owner,updated_at FROM crm_opportunities ORDER BY updated_at DESC`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []Opportunity{}
		for rows.Next() {
			var x Opportunity
			_ = rows.Scan(&x.ID, &x.ContactID, &x.Title, &x.Stage, &x.Value, &x.Owner, &x.UpdatedAt)
			out = append(out, x)
		}
		writeJSON(w, out)
	case http.MethodPost:
		var x Opportunity
		if json.NewDecoder(r.Body).Decode(&x) != nil || strings.TrimSpace(x.Title) == "" {
			writeError(w, errors.New("título obligatorio"), 400)
			return
		}
		x.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		res, err := a.db.Exec(`INSERT INTO crm_opportunities(contact_id,title,stage,value,owner,updated_at) VALUES(?,?,?,?,?,?)`, x.ContactID, x.Title, x.Stage, x.Value, x.Owner, x.UpdatedAt)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		x.ID, _ = res.LastInsertId()
		writeJSON(w, x)
	case http.MethodPut:
		var x Opportunity
		if json.NewDecoder(r.Body).Decode(&x) != nil || x.ID == 0 {
			writeError(w, errors.New("oportunidad inválida"), 400)
			return
		}
		x.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		_, err := a.db.Exec(`UPDATE crm_opportunities SET contact_id=?,title=?,stage=?,value=?,owner=?,updated_at=? WHERE id=?`, x.ContactID, x.Title, x.Stage, x.Value, x.Owner, x.UpdatedAt, x.ID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id == 0 {
			writeError(w, errors.New("oportunidad inválida"), 400)
			return
		}
		_, err := a.db.Exec(`DELETE FROM crm_opportunities WHERE id=?`, id)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
func (a *App) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	tenant, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}
	var conv, msgs, unread, contacts, opps, products, appointments int
	var value float64
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_contacts WHERE tenant_id=?`, tenant).Scan(&conv)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=?`, tenant).Scan(&msgs)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(unread),0) FROM worktic_contacts WHERE tenant_id=?`, tenant).Scan(&unread)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`, tenant).Scan(&contacts)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Ganado','Perdido')`, tenant).Scan(&opps)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(value),0) FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Perdido')`, tenant).Scan(&value)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_products WHERE tenant_id=? AND active=1`, tenant).Scan(&products)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND status IN ('Programada','Confirmada')`, tenant).Scan(&appointments)
	writeJSON(w, map[string]any{"conversations": conv, "messages": msgs, "unread": unread, "contacts": contacts, "open_opportunities": opps, "pipeline_value": value, "products": products, "appointments": appointments})
}

type contextKey string

const userContextKey contextKey = "user"

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
func hashPassword(password, salt string) string {
	h := sha256.Sum256([]byte(salt + ":" + password))
	v := h[:]
	for i := 0; i < 120000; i++ {
		x := sha256.Sum256(append(v, []byte(salt)...))
		v = x[:]
	}
	return hex.EncodeToString(v)
}
func (a *App) adminSalt() string { return a.setting("admin_salt") }
func (a *App) currentUser(r *http.Request) *User {
	if v := r.Context().Value(userContextKey); v != nil {
		if u, ok := v.(*User); ok {
			return u
		}
	}
	return nil
}
func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/api/auth/login") || strings.HasPrefix(r.URL.Path, "/api/auth/register") || strings.HasPrefix(r.URL.Path, "/api/team/invitations/accept") || strings.HasPrefix(r.URL.Path, "/webhooks/meta/messenger") || strings.HasPrefix(r.URL.Path, "/webhooks/messenger/") || strings.HasPrefix(r.URL.Path, "/webhooks/telegram/") {
			next.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie("worktic_session")
		if err != nil || c.Value == "" {
			writeError(w, errors.New("sesión requerida"), 401)
			return
		}
		var u User
		var active int
		err = a.db.QueryRow(`SELECT u.id,u.name,u.email,u.role,u.company,u.active,u.created_at,u.tenant_id,COALESCE(t.account_type,'personal') FROM app_sessions s JOIN app_users u ON u.id=s.user_id LEFT JOIN tenants t ON t.id=u.tenant_id WHERE s.token=? AND s.expires_at>?`, c.Value, time.Now().UTC().Format(time.RFC3339)).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Company, &active, &u.CreatedAt, &u.TenantID, &u.AccountType)
		if err != nil || active != 1 {
			writeError(w, errors.New("sesión inválida o vencida"), 401)
			return
		}
		u.Active = true
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, &u)))
	})
}
func (a *App) createSession(w http.ResponseWriter, userID int64) error {
	t := randomToken(32)
	exp := time.Now().Add(7 * 24 * time.Hour).UTC()
	_, err := a.db.Exec(`INSERT INTO app_sessions(token,user_id,expires_at) VALUES(?,?,?)`, t, userID, exp.Format(time.RFC3339))
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: "worktic_session", Value: t, Path: "/", HttpOnly: true, Secure: a.cfg.AppEnv == "production", SameSite: http.SameSiteLaxMode, Expires: exp})
	return nil
}
func (a *App) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Company  string `json:"company"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	var u User
	var ph string
	var active int
	err := a.db.QueryRow(`SELECT u.id,u.name,u.email,u.password_hash,u.role,u.company,u.active,u.created_at,u.tenant_id,COALESCE(t.account_type,'personal') FROM app_users u LEFT JOIN tenants t ON t.id=u.tenant_id WHERE lower(u.email)=lower(?)`, strings.TrimSpace(q.Email)).Scan(&u.ID, &u.Name, &u.Email, &ph, &u.Role, &u.Company, &active, &u.CreatedAt, &u.TenantID, &u.AccountType)
	if err != nil || active != 1 || ph != hashPassword(q.Password, a.adminSalt()) {
		writeError(w, errors.New("correo o contraseña incorrectos"), 401)
		return
	}
	if err = a.createSession(w, u.ID); err != nil {
		writeError(w, err, 500)
		return
	}
	u.Active = true
	writeJSON(w, map[string]any{"ok": true, "user": u})
}
func (a *App) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Name        string `json:"name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		Company     string `json:"company"`
		AccountType string `json:"account_type"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	if len(strings.TrimSpace(q.Name)) < 2 || !strings.Contains(q.Email, "@") || len(q.Password) < 8 {
		writeError(w, errors.New("nombre, correo válido y contraseña de mínimo 8 caracteres son obligatorios"), 400)
		return
	}
	if !strings.EqualFold(q.AccountType, "personal") && len(strings.TrimSpace(q.Company)) < 2 {
		writeError(w, errors.New("el nombre de la empresa es obligatorio"), 400)
		return
	}
	company := strings.TrimSpace(q.Company)
	if strings.EqualFold(q.AccountType, "personal") && company == "" {
		company = "Mi espacio"
	}
	now := time.Now().UTC()
	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO app_users(name,email,password_hash,role,company,active,created_at,tenant_id) VALUES(?,?,?,'owner',?,1,?,0)`, strings.TrimSpace(q.Name), strings.ToLower(strings.TrimSpace(q.Email)), hashPassword(q.Password, a.adminSalt()), company, now.Format(time.RFC3339))
	if err != nil {
		writeError(w, errors.New("el correo ya está registrado"), 400)
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		writeError(w, err, 500)
		return
	}
	accountType := "business"
	if strings.EqualFold(q.AccountType, "personal") {
		accountType = "personal"
	}
	tenantRes, err := tx.Exec(`INSERT INTO tenants(name,account_type,owner_user_id,status,created_at,updated_at) VALUES(?,?,?,?,?,?)`, company, accountType, id, "active", now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		writeError(w, fmt.Errorf("no se pudo crear el espacio del usuario: %w", err), 500)
		return
	}
	tenantID, err := tenantRes.LastInsertId()
	if err != nil || tenantID == 0 {
		writeError(w, errors.New("no se pudo asignar el espacio del usuario"), 500)
		return
	}
	if _, err = tx.Exec(`UPDATE app_users SET tenant_id=? WHERE id=?`, tenantID, id); err != nil {
		writeError(w, err, 500)
		return
	}
	if _, err = tx.Exec(`INSERT INTO tenant_users(tenant_id,user_id,role,created_at) VALUES(?,?,?,?)`, tenantID, id, "owner", now.Format(time.RFC3339)); err != nil {
		writeError(w, err, 500)
		return
	}
	if _, err = tx.Exec(`INSERT INTO billing_subscriptions(user_id,plan_code,status,starts_at,ends_at,created_at) VALUES(?, 'free','active',?,?,?)`, id, now.Format(time.RFC3339), now.AddDate(10, 0, 0).Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
		writeError(w, err, 500)
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, err, 500)
		return
	}
	if err = a.createSession(w, id); err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "tenant_id": tenantID})
}
func (a *App) authLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("worktic_session"); e == nil {
		_, _ = a.db.Exec(`DELETE FROM app_sessions WHERE token=?`, c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "worktic_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.AppEnv == "production", SameSite: http.SameSiteLaxMode})
	writeJSON(w, map[string]any{"ok": true})
}
func (a *App) meHandler(w http.ResponseWriter, r *http.Request) { writeJSON(w, a.currentUser(r)) }

func (a *App) billingAccountUserID(u *User) int64 {
	if u == nil || u.TenantID == 0 {
		if u != nil {
			return u.ID
		}
		return 0
	}
	var ownerID int64
	if err := a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, u.TenantID).Scan(&ownerID); err == nil && ownerID > 0 {
		return ownerID
	}
	return u.ID
}

func (a *App) accountProfileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	billingID := a.billingAccountUserID(u)
	p, sub, err := a.activePlan(billingID)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	var tenantName, accountType, tenantStatus string
	var ownerID int64
	_ = a.db.QueryRow(`SELECT name,account_type,status,owner_user_id FROM tenants WHERE id=?`, u.TenantID).Scan(&tenantName, &accountType, &tenantStatus, &ownerID)
	var pending int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE user_id=? AND status='pending'`, billingID).Scan(&pending)
	var usersUsed, channelsUsed, agentsUsed int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE tenant_id=? AND active=1`, u.TenantID).Scan(&usersUsed)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE tenant_id=? AND status NOT IN ('revoked','deleted')`, u.TenantID).Scan(&channelsUsed)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ai_agents WHERE tenant_id=?`, u.TenantID).Scan(&agentsUsed)
	daysRemaining := 0
	if end, e := time.Parse(time.RFC3339, sub.EndsAt); e == nil {
		daysRemaining = int(time.Until(end).Hours() / 24)
		if daysRemaining < 0 {
			daysRemaining = 0
		}
	}
	writeJSON(w, map[string]any{
		"user": u, "tenant_id": u.TenantID, "tenant_name": tenantName, "account_type": accountType,
		"tenant_status": tenantStatus, "is_owner": u.ID == ownerID, "plan": p, "subscription": sub,
		"days_remaining": daysRemaining, "pending_payments": pending,
		"usage": map[string]any{"users": usersUsed, "channels": channelsUsed, "agents": agentsUsed},
	})
}

func normalizePhone(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (a *App) teamInvitationsHandler(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil || (u.Role != "superadmin" && u.Role != "owner" && u.Role != "admin") {
		writeError(w, errors.New("acceso reservado a propietarios y administradores"), 403)
		return
	}
	if u.Role != "superadmin" && u.AccountType != "business" {
		writeError(w, errors.New("las invitaciones de equipo están disponibles para cuentas empresariales"), 403)
		return
	}
	tenantID := u.TenantID
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,name,phone,role,area,status,expires_at,created_at FROM team_invitations WHERE tenant_id=? ORDER BY id DESC`, tenantID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id int64
			var name, phone, role, area, status, expires, created string
			_ = rows.Scan(&id, &name, &phone, &role, &area, &status, &expires, &created)
			out = append(out, map[string]any{"id": id, "name": name, "phone": phone, "role": role, "area": area, "status": status, "expires_at": expires, "created_at": created})
		}
		writeJSON(w, out)
	case http.MethodPost:
		var q struct {
			Name  string `json:"name"`
			Phone string `json:"phone"`
			Role  string `json:"role"`
			Area  string `json:"area"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		q.Phone = normalizePhone(q.Phone)
		q.Name = strings.TrimSpace(q.Name)
		q.Area = strings.TrimSpace(q.Area)
		allowed := map[string]bool{"admin": true, "supervisor": true, "agent": true}
		if !allowed[q.Role] {
			q.Role = "agent"
		}
		if q.Name == "" || len(q.Phone) < 8 {
			writeError(w, errors.New("nombre y teléfono internacional son obligatorios"), 400)
			return
		}
		billingID := a.billingAccountUserID(u)
		plan, _, err := a.activePlan(billingID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		var members, pending int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE tenant_id=? AND active=1`, tenantID).Scan(&members)
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM team_invitations WHERE tenant_id=? AND status='pending' AND expires_at>?`, tenantID, time.Now().UTC().Format(time.RFC3339)).Scan(&pending)
		if members+pending >= plan.MaxUsers {
			writeError(w, fmt.Errorf("límite del plan alcanzado: %d de %d usuarios o invitaciones", members+pending, plan.MaxUsers), 403)
			return
		}
		token := randomToken(32)
		now := time.Now().UTC()
		expires := now.Add(72 * time.Hour)
		res, err := a.db.Exec(`INSERT INTO team_invitations(tenant_id,invited_by,name,phone,role,area,token,status,expires_at,created_at) VALUES(?,?,?,?,?,?,?,'pending',?,?)`, tenantID, u.ID, q.Name, q.Phone, q.Role, q.Area, token, expires.Format(time.RFC3339), now.Format(time.RFC3339))
		if err != nil {
			writeError(w, err, 500)
			return
		}
		id, _ := res.LastInsertId()
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		host := r.Host
		link := fmt.Sprintf("%s://%s/invite.html?token=%s", scheme, host, url.QueryEscape(token))
		company := u.Company
		var tenantName string
		_ = a.db.QueryRow(`SELECT name FROM tenants WHERE id=?`, tenantID).Scan(&tenantName)
		if tenantName != "" {
			company = tenantName
		}
		message := fmt.Sprintf("Hola %s, te invitaron a unirte al equipo de %s en Worktic AI como %s. Acepta la invitación aquí: %s", q.Name, company, q.Role, link)
		wa := fmt.Sprintf("https://wa.me/%s?text=%s", q.Phone, url.QueryEscape(message))
		writeJSON(w, map[string]any{"ok": true, "id": id, "invite_url": link, "whatsapp_url": wa, "expires_at": expires.Format(time.RFC3339)})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id == 0 {
			writeError(w, errors.New("invitación inválida"), 400)
			return
		}
		res, err := a.db.Exec(`UPDATE team_invitations SET status='cancelled' WHERE id=? AND tenant_id=? AND status='pending'`, id, tenantID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("invitación no encontrada o ya procesada"), 404)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) acceptTeamInvitationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		token := r.URL.Query().Get("token")
		var name, phone, role, area, status, expires, tenantName string
		err := a.db.QueryRow(`SELECT i.name,i.phone,i.role,i.area,i.status,i.expires_at,t.name FROM team_invitations i JOIN tenants t ON t.id=i.tenant_id WHERE i.token=?`, token).Scan(&name, &phone, &role, &area, &status, &expires, &tenantName)
		if err != nil {
			writeError(w, errors.New("invitación no encontrada"), 404)
			return
		}
		valid := status == "pending"
		if t, e := time.Parse(time.RFC3339, expires); e == nil && time.Now().After(t) {
			valid = false
		}
		writeJSON(w, map[string]any{"name": name, "phone": phone, "role": role, "area": area, "tenant_name": tenantName, "expires_at": expires, "valid": valid})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Token    string `json:"token"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	if !strings.Contains(q.Email, "@") || len(q.Password) < 8 {
		writeError(w, errors.New("correo válido y contraseña de mínimo 8 caracteres son obligatorios"), 400)
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer tx.Rollback()
	var inviteID, tenantID int64
	var invitedName, role, status, expires, company string
	err = tx.QueryRow(`SELECT i.id,i.tenant_id,i.name,i.role,i.status,i.expires_at,t.name FROM team_invitations i JOIN tenants t ON t.id=i.tenant_id WHERE i.token=?`, q.Token).Scan(&inviteID, &tenantID, &invitedName, &role, &status, &expires, &company)
	if err != nil || status != "pending" {
		writeError(w, errors.New("invitación inválida o utilizada"), 400)
		return
	}
	if t, e := time.Parse(time.RFC3339, expires); e == nil && time.Now().After(t) {
		writeError(w, errors.New("la invitación venció"), 400)
		return
	}
	name := strings.TrimSpace(q.Name)
	if name == "" {
		name = invitedName
	}
	now := time.Now().UTC()
	res, err := tx.Exec(`INSERT INTO app_users(name,email,password_hash,role,company,active,created_at,tenant_id) VALUES(?,?,?,?,?,1,?,?)`, name, strings.ToLower(strings.TrimSpace(q.Email)), hashPassword(q.Password, a.adminSalt()), role, company, now.Format(time.RFC3339), tenantID)
	if err != nil {
		writeError(w, errors.New("el correo ya está registrado"), 400)
		return
	}
	uid, _ := res.LastInsertId()
	if _, err = tx.Exec(`INSERT INTO tenant_users(tenant_id,user_id,role,created_at) VALUES(?,?,?,?)`, tenantID, uid, role, now.Format(time.RFC3339)); err != nil {
		writeError(w, err, 500)
		return
	}
	if _, err = tx.Exec(`UPDATE team_invitations SET status='accepted',accepted_user_id=?,accepted_at=? WHERE id=?`, uid, now.Format(time.RFC3339), inviteID); err != nil {
		writeError(w, err, 500)
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, err, 500)
		return
	}
	if err = a.createSession(w, uid); err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) usersHandler(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil || (u.Role != "superadmin" && u.Role != "owner" && u.Role != "admin") {
		writeError(w, errors.New("acceso reservado a propietarios y administradores"), 403)
		return
	}
	if u.Role != "superadmin" && u.AccountType == "personal" {
		writeError(w, errors.New("las cuentas personales no incluyen administración de equipo; mejora a un plan empresarial y convierte tu espacio a empresa"), 403)
		return
	}
	targetAllowed := func(id int64) bool {
		if u.Role == "superadmin" {
			return true
		}
		var tid int64
		return a.db.QueryRow(`SELECT tenant_id FROM app_users WHERE id=?`, id).Scan(&tid) == nil && tid == u.TenantID
	}
	switch r.Method {
	case http.MethodGet:
		query := `SELECT id,name,email,role,company,active,created_at FROM app_users WHERE tenant_id=? ORDER BY id`
		args := []any{u.TenantID}
		if u.Role == "superadmin" {
			query = `SELECT id,name,email,role,company,active,created_at FROM app_users ORDER BY id`
			args = nil
		}
		rows, e := a.db.Query(query, args...)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []User{}
		for rows.Next() {
			var x User
			var ac int
			_ = rows.Scan(&x.ID, &x.Name, &x.Email, &x.Role, &x.Company, &ac, &x.CreatedAt)
			x.Active = ac == 1
			x.TenantID = u.TenantID
			out = append(out, x)
		}
		writeJSON(w, out)
	case http.MethodPut:
		var x User
		_ = json.NewDecoder(r.Body).Decode(&x)
		if x.ID == 0 || !targetAllowed(x.ID) {
			writeError(w, errors.New("usuario inválido o fuera de tu empresa"), 403)
			return
		}
		if x.ID == u.ID && !x.Active {
			writeError(w, errors.New("no puedes desactivar tu propio acceso"), 400)
			return
		}
		ac := 0
		if x.Active {
			ac = 1
		}
		allowed := map[string]bool{"owner": true, "admin": true, "supervisor": true, "agent": true}
		if u.Role == "superadmin" {
			allowed["superadmin"] = true
		}
		if !allowed[x.Role] {
			x.Role = "agent"
		}
		if u.Role == "admin" && (x.Role == "owner" || x.Role == "superadmin") {
			writeError(w, errors.New("un administrador no puede asignar ese rol"), 403)
			return
		}
		_, e := a.db.Exec(`UPDATE app_users SET name=?,role=?,active=? WHERE id=?`, x.Name, x.Role, ac, x.ID)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		_, _ = a.db.Exec(`UPDATE tenant_users SET role=? WHERE tenant_id=? AND user_id=?`, x.Role, u.TenantID, x.ID)
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id == 0 || id == u.ID || !targetAllowed(id) {
			writeError(w, errors.New("no puedes eliminar este usuario"), 400)
			return
		}
		_, _ = a.db.Exec(`DELETE FROM tenant_users WHERE user_id=?`, id)
		_, e := a.db.Exec(`DELETE FROM app_users WHERE id=?`, id)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
func (a *App) legacyProductsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, e := a.db.Query(`SELECT id,name,description,price,currency,stock,active,updated_at FROM crm_products ORDER BY updated_at DESC`)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []Product{}
		for rows.Next() {
			var x Product
			var ac int
			_ = rows.Scan(&x.ID, &x.Name, &x.Description, &x.Price, &x.Currency, &x.Stock, &ac, &x.UpdatedAt)
			x.Active = ac == 1
			out = append(out, x)
		}
		writeJSON(w, out)
	case http.MethodPost:
		if err := a.quotaAllowed(r, "products"); err != nil {
			writeError(w, err, 403)
			return
		}
		var x Product
		_ = json.NewDecoder(r.Body).Decode(&x)
		if strings.TrimSpace(x.Name) == "" {
			writeError(w, errors.New("nombre obligatorio"), 400)
			return
		}
		if x.Currency == "" {
			x.Currency = "COP"
		}
		x.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		ac := 0
		if x.Active {
			ac = 1
		}
		res, e := a.db.Exec(`INSERT INTO crm_products(name,description,price,currency,stock,active,updated_at) VALUES(?,?,?,?,?,?,?)`, x.Name, x.Description, x.Price, x.Currency, x.Stock, ac, x.UpdatedAt)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		x.ID, _ = res.LastInsertId()
		writeJSON(w, x)
	case http.MethodPut:
		var x Product
		_ = json.NewDecoder(r.Body).Decode(&x)
		ac := 0
		if x.Active {
			ac = 1
		}
		x.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		_, e := a.db.Exec(`UPDATE crm_products SET name=?,description=?,price=?,currency=?,stock=?,active=?,updated_at=? WHERE id=?`, x.Name, x.Description, x.Price, x.Currency, x.Stock, ac, x.UpdatedAt, x.ID)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, e := a.db.Exec(`DELETE FROM crm_products WHERE id=?`, id)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
func (a *App) appointmentsHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, e := a.db.Query(`SELECT id,contact_name,contact_phone,service,starts_at,duration_minutes,status,notes,created_at,professional_id,service_id,timezone FROM crm_appointments WHERE tenant_id=? ORDER BY starts_at`, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var x Appointment
			var professionalID, serviceID int64
			var timezone string
			_ = rows.Scan(&x.ID, &x.ContactName, &x.ContactPhone, &x.Service, &x.StartsAt, &x.DurationMinutes, &x.Status, &x.Notes, &x.CreatedAt, &professionalID, &serviceID, &timezone)
			out = append(out, map[string]any{"id": x.ID, "contact_name": x.ContactName, "contact_phone": x.ContactPhone, "service": x.Service, "starts_at": x.StartsAt, "duration_minutes": x.DurationMinutes, "status": x.Status, "notes": x.Notes, "created_at": x.CreatedAt, "professional_id": professionalID, "service_id": serviceID, "timezone": timezone})
		}
		writeJSON(w, out)
	case http.MethodPost:
		var q struct {
			Appointment
			ProfessionalID int64  `json:"professional_id"`
			ServiceID      int64  `json:"service_id"`
			Timezone       string `json:"timezone"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		if q.ContactName == "" || q.Service == "" || q.StartsAt == "" {
			writeError(w, errors.New("cliente, servicio y fecha son obligatorios"), 400)
			return
		}
		if q.DurationMinutes <= 0 {
			q.DurationMinutes = 30
		}
		if q.Status == "" {
			q.Status = "Programada"
		}
		if q.Timezone == "" {
			q.Timezone = "America/Bogota"
		}
		var overlap int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND professional_id=? AND status NOT IN ('Cancelada','No asistió') AND starts_at < datetime(?, '+' || ? || ' minutes') AND datetime(starts_at, '+' || duration_minutes || ' minutes') > ?`, tid, q.ProfessionalID, q.StartsAt, q.DurationMinutes, q.StartsAt).Scan(&overlap)
		if overlap > 0 {
			writeError(w, errors.New("ese horario ya está ocupado para el profesional seleccionado"), 409)
			return
		}
		q.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		res, e := a.db.Exec(`INSERT INTO crm_appointments(tenant_id,contact_name,contact_phone,service,starts_at,duration_minutes,status,notes,created_at,professional_id,service_id,timezone) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, tid, q.ContactName, q.ContactPhone, q.Service, q.StartsAt, q.DurationMinutes, q.Status, q.Notes, q.CreatedAt, q.ProfessionalID, q.ServiceID, q.Timezone)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		q.ID, _ = res.LastInsertId()
		writeJSON(w, q)
	case http.MethodPut:
		var q struct {
			Appointment
			ProfessionalID int64  `json:"professional_id"`
			ServiceID      int64  `json:"service_id"`
			Timezone       string `json:"timezone"`
		}
		if e := json.NewDecoder(r.Body).Decode(&q); e != nil {
			writeError(w, errors.New("datos de cita inválidos"), 400)
			return
		}
		if q.ID <= 0 || strings.TrimSpace(q.ContactName) == "" || strings.TrimSpace(q.Service) == "" || strings.TrimSpace(q.StartsAt) == "" {
			writeError(w, errors.New("cliente, servicio y fecha son obligatorios"), 400)
			return
		}
		if q.DurationMinutes <= 0 {
			q.DurationMinutes = 30
		}
		if q.Status == "" {
			q.Status = "Programada"
		}
		if q.Timezone == "" {
			q.Timezone = "America/Bogota"
		}
		var overlap int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND professional_id=? AND id<>? AND status NOT IN ('Cancelada','No asistió') AND starts_at < datetime(?, '+' || ? || ' minutes') AND datetime(starts_at, '+' || duration_minutes || ' minutes') > ?`, tid, q.ProfessionalID, q.ID, q.StartsAt, q.DurationMinutes, q.StartsAt).Scan(&overlap)
		if overlap > 0 {
			writeError(w, errors.New("ese horario ya está ocupado para el profesional seleccionado"), 409)
			return
		}
		res, e := a.db.Exec(`UPDATE crm_appointments SET contact_name=?,contact_phone=?,service=?,starts_at=?,duration_minutes=?,status=?,notes=?,professional_id=?,service_id=?,timezone=? WHERE id=? AND tenant_id=?`, q.ContactName, q.ContactPhone, q.Service, q.StartsAt, q.DurationMinutes, q.Status, q.Notes, q.ProfessionalID, q.ServiceID, q.Timezone, q.ID, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("cita no encontrada"), 404)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, e := a.db.Exec(`DELETE FROM crm_appointments WHERE id=? AND tenant_id=?`, id, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) messengerPageToken() string { return a.setting("messenger_page_token") }
func (a *App) messengerPageID() string    { return a.setting("messenger_page_id") }
func (a *App) messengerHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"configured": a.messengerPageToken() != "", "connected": a.messengerPageToken() != "", "page_id": a.messengerPageID(), "page_name": a.setting("messenger_page_name"), "webhook": "/webhooks/meta/messenger"})
	case http.MethodPost:
		if a.messengerPageToken() == "" {
			if err := a.quotaAllowed(r, "channels"); err != nil {
				writeError(w, err, 403)
				return
			}
		}
		var q struct {
			PageToken string `json:"page_token"`
			PageID    string `json:"page_id"`
			PageName  string `json:"page_name"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || strings.TrimSpace(q.PageToken) == "" || strings.TrimSpace(q.PageID) == "" {
			writeError(w, errors.New("Page Access Token y Page ID son obligatorios"), 400)
			return
		}
		pageToken := strings.TrimSpace(q.PageToken)
		pageID := strings.TrimSpace(q.PageID)
		validation, err := a.validateMessengerPageToken(pageToken, pageID, a.cfg.MetaAppID, a.cfg.MetaAppSecret)
		if err != nil {
			writeError(w, err, 400)
			return
		}
		pageName := strings.TrimSpace(q.PageName)
		if pageName == "" {
			pageName = "Página " + pageID
		}
		a.setSetting("messenger_page_token", pageToken)
		a.setSetting("messenger_page_id", pageID)
		a.setSetting("messenger_page_name", pageName)
		writeJSON(w, map[string]any{
			"ok":                true,
			"page_id":           pageID,
			"page_name":         pageName,
			"validation_method": validation.Method,
			"warning":           validation.Warning,
		})
	case http.MethodDelete:
		a.setSetting("messenger_page_token", "")
		a.setSetting("messenger_page_id", "")
		a.setSetting("messenger_page_name", "")
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
func (a *App) messengerWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("hub.mode") == "subscribe" && r.URL.Query().Get("hub.verify_token") == a.cfg.MessengerVerifyToken {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(r.URL.Query().Get("hub.challenge")))
			return
		}
		http.Error(w, "token de verificación inválido", 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var payload struct {
		Object string `json:"object"`
		Entry  []struct {
			Messaging []struct {
				Sender struct {
					ID string `json:"id"`
				} `json:"sender"`
				Recipient struct {
					ID string `json:"id"`
				} `json:"recipient"`
				Timestamp int64 `json:"timestamp"`
				Message   *struct {
					MID    string `json:"mid"`
					Text   string `json:"text"`
					IsEcho bool   `json:"is_echo"`
				} `json:"message"`
			} `json:"messaging"`
		} `json:"entry"`
	}
	if json.NewDecoder(r.Body).Decode(&payload) != nil {
		http.Error(w, "payload inválido", 400)
		return
	}
	for _, e := range payload.Entry {
		for _, m := range e.Messaging {
			if m.Message == nil || m.Message.IsEcho || strings.TrimSpace(m.Message.Text) == "" {
				continue
			}
			chat := "messenger:" + m.Sender.ID
			ts := time.Now().UTC()
			if m.Timestamp > 0 {
				ts = time.UnixMilli(m.Timestamp).UTC()
			}
			timestamp := ts.Format(time.RFC3339)
			_ = a.saveMessage(StoredMessage{Channel: "messenger", WAID: m.Message.MID, ChatJID: chat, SenderJID: m.Sender.ID, Direction: "in", MessageType: "text", Text: m.Message.Text, Status: "received", Timestamp: timestamp})
			_ = a.upsertContact(chat, m.Sender.ID, "Usuario Messenger", 1)
			_ = a.syncLegacyOpportunityIfSingleTenant(chat, "messenger", m.Message.Text, timestamp)
			go a.maybeAutoReply("messenger", chat, m.Message.Text)
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("EVENT_RECEIVED"))
}
func (a *App) sendMessengerText(ctx context.Context, psid, text string) (string, error) {
	token := a.messengerPageToken()
	if token == "" {
		return "", errors.New("Messenger no está conectado")
	}
	body, _ := json.Marshal(map[string]any{"recipient": map[string]string{"id": psid}, "messaging_type": "RESPONSE", "message": map[string]string{"text": text}})
	url := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/me/messages?access_token=" + token
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("Messenger: %s", strings.TrimSpace(string(b)))
	}
	var out struct {
		MessageID string `json:"message_id"`
	}
	_ = json.Unmarshal(b, &out)
	if out.MessageID == "" {
		out.MessageID = "messenger-" + randomToken(8)
	}
	chat := "messenger:" + psid
	_ = a.saveMessage(StoredMessage{Channel: "messenger", WAID: out.MessageID, ChatJID: chat, SenderJID: a.messengerPageID(), Direction: "out", MessageType: "text", Text: text, Status: "sent", Timestamp: time.Now().UTC().Format(time.RFC3339)})
	_ = a.upsertContact(chat, psid, "", 0)
	return out.MessageID, nil
}

func shortJID(v string) string { return strings.Split(v, "@")[0] }
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
}

func (a *App) quotaAllowed(r *http.Request, kind string) error {
	u := a.currentUser(r)
	if u == nil {
		return errors.New("sesión requerida")
	}
	if u.Role == "superadmin" {
		return nil
	}
	p, _, err := a.activePlan(a.billingAccountUserID(u))
	if err != nil {
		return err
	}
	var used, max int
	switch kind {
	case "products":
		max = p.MaxProducts
		tid, _, _ := a.tenantForRequest(r)
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_products WHERE tenant_id=?`, tid).Scan(&used)
	case "rules":
		max = p.MaxRules
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_auto_rules`).Scan(&used)
	case "contacts":
		max = p.MaxContacts
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_contacts`).Scan(&used)
	case "channels":
		max = p.MaxChannels
		if a.client != nil && a.client.Store.ID != nil {
			used++
		}
		if a.telegramToken() != "" {
			used++
		}
		if a.messengerPageToken() != "" {
			used++
		}
	default:
		return nil
	}
	if max >= 0 && used >= max {
		return fmt.Errorf("alcanzaste el límite de %s de tu plan %s", kind, p.Name)
	}
	return nil
}

func (a *App) plansHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT id,code,name,description,price_usdt,billing_days,max_users,max_channels,max_contacts,max_ai_responses,max_products,max_rules,active FROM billing_plans WHERE active=1 ORDER BY price_usdt`)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []Plan{}
	for rows.Next() {
		var p Plan
		var active int
		if rows.Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.PriceUSDT, &p.BillingDays, &p.MaxUsers, &p.MaxChannels, &p.MaxContacts, &p.MaxAIResponses, &p.MaxProducts, &p.MaxRules, &active) == nil {
			p.Active = active == 1
			out = append(out, p)
		}
	}
	writeJSON(w, out)
}

func (a *App) activePlan(userID int64) (Plan, Subscription, error) {
	var p Plan
	var sub Subscription
	var active int
	err := a.db.QueryRow(`SELECT p.id,p.code,p.name,p.description,p.price_usdt,p.billing_days,p.max_users,p.max_channels,p.max_contacts,p.max_ai_responses,p.max_products,p.max_rules,p.active,
	 s.id,s.user_id,s.plan_code,s.status,s.starts_at,s.ends_at,s.created_at
	 FROM billing_subscriptions s JOIN billing_plans p ON p.code=s.plan_code
	 WHERE s.user_id=? AND s.status='active' AND s.ends_at>? ORDER BY s.ends_at DESC LIMIT 1`, userID, time.Now().UTC().Format(time.RFC3339)).Scan(
		&p.ID, &p.Code, &p.Name, &p.Description, &p.PriceUSDT, &p.BillingDays, &p.MaxUsers, &p.MaxChannels, &p.MaxContacts, &p.MaxAIResponses, &p.MaxProducts, &p.MaxRules, &active,
		&sub.ID, &sub.UserID, &sub.PlanCode, &sub.Status, &sub.StartsAt, &sub.EndsAt, &sub.CreatedAt)
	p.Active = active == 1
	if err == sql.ErrNoRows {
		now := time.Now().UTC()
		_, _ = a.db.Exec(`INSERT INTO billing_subscriptions(user_id,plan_code,status,starts_at,ends_at,created_at) VALUES(?,'free','active',?,?,?)`, userID, now.Format(time.RFC3339), now.AddDate(10, 0, 0).Format(time.RFC3339), now.Format(time.RFC3339))
		return a.activePlan(userID)
	}
	return p, sub, err
}

func (a *App) entitlementsHandler(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	billingID := a.billingAccountUserID(u)
	p, sub, err := a.activePlan(billingID)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	e := Entitlements{PlanCode: p.Code, PlanName: p.Name, Subscription: sub.Status, EndsAt: sub.EndsAt, MaxUsers: p.MaxUsers, MaxChannels: p.MaxChannels, MaxContacts: p.MaxContacts, MaxAIResponses: p.MaxAIResponses, MaxProducts: p.MaxProducts, MaxRules: p.MaxRules}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE tenant_id=? AND active=1`, u.TenantID).Scan(&e.UsersUsed)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_contacts`).Scan(&e.ContactsUsed)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_products`).Scan(&e.ProductsUsed)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_auto_rules`).Scan(&e.RulesUsed)
	_ = a.db.QueryRow(`SELECT COALESCE(ai_responses,0) FROM usage_monthly WHERE user_id=? AND period=?`, billingID, time.Now().UTC().Format("2006-01")).Scan(&e.AIUsed)
	writeJSON(w, e)
}

func (a *App) billingConfigHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"bep20_address": a.cfg.USDTBEP20Address, "trc20_address": a.cfg.USDTTRC20Address, "confirmations": a.cfg.PaymentConfirmations})
}

func (a *App) checkoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	var q struct {
		PlanCode string `json:"plan_code"`
		Network  string `json:"network"`
		TxHash   string `json:"tx_hash"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	q.PlanCode = strings.ToLower(strings.TrimSpace(q.PlanCode))
	q.Network = strings.ToUpper(strings.TrimSpace(q.Network))
	q.TxHash = strings.TrimSpace(q.TxHash)
	if q.PlanCode == "free" {
		writeError(w, errors.New("el plan Free no requiere pago"), 400)
		return
	}
	var p Plan
	var active int
	if a.db.QueryRow(`SELECT id,code,name,description,price_usdt,billing_days,max_users,max_channels,max_contacts,max_ai_responses,max_products,max_rules,active FROM billing_plans WHERE code=? AND active=1`, q.PlanCode).Scan(&p.ID, &p.Code, &p.Name, &p.Description, &p.PriceUSDT, &p.BillingDays, &p.MaxUsers, &p.MaxChannels, &p.MaxContacts, &p.MaxAIResponses, &p.MaxProducts, &p.MaxRules, &active) != nil {
		writeError(w, errors.New("plan inválido"), 400)
		return
	}
	wallet := ""
	if q.Network == "BEP20" {
		wallet = a.cfg.USDTBEP20Address
	} else if q.Network == "TRC20" {
		wallet = a.cfg.USDTTRC20Address
	} else {
		writeError(w, errors.New("selecciona BEP20 o TRC20"), 400)
		return
	}
	if wallet == "" {
		writeError(w, errors.New("la billetera de esta red no está configurada"), 503)
		return
	}
	if len(q.TxHash) < 20 {
		writeError(w, errors.New("ingresa un hash de transacción válido"), 400)
		return
	}
	res, err := a.db.Exec(`INSERT INTO billing_payments(user_id,plan_code,network,wallet,amount_usdt,tx_hash,status,created_at) VALUES(?,?,?,?,?,?,'pending',?)`, a.billingAccountUserID(u), p.Code, q.Network, wallet, p.PriceUSDT, q.TxHash, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		writeError(w, errors.New("esa transacción ya fue registrada"), 400)
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"ok": true, "payment_id": id, "status": "pending", "message": "Pago registrado. Worktic lo verificará antes de activar el plan."})
}

func (a *App) paymentsHandler(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	rows, err := a.db.Query(`SELECT bp.id,bp.user_id,u.name,u.email,bp.plan_code,p.name,bp.network,bp.wallet,bp.amount_usdt,bp.tx_hash,bp.status,bp.admin_note,bp.created_at,bp.reviewed_at FROM billing_payments bp JOIN app_users u ON u.id=bp.user_id JOIN billing_plans p ON p.code=bp.plan_code WHERE bp.user_id=? ORDER BY bp.id DESC`, a.billingAccountUserID(u))
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []CryptoPayment{}
	for rows.Next() {
		var x CryptoPayment
		_ = rows.Scan(&x.ID, &x.UserID, &x.UserName, &x.UserEmail, &x.PlanCode, &x.PlanName, &x.Network, &x.Wallet, &x.AmountUSDT, &x.TxHash, &x.Status, &x.AdminNote, &x.CreatedAt, &x.ReviewedAt)
		out = append(out, x)
	}
	writeJSON(w, out)
}

func (a *App) requireSuperadmin(w http.ResponseWriter, r *http.Request) (*User, bool) {
	u := a.currentUser(r)
	if u == nil || u.Role != "superadmin" {
		writeError(w, errors.New("acceso reservado al superadministrador"), 403)
		return nil, false
	}
	return u, true
}

func (a *App) adminOverviewHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	var users, activeUsers, pending, approved, subs int
	var revenue float64
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM app_users`).Scan(&users)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE active=1`).Scan(&activeUsers)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE status='pending'`).Scan(&pending)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE status='approved'`).Scan(&approved)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(amount_usdt),0) FROM billing_payments WHERE status='approved'`).Scan(&revenue)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_subscriptions WHERE status='active' AND ends_at>?`, time.Now().UTC().Format(time.RFC3339)).Scan(&subs)
	writeJSON(w, map[string]any{"users": users, "active_users": activeUsers, "pending_payments": pending, "approved_payments": approved, "revenue_usdt": revenue, "active_subscriptions": subs})
}

func (a *App) adminPaymentsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := a.db.Query(`SELECT bp.id,bp.user_id,u.name,u.email,bp.plan_code,p.name,bp.network,bp.wallet,bp.amount_usdt,bp.tx_hash,bp.status,bp.admin_note,bp.created_at,bp.reviewed_at FROM billing_payments bp JOIN app_users u ON u.id=bp.user_id JOIN billing_plans p ON p.code=bp.plan_code ORDER BY CASE bp.status WHEN 'pending' THEN 0 ELSE 1 END,bp.id DESC`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []CryptoPayment{}
		for rows.Next() {
			var x CryptoPayment
			_ = rows.Scan(&x.ID, &x.UserID, &x.UserName, &x.UserEmail, &x.PlanCode, &x.PlanName, &x.Network, &x.Wallet, &x.AmountUSDT, &x.TxHash, &x.Status, &x.AdminNote, &x.CreatedAt, &x.ReviewedAt)
			out = append(out, x)
		}
		writeJSON(w, out)
		return
	}
	if r.Method == http.MethodPut {
		var q struct {
			ID     int64  `json:"id"`
			Action string `json:"action"`
			Note   string `json:"note"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		var userID int64
		var planCode, status string
		if a.db.QueryRow(`SELECT user_id,plan_code,status FROM billing_payments WHERE id=?`, q.ID).Scan(&userID, &planCode, &status) != nil {
			writeError(w, errors.New("pago no encontrado"), 404)
			return
		}
		if status != "pending" {
			writeError(w, errors.New("el pago ya fue procesado"), 400)
			return
		}
		now := time.Now().UTC()
		if q.Action == "approve" {
			var days int
			_ = a.db.QueryRow(`SELECT billing_days FROM billing_plans WHERE code=?`, planCode).Scan(&days)
			_, _ = a.db.Exec(`UPDATE billing_subscriptions SET status='replaced' WHERE user_id=? AND status='active'`, userID)
			_, err := a.db.Exec(`INSERT INTO billing_subscriptions(user_id,plan_code,status,starts_at,ends_at,created_at) VALUES(?,?,'active',?,?,?)`, userID, planCode, now.Format(time.RFC3339), now.AddDate(0, 0, days).Format(time.RFC3339), now.Format(time.RFC3339))
			if err != nil {
				writeError(w, err, 500)
				return
			}
			_, _ = a.db.Exec(`UPDATE billing_payments SET status='approved',admin_note=?,reviewed_at=? WHERE id=?`, strings.TrimSpace(q.Note), now.Format(time.RFC3339), q.ID)
		} else if q.Action == "reject" {
			_, _ = a.db.Exec(`UPDATE billing_payments SET status='rejected',admin_note=?,reviewed_at=? WHERE id=?`, strings.TrimSpace(q.Note), now.Format(time.RFC3339), q.ID)
		} else {
			writeError(w, errors.New("acción inválida"), 400)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	http.Error(w, "Método no permitido", 405)
}

func (a *App) adminSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	rows, err := a.db.Query(`SELECT s.id,s.user_id,s.plan_code,p.name,s.status,s.starts_at,s.ends_at,s.created_at,u.name,u.email,u.company FROM billing_subscriptions s JOIN billing_plans p ON p.code=s.plan_code JOIN app_users u ON u.id=s.user_id ORDER BY s.id DESC LIMIT 500`)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, uid int64
		var pc, pn, st, sa, ea, ca, un, ue, co string
		_ = rows.Scan(&id, &uid, &pc, &pn, &st, &sa, &ea, &ca, &un, &ue, &co)
		out = append(out, map[string]any{"id": id, "user_id": uid, "plan_code": pc, "plan_name": pn, "status": st, "starts_at": sa, "ends_at": ea, "created_at": ca, "user_name": un, "user_email": ue, "company": co})
	}
	writeJSON(w, out)
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func envInt(k string, d int) int {
	v, err := strconv.Atoi(env(k, ""))
	if err != nil {
		return d
	}
	return v
}
func loadEnvFile(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := strings.SplitN(line, "=", 2)
		if len(p) == 2 {
			_ = os.Setenv(strings.TrimSpace(p[0]), strings.Trim(strings.TrimSpace(p[1]), "\"'"))
		}
	}
}
