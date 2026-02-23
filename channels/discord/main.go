// Package main is the entry point for the Discord channel pod.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/k8sclaw/k8sclaw/internal/channel"
	"github.com/k8sclaw/k8sclaw/internal/eventbus"
)

// DiscordChannel implements the Discord Gateway channel.
type DiscordChannel struct {
	channel.BaseChannel
	BotToken string
	client   *http.Client
	healthy  bool
}

func main() {
	var instanceName string
	var eventBusURL string
	var botToken string

	flag.StringVar(&instanceName, "instance", os.Getenv("INSTANCE_NAME"), "ClawInstance name")
	flag.StringVar(&eventBusURL, "event-bus-url", os.Getenv("EVENT_BUS_URL"), "Event bus URL")
	flag.StringVar(&botToken, "bot-token", os.Getenv("DISCORD_BOT_TOKEN"), "Discord bot token")
	flag.Parse()

	if botToken == "" {
		fmt.Fprintln(os.Stderr, "DISCORD_BOT_TOKEN is required")
		os.Exit(1)
	}

	log := zap.New(zap.UseDevMode(false)).WithName("channel-discord")

	bus, err := eventbus.NewNATSEventBus(eventBusURL)
	if err != nil {
		log.Error(err, "failed to connect to event bus")
		os.Exit(1)
	}
	defer bus.Close()

	ch := &DiscordChannel{
		BaseChannel: channel.BaseChannel{
			ChannelType:  "discord",
			InstanceName: instanceName,
			EventBus:     bus,
		},
		BotToken: botToken,
		client:   &http.Client{Timeout: 30 * time.Second},
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Health server
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			if ch.healthy {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		})
		_ = http.ListenAndServe(":8080", mux)
	}()

	go ch.handleOutbound(ctx)

	log.Info("Starting Discord channel", "instance", instanceName)
	if err := ch.connectGateway(ctx); err != nil {
		log.Error(err, "discord gateway failed")
	}
}

// connectGateway connects to the Discord WebSocket Gateway.
// This is a simplified placeholder — full implementation would use the
// Discord Gateway API with heartbeats, resume, and intent-based events.
func (dc *DiscordChannel) connectGateway(ctx context.Context) error {
	dc.healthy = true
	_ = dc.PublishHealth(ctx, channel.HealthStatus{Connected: true})

	// In a full implementation, this would:
	// 1. GET /api/v10/gateway/bot to get the WebSocket URL
	// 2. Connect via WebSocket with proper intents
	// 3. Handle READY, MESSAGE_CREATE, etc. events
	// 4. Send heartbeats per the hello payload interval

	<-ctx.Done()
	return nil
}

// handleOutbound subscribes to outbound messages and sends them via Discord REST API.
func (dc *DiscordChannel) handleOutbound(ctx context.Context) {
	events, err := dc.SubscribeOutbound(ctx)
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
			if msg.Channel != "discord" {
				continue
			}
			_ = dc.sendMessage(ctx, msg)
		}
	}
}

// sendMessage sends a message via the Discord REST API.
func (dc *DiscordChannel) sendMessage(ctx context.Context, msg channel.OutboundMessage) error {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", msg.ChatID)

	payload := map[string]interface{}{
		"content": msg.Text,
	}
	body, _ := json.Marshal(payload)
	_ = body

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+dc.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := dc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
