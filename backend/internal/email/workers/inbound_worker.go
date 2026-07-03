// Package workers contains background goroutines for email processing.
package workers

import (
	"context"
	"time"

	"github.com/ayush/supportiq/internal/repositories"
	"github.com/ayush/supportiq/internal/services"
	"github.com/ayush/supportiq/internal/utils"
)

// StartInboundWorker polls all active IMAP mailboxes on every pollInterval tick.
// Runs in the caller's goroutine; returns when ctx is cancelled.
func StartInboundWorker(
	ctx context.Context,
	accountRepo *repositories.EmailAccountRepository,
	emailSvc *services.EmailService,
	accountSvc *services.EmailAccountService,
	pollInterval time.Duration,
) {
	utils.Logger.WithField("interval", pollInterval).Info("InboundWorker: started")

	doPoll := func() {
		accounts, err := accountRepo.FindActive()
		if err != nil {
			utils.Logger.WithError(err).Warn("InboundWorker: load accounts failed")
			return
		}

		for i := range accounts {
			account := &accounts[i]
			if account.IMAPHost == "" {
				continue
			}

			receiver, err := accountSvc.BuildReceiver(account)
			if err != nil {
				utils.Logger.WithError(err).
					WithField("account", account.EmailAddress).
					Warn("InboundWorker: build receiver failed")
				continue
			}

			parsed, err := receiver.FetchUnread(ctx)
			if err != nil {
				utils.Logger.WithError(err).
					WithField("account", account.EmailAddress).
					Warn("InboundWorker: IMAP fetch failed")
				continue
			}

			for j := range parsed {
				if err := emailSvc.ProcessInbound(ctx, account, &parsed[j]); err != nil {
					utils.Logger.WithError(err).
						WithField("message_id", parsed[j].MessageID).
						Warn("InboundWorker: process inbound failed")
					continue
				}

				// Mark as seen only after successful processing
				if parsed[j].UID > 0 {
					if err := receiver.MarkSeen(ctx, parsed[j].UID); err != nil {
						utils.Logger.WithError(err).
							WithField("uid", parsed[j].UID).
							Warn("InboundWorker: mark seen failed")
					}
				}
			}

			// Persist last sync timestamp
			now := time.Now()
			account.LastSyncAt = &now
			if err := accountRepo.Update(account); err != nil {
				utils.Logger.WithError(err).Warn("InboundWorker: update last_sync_at failed")
			}

			utils.Logger.WithField("account", account.EmailAddress).
				WithField("count", len(parsed)).
				Info("InboundWorker: poll complete")
		}
	}

	// Run immediately at startup
	doPoll()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			utils.Logger.Info("InboundWorker: stopping")
			return
		case <-ticker.C:
			doPoll()
		}
	}
}
