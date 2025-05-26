package discord

import (
	"context"
	"log"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
)

// PeriodicTask represents a task that runs periodically
type PeriodicTask struct {
	Name     string
	Interval time.Duration
	Handler  func(ctx context.Context, config *config.Config, nrApp *newrelic.Application) error
}

// startPeriodicTaskManager runs multiple periodic tasks
func (b *Bot) startPeriodicTaskManager(ctx context.Context, config *config.Config, nrApp *newrelic.Application) {
	tasks := []PeriodicTask{
		{
			Name:     "delayed-bot-disconnect",
			Interval: 10 * time.Minute,
			Handler:  b.isBotAlone,
		},
		// Add more periodic tasks here in the future:
	}

	log.Printf("starting periodic task manager with %d tasks", len(tasks))

	for _, task := range tasks {
		go b.runPeriodicTask(ctx, task, config, nrApp)
	}
}

func (b *Bot) runPeriodicTask(ctx context.Context, task PeriodicTask, config *config.Config, nrApp *newrelic.Application) {
	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()

	log.Printf("started periodic task: %s (interval: %s)", task.Name, task.Interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("stopping periodic task: %s", task.Name)
			return
		case <-ticker.C:
			if err := task.Handler(ctx, config, nrApp); err != nil {
				log.Printf("error in periodic task %s: %v", task.Name, err)
			}
		}
	}
}
