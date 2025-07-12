package discord

import (
	"context"
	"log"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/vcaldo/discord-iptv-player/remote_control/internal/config"
)

type PeriodicTask struct {
	Name         string
	Interval     time.Duration
	InitialDelay time.Duration // Optional initial delay before first execution
	Handler      func(ctx context.Context, config *config.Config, nrApp *newrelic.Application) error
}

func (b *Bot) startPeriodicTaskManager(ctx context.Context, config *config.Config, nrApp *newrelic.Application) {
	tasks := []PeriodicTask{
		{
			Name:         "delayed-bot-disconnect",
			Interval:     5 * time.Minute,
			InitialDelay: 30 * time.Second, // Start checking after 30 seconds
			Handler:      b.isBotAlone,
		},
		// Add more periodic tasks here in the future:
	}

	log.Printf("starting periodic task manager with %d tasks", len(tasks))
	log.Printf("context error state at startup: %v", ctx.Err())

	for _, task := range tasks {
		go b.runPeriodicTask(ctx, task, config, nrApp)
	}
}

func (b *Bot) runPeriodicTask(ctx context.Context, task PeriodicTask, config *config.Config, nrApp *newrelic.Application) {
	log.Printf("started periodic task: %s (interval: %s, initial delay: %s)", task.Name, task.Interval, task.InitialDelay)

	// Handle initial delay
	if task.InitialDelay > 0 {
		log.Printf("waiting %s before first execution of task: %s", task.InitialDelay, task.Name)
		select {
		case <-ctx.Done():
			log.Printf("stopping periodic task: %s during initial delay due to context cancellation: %v", task.Name, ctx.Err())
			return
		case <-time.After(task.InitialDelay):
			// Continue to first execution
		}
	}

	// Execute the task immediately after initial delay
	log.Printf("executing initial run of periodic task: %s", task.Name)
	if err := task.Handler(ctx, config, nrApp); err != nil {
		log.Printf("error in initial run of periodic task %s: %v", task.Name, err)
	} else {
		log.Printf("initial run of periodic task %s completed successfully", task.Name)
	}

	// Now start the regular ticker
	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("stopping periodic task: %s due to context cancellation: %v", task.Name, ctx.Err())
			return
		case <-ticker.C:
			// Check if context is still valid before executing
			if ctx.Err() != nil {
				log.Printf("context cancelled, stopping periodic task: %s", task.Name)
				return
			}

			log.Printf("executing periodic task: %s", task.Name)
			if err := task.Handler(ctx, config, nrApp); err != nil {
				log.Printf("error in periodic task %s: %v", task.Name, err)
			} else {
				log.Printf("periodic task %s completed successfully", task.Name)
			}
		}
	}
}
