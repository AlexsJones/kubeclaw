// Package main is the entry point for the WhatsApp channel pod.
// WhatsApp uses a StatefulSet for persistent session auth.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/k8sclaw/k8sclaw/internal/channel"
	"github.com/k8sclaw/k8sclaw/internal/eventbus"
)

// WhatsAppChannel implements the WhatsApp Business Cloud API channel.
type WhatsAppChannel struct {
	channel.BaseChannel
	AccessToken   string
	PhoneNumberID string
	VerifyToken   string
	client        *http.Client
	healthy       bool
}

func main() {
	var instanceName string
	var eventBusURL string
	var accessToken string
	var phoneNumberID string
	var verifyToken string
	var listenAddr string

	flag.StringVar(&instanceName, "instance", os.Getenv("INSTANCE_NAME"), "ClawInstance name")
	flag.StringVar(&eventBusURL, "event-bus-url", os.Getenv("EVENT_BUS_URL"), "Event bus URL")
	flag.StringVar(&accessToken, "access-token", os.Getenv("WHATSAPP_ACCESS_TOKEN"), "WhatsApp access token")
	flag.StringVar(&phoneNumberID, "phone-number-id", os.Getenv("WHATSAPP_PHONE_NUMBER_ID"), "WhatsApp phone number ID")
	flag.StringVar(&verifyToken, "verify-token", os.Getenv("WHATSAPP_VERIFY_TOKEN"), "Webhook verify token")
	flag.StringVar(&listenAddr, "addr", ":3000", "Listen address for webhook")
	flag.Parse()

	if accessToken == "" {
		fmt.Fprintln(os.Stderr, "WHATSAPP_ACCESS_TOKEN is required")
		os.Exit(1)
	}

	log := zap.New(zap.UseDevMode(false)).WithName("channel-whatsapp")

	bus, err := eventbus.NewNATSEventBus(eventBusURL)
	if err != nil {
		log.Error(err, "failed to connect to event bus")
		os.Exit(1)
	}
	defer bus.Close()

	ch := &WhatsAppChannel{
		BaseChannel: channel.BaseChannel{
			ChannelType:  "whatsapp",
			InstanceName: instanceName,
			EventBus:     bus,
		},
		AccessToken:   accessToken,
		PhoneNumberID: phoneNumberID,
		VerifyToken:   verifyToken,
		client:        &http.Client{Timeout: 30 * time.Second},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go ch.handleOutbound(ctx)

	log.Info("Starting WhatsApp channel", "instance", instanceName, "addr", listenAddr)
	ch.healthy = true
	_ = ch.PublishHealth(ctx, channel.HealthStatus{Connected: true})

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", ch.handleWebhook)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if ch.healthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error(err, "whatsapp server failed")
	}
}

// handleWebhook processes WhatsApp Cloud API webhooks.
func (wc *WhatsAppChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Verification challenge (GET)
	if r.Method == http.MethodGet {
		mode := r.URL.Query().Get("hub.mode")
		token := r.URL.Query().Get("hub.verify_token")
		challenge := r.URL.Query().Get("hub.challenge")

		if mode == "subscribe" && token == wc.VerifyToken {
			fmt.Fprint(w, challenge)
			return
		}
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Process incoming messages (POST)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var payload struct {
		Entry []struct {
			Changes []struct {
				Value struct {
					Messages []struct {
						From string `json:"from"`
						Type string `json:"type"`
						Text struct {
							Body string `json:"body"`
						} `json:"text"`
						Timestamp string `json:"timestamp"`
						ID        string `json:"id"`
					} `json:"messages"`
					Contacts []struct {
						Profile struct {
							Name string `json:"name"`
						} `json:"profile"`
						WaID string `json:"wa_id"`
					} `json:"contacts"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, message := range change.Value.Messages {
				if message.Type != "text" {
					continue
				}

				senderName := message.From
				for _, contact := range change.Value.Contacts {
					if contact.WaID == message.From {
						senderName = contact.Profile.Name
						break
					}
				}

				msg := channel.InboundMessage{
					SenderID:   message.From,
					SenderName: senderName,
					ChatID:     message.From,
					Text:       message.Text.Body,
					Metadata: map[string]string{
						"messageId": message.ID,
						"timestamp": message.Timestamp,
					},
				}

				if err := wc.PublishInbound(r.Context(), msg); err != nil {
					fmt.Fprintf(os.Stderr, "failed to publish inbound: %v\n", err)
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleOutbound subscribes to outbound messages.
func (wc *WhatsAppChannel) handleOutbound(ctx context.Context) {
	events, err := wc.SubscribeOutbound(ctx)
	if err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-events:
			var msg channel.OutboundMessage
			if err := json.Unmarshal(event.Data, &msg); err != nil {
				continue
			}
			if msg.Channel != "whatsapp" {
				continue
			}
			_ = wc.sendMessage(ctx, msg)
		}
	}
}

// sendMessage sends a message via the WhatsApp Cloud API.
func (wc *WhatsAppChannel) sendMessage(ctx context.Context, msg channel.OutboundMessage) error {
	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", wc.PhoneNumberID)

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                msg.ChatID,
		"type":              "text",
		"text": map[string]string{
			"body": msg.Text,
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+wc.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := wc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
