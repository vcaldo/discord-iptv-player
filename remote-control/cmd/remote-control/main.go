package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/discord"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/redis"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("error loading config: %v", err)
		return
	}

	nrApp, err := newrelic.NewApplication(
		newrelic.ConfigAppName(config.NewRelicAppName),
		newrelic.ConfigLicense(config.NewRelicLicenseKey),
		newrelic.ConfigDistributedTracerEnabled(true),
		newrelic.ConfigAppLogForwardingEnabled(true),
	)
	if err != nil {
		log.Printf("error initializing New Relic: %v", err)
		log.Println("continuing without instrumentation")
	} else {
		log.Println("new relic initialized successfully")
		// Allow some time for New Relic to initialize
		time.Sleep(1 * time.Second)
	}

	redisClient, err := redis.NewClient(ctx, config, nrApp)
	if err != nil {
		log.Fatalf("error initializing Redis client: %v", err)
	}
	defer redisClient.Close()

	discordBot, err := discord.NewBot(config, redisClient, nrApp)
	if err != nil {
		log.Fatalf("error initializing Discord bot: %v", err)
	}

	go func() {
		if err := discordBot.Start(ctx, nrApp); err != nil {
			log.Printf("error running Discord bot: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	nrApp.Shutdown(5 * time.Second)
}
