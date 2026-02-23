// Package main is the entry point for the Slack channel pod.
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

// SlackChannel implements the Slack Events API channel.
type SlackChannel struct {
	channel.BaseChannel
	BotToken      string
	SigningSecret string
	client        *http.Client
	healthy       bool
}

func main() {
	var instanceName string
	var eventBusURL string
	var botToken string
	var signingSecret string
	var listenAddr string

	flag.StringVar(&instanceName, "instance", os.Getenv("INSTANCE_NAME"), "ClawInstance name")
	flag.StringVar(&eventBusURL, "event-bus-url", os.Getenv("EVENT_BUS_URL"), "Event bus URL")
	flag.StringVar(&botToken, "bot-token", os.Getenv("SLACK_BOT_TOKEN"), "Slack bot token")
	flag.StringVar(&signingSecret, "signing-secret", os.Getenv("SLACK_SIGNING_SECRET"), "Slack signing secret")
	flag.StringVar(&listenAddr, "addr", ":3000", "Listen address for Slack events")
	flag.Parse()

	if botToken == "" {
		fmt.Fprintln(os.Stderr, "SLACK_BOT_TOKEN is required")
		os.Exit(1)
	}

	log := zap.New(zap.UseDevMode(false)).WithName("channel-slack")

	bus, err := eventbus.NewNATSEventBus(eventBusURL)
	if err != nil {
		log.Error(err, "failed to connect to event bus")
		os.Exit(1)
	}
	defer bus.Close()

	ch := &SlackChannel{
		BaseChannel: channel.BaseChannel{
			ChannelType:  "slack",
			InstanceName: instanceName,
			EventBus:     bus,
		},
		BotToken:      botToken,
		SigningSecret: signingSecret,
		client:        &http.Client{Timeout: 30 * time.Second},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go ch.handleOutbound(ctx)

	log.Info("Starting Slack channel", "instance", instanceName, "addr", listenAddr)
	ch.healthy = true
	_ = ch.PublishHealth(ctx, channel.HealthStatus{Connected: true})

	mux := http.NewServeMux()
	mux.HandleFunc("/slack/events", ch.handleSlackEvents)
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
		log.Error(err, "slack server failed")
	}
}

// handleSlackEvents processes incoming Slack Events API payloads.
func (sc *SlackChannel) handleSlackEvents(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse the event envelope
	var envelope struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
		Event     struct {
			Type    string `json:"type"`
			User    string `json:"user"`
			Text    string `json:"text"`
			Channel string `json:"channel"`
			TS      string `json:"ts"`
			ThreadTS string `json:"thread_ts"`
		} `json:"event"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Handle URL verification challenge
	if envelope.Type == "url_verification" {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, envelope.Challenge)
		return
	}

	// Process message events
	if envelope.Type == "event_callback" && envelope.Event.Type == "message" {
		if envelope.Event.User == "" || envelope.Event.Text == "" {
			w.WriteHeader(http.StatusOK)
			return
		}

		msg := channel.InboundMessage{
			SenderID: envelope.Event.User,
			ChatID:   envelope.Event.Channel,
			ThreadID: envelope.Event.ThreadTS,
			Text:     envelope.Event.Text,
			Metadata: map[string]string{
				"ts": envelope.Event.TS,
			},
		}

		if err := sc.PublishInbound(r.Context(), msg); err != nil {
			fmt.Fprintf(os.Stderr, "failed to publish inbound: %v\n", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleOutbound subscribes to outbound messages and sends them via Slack API.
func (sc *SlackChannel) handleOutbound(ctx context.Context) {
	events, err := sc.SubscribeOutbound(ctx)
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
			if msg.Channel != "slack" {
				continue
			}
			_ = sc.sendMessage(ctx, msg)
		}
	}
}

// sendMessage sends a message via the Slack chat.postMessage API.
func (sc *SlackChannel) sendMessage(ctx context.Context, msg channel.OutboundMessage) error {
	payload := map[string]interface{}{
		"channel": msg.ChatID,
		"text":    msg.Text,
	}
	if msg.ThreadID != "" {
		payload["thread_ts"] = msg.ThreadID
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://slack.com/api/chat.postMessage",
		strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+sc.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
